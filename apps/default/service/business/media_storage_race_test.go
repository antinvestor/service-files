package business

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider/s3"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/v3/frametests/definition"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
)

// gatedProvider pauses the first DeleteFile call until released, so a test
// can run other operations while a delete is in flight.
type gatedProvider struct {
	storage.Provider

	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func newGatedProvider(p storage.Provider) *gatedProvider {
	return &gatedProvider{Provider: p, entered: make(chan struct{}), release: make(chan struct{})}
}

func (g *gatedProvider) DeleteFile(ctx context.Context, bucket string, key types.Path) error {
	first := false
	g.once.Do(func() { first = true })
	if first {
		close(g.entered)
		select {
		case <-g.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return g.Provider.DeleteFile(ctx, bucket, key)
}

func (s *MediaStorageS3TestSuite) s3Provider(t *testing.T, ctx context.Context) storage.Provider {
	t.Helper()
	prov := s3.NewProvider("S3", s3PrivateBucket, s3PublicBucket, s.minio.Endpoint, tests.MinioRegion,
		s.minio.Container.Password, "", s.minio.Container.Username)
	require.NoError(t, prov.Setup(ctx))
	return prov
}

func (s *MediaStorageS3TestSuite) envWithProvider(t *testing.T, dep *definition.DependencyOption, wrap func(storage.Provider) storage.Provider) *s3Env {
	t.Helper()
	ctx, svc, res := s.CreateService(t, dep)
	cfg := svc.Config().(*config.FilesConfig)
	prov := wrap(s.s3Provider(t, ctx))
	db := &connection.Database{WorkManager: svc.WorkManager(), MediaRepository: res.MediaRepository}
	return &s3Env{ctx: ctx, cfg: cfg, db: db, service: NewMediaService(db, prov), prov: prov}
}

// A replica that has not yet replicated another partition's reference must
// not make a shared object look unreferenced: the reference count has to be
// read from the primary.
func (s *MediaStorageS3TestSuite) TestDeleteMedia_CountsReferencesOnPrimary() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, svc, res, replicaRes := s.CreateServiceWithReplica(t, dep)
		cfg := svc.Config().(*config.FilesConfig)
		prov := s.s3Provider(t, ctx)
		db := &connection.Database{WorkManager: svc.WorkManager(), MediaRepository: res.MediaRepository}
		service := NewMediaService(db, prov)
		env := &s3Env{ctx: ctx, cfg: cfg, db: db, service: service, prov: prov}

		content := []byte("replicated shared content " + util.IDString())
		idA := env.upload(t, ctx, "@a:example.com", content, true)
		idB := env.upload(t, ctx, "@b:example.com", content, true)

		primary := res.MediaRepository.Pool().DB(ctx, false)
		rowA, rowB := &models.MediaMetadata{}, &models.MediaMetadata{}
		require.NoError(t, primary.First(rowA, "id = ?", string(idA)).Error)
		require.NoError(t, primary.First(rowB, "id = ?", string(idB)).Error)
		require.Equal(t, rowA.ObjectKey, rowB.ObjectKey, "precondition: A and B share one object")

		// The replica has caught up with A but not yet with B.
		require.NoError(t, replicaRes.MediaRepository.Pool().DB(ctx, false).Create(rowA).Error)
		metaB := rowB

		require.NoError(t, service.DeleteMedia(ctx, idA, cfg))
		s.requireObject(t, ctx, metaB.BucketName, metaB.ObjectKey, true)
	})
}

// A delete that decided the object is unreferenced must not race an upload
// that dedups against that same object: the upload either lands before the
// count (and keeps the object alive) or after the delete (and rewrites it).
func (s *MediaStorageS3TestSuite) TestDeleteMedia_DoesNotStrandConcurrentDedupUpload() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		var gate *gatedProvider
		env := s.envWithProvider(t, dep, func(p storage.Provider) storage.Provider {
			gate = newGatedProvider(p)
			return gate
		})
		ctx := env.ctx
		content := []byte("racing public content " + util.IDString())

		idA := env.upload(t, ctx, "@a:example.com", content, true)
		metaA := env.metadata(t, ctx, idA)

		deleteDone := make(chan error, 1)
		go func() { deleteDone <- env.service.DeleteMedia(ctx, idA, env.cfg) }()
		<-gate.entered // A's delete has counted zero references and is removing the object.

		type uploadOutcome struct {
			res *UploadResult
			err error
		}
		uploadDone := make(chan uploadOutcome, 1)
		go func() {
			res, err := env.service.UploadFile(ctx, &UploadRequest{
				OwnerID:       "@b:example.com",
				UploadName:    "file.bin",
				ContentType:   "application/octet-stream",
				FileSizeBytes: types.FileSizeBytes(len(content)),
				FileData:      bytes.NewReader(content),
				Config:        env.cfg,
				IsPublic:      true,
			})
			uploadDone <- uploadOutcome{res: res, err: err}
		}()

		select {
		case <-uploadDone:
			t.Fatal("upload must wait for the in-flight delete of the object it dedups against")
		case <-time.After(500 * time.Millisecond):
		}

		close(gate.release)
		require.NoError(t, <-deleteDone)
		outcome := <-uploadDone
		require.NoError(t, outcome.err)
		idB := outcome.res.MediaID

		metaB := env.metadata(t, ctx, idB)
		require.Equal(t, metaA.ObjectKey, metaB.ObjectKey)
		s.requireObject(t, ctx, metaB.BucketName, metaB.ObjectKey, true)
		require.Equal(t, content, env.download(t, ctx, idB))
	})
}

// Concurrent deletes of every row sharing an object must remove the object
// exactly when the last reference goes, never leaking it.
func (s *MediaStorageS3TestSuite) TestDeleteMedia_ConcurrentDeletesRemoveSharedObject() {
	s.WithTestDependancies(s.T(), func(t *testing.T, dep *definition.DependencyOption) {
		env := s.newEnv(t, dep)
		ctx := env.ctx

		const rounds, sharers = 5, 4
		for round := 0; round < rounds; round++ {
			content := []byte("concurrently deleted " + util.IDString())
			ids := make([]types.MediaID, 0, sharers)
			for i := 0; i < sharers; i++ {
				ids = append(ids, env.upload(t, ctx, "@sharer-"+util.IDString()+":example.com", content, true))
			}
			meta := env.metadata(t, ctx, ids[0])

			start := make(chan struct{})
			errs := make(chan error, sharers)
			var wg sync.WaitGroup
			for _, id := range ids {
				wg.Add(1)
				go func(id types.MediaID) {
					defer wg.Done()
					<-start
					errs <- env.service.DeleteMedia(ctx, id, env.cfg)
				}(id)
			}
			close(start)
			wg.Wait()
			close(errs)
			for err := range errs {
				require.NoError(t, err)
			}

			s.requireObject(t, ctx, meta.BucketName, meta.ObjectKey, false)
		}
	})
}
