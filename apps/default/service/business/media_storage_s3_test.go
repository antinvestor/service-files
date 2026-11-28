package business

import (
	"bytes"
	"context"
	"image"
	color "image/color" //nolint:misspell
	"image/jpeg"
	"io"
	"strings"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider/s3"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/service/utils"
	"github.com/pitabwire/frame/v3/frametests/definition"
	"github.com/pitabwire/frame/v3/security"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	s3PrivateBucket = "media-private"
	s3PublicBucket  = "media-public"
)

// MediaStorageS3TestSuite runs the media service against Postgres (with RLS
// enforced) and a real S3-compatible store, asserting on what actually lands
// in the bucket and in media_metadata.
type MediaStorageS3TestSuite struct {
	tests.BaseTestSuite

	minio *tests.MinioServer
}

func TestMediaStorageS3TestSuite(t *testing.T) {
	suite.Run(t, new(MediaStorageS3TestSuite))
}

func (s *MediaStorageS3TestSuite) SetupSuite() {
	s.BaseTestSuite.SetupSuite()
	srv, err := tests.StartMinio(context.Background(), s3PrivateBucket, s3PublicBucket)
	s.Require().NoError(err)
	s.minio = srv
}

func (s *MediaStorageS3TestSuite) TearDownSuite() {
	s.Require().NoError(s.minio.Terminate(context.Background()))
	s.BaseTestSuite.TearDownSuite()
}

type s3Env struct {
	ctx     context.Context
	cfg     *config.FilesConfig
	db      *connection.Database
	service MediaService
	prov    storage.Provider
}

func (s *MediaStorageS3TestSuite) newEnv(t *testing.T, dep *definition.DependencyOption) *s3Env {
	t.Helper()
	ctx, svc, res := s.CreateService(t, dep)
	cfg := svc.Config().(*config.FilesConfig)
	cfg.DynamicThumbnails = true
	cfg.ThumbnailSizes = []config.ThumbnailSize{{Width: 32, Height: 32, ResizeMethod: "scale"}}

	prov := s3.NewProvider("S3", s3PrivateBucket, s3PublicBucket, s.minio.Endpoint, tests.MinioRegion,
		s.minio.Container.Password, "", s.minio.Container.Username)
	require.NoError(t, prov.Setup(ctx))

	db := &connection.Database{WorkManager: svc.WorkManager(), MediaRepository: res.MediaRepository}
	return &s3Env{ctx: ctx, cfg: cfg, db: db, service: NewMediaService(db, prov), prov: prov}
}

func partitionCtx(ctx context.Context, tenantID, partitionID string) context.Context {
	claims := &security.AuthenticationClaims{
		TenantID:    tenantID,
		PartitionID: partitionID,
		AccessID:    util.IDString(),
	}
	claims.Subject = "user-" + partitionID
	return claims.ClaimsToContext(ctx)
}

func (e *s3Env) upload(t *testing.T, ctx context.Context, owner string, content []byte, isPublic bool) types.MediaID {
	t.Helper()
	res, err := e.service.UploadFile(ctx, &UploadRequest{
		OwnerID:       types.OwnerID(owner),
		UploadName:    "file.bin",
		ContentType:   "application/octet-stream",
		FileSizeBytes: types.FileSizeBytes(len(content)),
		FileData:      bytes.NewReader(content),
		Config:        e.cfg,
		IsPublic:      isPublic,
	})
	require.NoError(t, err)
	return res.MediaID
}

func (e *s3Env) download(t *testing.T, ctx context.Context, id types.MediaID) []byte {
	t.Helper()
	res, err := e.service.DownloadFile(ctx, &DownloadRequest{MediaID: id, Config: e.cfg})
	require.NoError(t, err)
	defer util.CloseAndLogOnError(ctx, res.FileData)
	body, err := io.ReadAll(res.FileData)
	require.NoError(t, err)
	return body
}

func (e *s3Env) metadata(t *testing.T, ctx context.Context, id types.MediaID) *types.MediaMetadata {
	t.Helper()
	meta, err := e.db.GetMediaMetadata(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, meta, "media %s must exist", id)
	return meta
}

func (s *MediaStorageS3TestSuite) requireObject(t *testing.T, ctx context.Context, bucket, key string, want bool) {
	t.Helper()
	exists, err := s.minio.ObjectExists(ctx, bucket, key)
	require.NoError(t, err)
	require.Equal(t, want, exists, "object %s/%s presence", bucket, key)
}

func (s *MediaStorageS3TestSuite) TestUploadFile_PersistsBucketAndRelativeKey() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		env := s.newEnv(t, dep)

		testCases := []struct {
			name       string
			isPublic   bool
			wantBucket string
			wantSuffix string
		}{
			{name: "private_encrypted", isPublic: false, wantBucket: s3PrivateBucket},
			{name: "public_plain", isPublic: true, wantBucket: s3PublicBucket, wantSuffix: "/file"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx := env.ctx
				content := []byte("upload " + tc.name + " " + util.IDString())
				id := env.upload(t, ctx, "@owner:example.com", content, tc.isPublic)

				meta := env.metadata(t, ctx, id)
				require.Equal(t, tc.wantBucket, meta.BucketName, "bucket_name must record the bucket written to")
				require.Equal(t, "S3", meta.StorageProvider)
				h := string(meta.Base64Hash)
				hashPrefix := h[0:1] + "/" + h[1:2] + "/" + h[2:] + "/"
				require.True(t, strings.HasPrefix(meta.ObjectKey, hashPrefix),
					"object_key must record the hash-prefixed key written to: %q", meta.ObjectKey)
				require.False(t, strings.HasPrefix(meta.ObjectKey, "/"), "key must be bucket-relative: %q", meta.ObjectKey)
				require.NotContains(t, meta.ObjectKey, string(env.cfg.AbsBasePath), "key must not embed the pod-local base path")
				if tc.wantSuffix != "" {
					require.True(t, strings.HasSuffix(meta.ObjectKey, tc.wantSuffix))
				} else {
					require.NotEqual(t, hashPrefix+"file", meta.ObjectKey, "encrypted objects are keyed per upload, not by content")
				}

				s.requireObject(t, ctx, tc.wantBucket, meta.ObjectKey, true)
				legacyKey, err := utils.GetPathFromBase64Hash(meta.Base64Hash, env.cfg.AbsBasePath)
				require.NoError(t, err)
				s.requireObject(t, ctx, tc.wantBucket, legacyKey, false)

				require.Equal(t, content, env.download(t, ctx, id))
			})
		}
	})
}

func (s *MediaStorageS3TestSuite) TestUploadFile_EncryptedDuplicatesAcrossOwnersStayReadable() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		env := s.newEnv(t, dep)
		content := []byte("same private plaintext " + util.IDString())

		idA := env.upload(t, env.ctx, "@a:example.com", content, false)
		idB := env.upload(t, env.ctx, "@b:example.com", content, false)
		require.NotEqual(t, idA, idB)

		metaA, metaB := env.metadata(t, env.ctx, idA), env.metadata(t, env.ctx, idB)
		require.Equal(t, metaA.Base64Hash, metaB.Base64Hash)
		require.NotEqual(t, metaA.ObjectKey, metaB.ObjectKey,
			"ciphertexts sealed with different data keys must not share an object")

		require.Equal(t, content, env.download(t, env.ctx, idA))
		require.Equal(t, content, env.download(t, env.ctx, idB))
	})
}

// legacyRow stores content the way releases before object_key did: under the
// absolute hash path in the visibility bucket, with bucket_name and
// object_key left empty on the row.
func (s *MediaStorageS3TestSuite) legacyRow(t *testing.T, env *s3Env, content []byte, isPublic bool) (types.MediaID, string, string) {
	t.Helper()
	ctx := env.ctx
	hash := types.Base64Hash(strings.ReplaceAll(util.IDString(), ".", "") + "legacyhash")
	meta := &types.MediaMetadata{
		MediaID:       types.MediaID(util.IDString()),
		Base64Hash:    hash,
		FileSizeBytes: types.FileSizeBytes(len(content)),
		ContentType:   "application/octet-stream",
		UploadName:    "legacy.bin",
		OwnerID:       "@legacy:example.com",
		IsPublic:      isPublic,
	}

	body := content
	if !isPublic {
		var sealed bytes.Buffer
		info, err := storage.EncryptStream(ctx, bytes.NewReader(content), &sealed, []byte(env.cfg.EnvStorageEncryptionPhrase))
		require.NoError(t, err)
		meta.Encryption = info
		body = sealed.Bytes()
	}

	bucket := env.prov.GetBucket(isPublic)
	key, err := utils.GetPathFromBase64Hash(hash, env.cfg.AbsBasePath)
	require.NoError(t, err)
	require.NoError(t, s.minio.PutObject(ctx, bucket, key, body))

	row := &models.MediaMetadata{}
	row.Fill(meta)
	require.NoError(t, env.db.MediaRepository.Create(ctx, row))
	stored := env.metadata(t, ctx, meta.MediaID)
	require.Empty(t, stored.BucketName, "precondition: legacy row has no bucket_name")
	require.Empty(t, stored.ObjectKey, "precondition: legacy row has no object_key")
	return meta.MediaID, bucket, key
}

func (s *MediaStorageS3TestSuite) TestLegacyRows_StayReadableAndDeletable() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		env := s.newEnv(t, dep)

		for _, isPublic := range []bool{false, true} {
			name := map[bool]string{false: "private", true: "public"}[isPublic]
			t.Run(name, func(t *testing.T) {
				content := []byte("legacy " + name + " " + util.IDString())
				id, bucket, key := s.legacyRow(t, env, content, isPublic)

				require.Equal(t, content, env.download(t, env.ctx, id), "legacy objects must remain readable")

				require.NoError(t, env.service.DeleteMedia(env.ctx, id, env.cfg))
				s.requireObject(t, env.ctx, bucket, key, false)
			})
		}
	})
}

func (s *MediaStorageS3TestSuite) TestDeleteMedia_RemovesObjectRowAndThumbnails() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		env := s.newEnv(t, dep)

		testCases := []struct {
			name     string
			isPublic bool
			// Thumbnails are generated only for public media here: decoding
			// an encrypted source for thumbnailing is not supported yet.
			withThumbnail bool
		}{
			{name: "private", isPublic: false},
			{name: "public_with_thumbnail", isPublic: true, withThumbnail: true},
		}

		for i, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx := env.ctx
				id := env.upload(t, ctx, "@owner-"+tc.name+":example.com", jpegPayload(t, 128, uint8(i)), tc.isPublic)
				meta := env.metadata(t, ctx, id)
				s.requireObject(t, ctx, meta.BucketName, meta.ObjectKey, true)

				var thumb *types.MediaMetadata
				if tc.withThumbnail {
					thumbRes, err := env.service.DownloadFile(ctx, &DownloadRequest{
						MediaID: id, IsThumbnailRequest: true, Config: env.cfg,
						ThumbnailSize: &types.ThumbnailSize{Width: 32, Height: 32, ResizeMethod: types.Scale},
					})
					require.NoError(t, err)
					util.CloseAndLogOnError(ctx, thumbRes.FileData)
					thumbs, err := env.db.GetThumbnails(ctx, id)
					require.NoError(t, err)
					require.Len(t, thumbs, 1)
					thumb = thumbs[0].MediaMetadata
					require.NotEmpty(t, thumb.ObjectKey, "thumbnails record their key too")
					require.Equal(t, meta.BucketName, thumb.BucketName)
					s.requireObject(t, ctx, thumb.BucketName, thumb.ObjectKey, true)
				}

				require.NoError(t, env.service.DeleteMedia(ctx, id, env.cfg))

				s.requireObject(t, ctx, meta.BucketName, meta.ObjectKey, false)
				gone, err := env.db.GetMediaMetadata(ctx, id)
				require.NoError(t, err)
				require.Nil(t, gone, "row must be deleted")
				if thumb != nil {
					s.requireObject(t, ctx, thumb.BucketName, thumb.ObjectKey, false)
					thumbsAfter, thumbErr := env.db.GetThumbnails(ctx, id)
					require.NoError(t, thumbErr)
					require.Empty(t, thumbsAfter, "thumbnail rows must be deleted")
				}

				require.ErrorIs(t, env.service.DeleteMedia(ctx, id, env.cfg), ErrMediaNotFound)
			})
		}
	})
}

func (s *MediaStorageS3TestSuite) TestDeleteMedia_KeepsObjectSharedAcrossPartitions() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		env := s.newEnv(t, dep)
		ctxA := partitionCtx(env.ctx, "tenant-share-a", "partition-share-a")
		ctxB := partitionCtx(env.ctx, "tenant-share-b", "partition-share-b")
		content := []byte("shared public content " + util.IDString())

		idA := env.upload(t, ctxA, "@a:example.com", content, true)
		idB := env.upload(t, ctxB, "@b:example.com", content, true)
		metaA, metaB := env.metadata(t, ctxA, idA), env.metadata(t, ctxB, idB)
		require.Equal(t, metaA.ObjectKey, metaB.ObjectKey, "identical public content is stored once")

		require.NoError(t, env.service.DeleteMedia(ctxA, idA, env.cfg))
		s.requireObject(t, env.ctx, metaB.BucketName, metaB.ObjectKey, true)
		require.Equal(t, content, env.download(t, ctxB, idB),
			"another partition's row still references the object, so it must survive")

		require.NoError(t, env.service.DeleteMedia(ctxB, idB, env.cfg))
		s.requireObject(t, env.ctx, metaB.BucketName, metaB.ObjectKey, false)
	})
}

func jpegPayload(t *testing.T, size int, seed uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: seed, A: 255}) //nolint:misspell,gosec
		}
	}
	buf := new(bytes.Buffer)
	require.NoError(t, jpeg.Encode(buf, img, nil))
	return buf.Bytes()
}
