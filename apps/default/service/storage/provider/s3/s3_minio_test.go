package s3_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider/s3"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	tcminio "github.com/testcontainers/testcontainers-go/modules/minio"
	"gocloud.dev/gcerrors"
)

const (
	// MinIO stopped publishing images to Docker Hub and Quay; Chainguard
	// ships a maintained build whose entrypoint is the minio binary.
	minioImage     = "cgr.dev/chainguard/minio:latest"
	minioRegion    = "us-east-1"
	privateBucket  = "files-private"
	publicBucket   = "files-public"
	providerS3Name = "S3"
)

// S3MinioTestSuite exercises ProviderS3 against a real S3-compatible server so
// that every provider operation is proven to reach the object store instead of
// the local filesystem.
type S3MinioTestSuite struct {
	suite.Suite

	container *tcminio.MinioContainer
	endpoint  string
	rawClient *awss3.Client
}

func TestS3MinioTestSuite(t *testing.T) {
	suite.Run(t, new(S3MinioTestSuite))
}

func (s *S3MinioTestSuite) SetupSuite() {
	ctx := context.Background()

	container, err := tcminio.Run(ctx, minioImage)
	s.Require().NoError(err, "start minio container")
	s.container = container

	conn, err := container.ConnectionString(ctx)
	s.Require().NoError(err)
	s.endpoint = "http://" + conn

	s.rawClient = awss3.NewFromConfig(aws.Config{
		Credentials:  credentials.NewStaticCredentialsProvider(container.Username, container.Password, ""),
		BaseEndpoint: aws.String(s.endpoint),
		Region:       minioRegion,
	}, func(o *awss3.Options) { o.UsePathStyle = true })

	for _, b := range []string{privateBucket, publicBucket} {
		_, err = s.rawClient.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(b)})
		s.Require().NoError(err, "create bucket %s", b)
	}
}

func (s *S3MinioTestSuite) TearDownSuite() {
	if s.container != nil {
		s.Require().NoError(s.container.Terminate(context.Background()))
	}
}

func (s *S3MinioTestSuite) newProvider(t *testing.T) storage.Provider {
	t.Helper()
	p := s3.NewProvider(providerS3Name, privateBucket, publicBucket,
		s.endpoint, minioRegion, s.container.Password, "", s.container.Username)
	require.NoError(t, p.Setup(t.Context()))
	return p
}

func writeTempFile(t *testing.T, content []byte) types.Path {
	t.Helper()
	path := filepath.Join(t.TempDir(), "upload.bin")
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return types.Path(path)
}

// objectBody reads the object straight from MinIO, bypassing the provider, so
// the assertion holds only if the provider really wrote to the object store.
func (s *S3MinioTestSuite) objectBody(ctx context.Context, bucket, key string) ([]byte, error) {
	out, err := s.rawClient.GetObject(ctx, &awss3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	defer util.CloseAndLogOnError(ctx, out.Body)
	return io.ReadAll(out.Body)
}

func (s *S3MinioTestSuite) TestUploadDownloadRoundTrip_UsesBucketForVisibility() {
	testCases := []struct {
		name       string
		isPublic   bool
		wantBucket string
		otherBkt   string
		// Keys mirror production paths: absolute, hash-derived paths under
		// the service base path, which fileblob rejects as escaping its root.
		key string
	}{
		{name: "private_object", isPublic: false, wantBucket: privateBucket, otherBkt: publicBucket,
			key: "/tmp/files/ab/cd/private-" + util.IDString() + "/file"},
		{name: "public_object", isPublic: true, wantBucket: publicBucket, otherBkt: privateBucket,
			key: "/tmp/files/ab/cd/public-" + util.IDString() + "/file"},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			t := s.T()
			ctx := t.Context()
			p := s.newProvider(t)
			content := []byte(fmt.Sprintf("payload for %s %s", tc.name, util.IDString()))

			bucket := p.GetBucket(tc.isPublic)
			require.Equal(t, tc.wantBucket, bucket)

			duplicate, err := p.UploadFile(ctx, bucket, writeTempFile(t, content), types.Path(tc.key))
			require.NoError(t, err, "upload must reach S3")
			require.False(t, duplicate, "first upload is not a duplicate")

			stored, err := s.objectBody(ctx, tc.wantBucket, tc.key)
			require.NoError(t, err, "object must exist in %s", tc.wantBucket)
			require.Equal(t, content, stored)

			_, err = s.objectBody(ctx, tc.otherBkt, tc.key)
			require.Error(t, err, "object must not leak into %s", tc.otherBkt)

			duplicate, err = p.UploadFile(ctx, bucket, writeTempFile(t, content), types.Path(tc.key))
			require.NoError(t, err)
			require.True(t, duplicate, "re-upload of an existing key reports duplicate")

			reader, finish, err := p.DownloadFile(ctx, bucket, types.Path(tc.key))
			require.NoError(t, err, "download must read from S3")
			got, err := io.ReadAll(reader)
			finish()
			require.NoError(t, err)
			require.Equal(t, content, got)

			rs, err := storage.NewBlobReadSeeker(ctx, p, bucket, types.Path(tc.key), int64(len(content)))
			require.NoError(t, err)
			_, err = rs.Seek(8, io.SeekStart)
			require.NoError(t, err)
			tail, err := io.ReadAll(rs)
			require.NoError(t, rs.Close())
			require.NoError(t, err)
			require.Equal(t, content[8:], tail, "range reads go through the S3 bucket")
		})
	}
}

func (s *S3MinioTestSuite) TestDownloadFile_MissingObjectIsNotFound() {
	t := s.T()
	p := s.newProvider(t)

	_, _, err := p.DownloadFile(t.Context(), privateBucket, types.Path("missing/"+util.IDString()))
	require.Error(t, err)
	require.Equal(t, gcerrors.NotFound, gcerrors.Code(err))
}

func (s *S3MinioTestSuite) TestDelete_RemovesObjectFromS3() {
	t := s.T()
	ctx := t.Context()
	p := s.newProvider(t)
	key := "delete/" + util.IDString()
	content := []byte("to be deleted")

	_, err := p.UploadFile(ctx, privateBucket, writeTempFile(t, content), types.Path(key))
	require.NoError(t, err)

	bucket, err := p.Init(ctx, privateBucket)
	require.NoError(t, err)
	defer util.CloseAndLogOnError(ctx, bucket)

	require.NoError(t, bucket.Delete(ctx, key))

	_, err = s.objectBody(ctx, privateBucket, key)
	require.Error(t, err, "object must be gone from S3 after delete")

	_, _, err = p.DownloadFile(ctx, privateBucket, types.Path(key))
	require.Equal(t, gcerrors.NotFound, gcerrors.Code(err))
}

func (s *S3MinioTestSuite) TestUploadFile_UnknownBucketFails() {
	t := s.T()
	p := s.newProvider(t)

	_, err := p.UploadFile(t.Context(), "bucket-does-not-exist", writeTempFile(t, []byte("x")), types.Path("k"))
	require.Error(t, err, "upload to a missing S3 bucket must surface an error")
}
