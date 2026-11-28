// Package blobprovider holds the bucket-agnostic upload/download logic shared by
// every storage provider. Each concrete provider supplies only how a bucket is
// opened; all object operations go through that opener so a provider can never
// silently fall back to another backend's bucket.
package blobprovider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/util"
	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"
)

// BucketOpener opens the named bucket on a concrete backend.
type BucketOpener func(ctx context.Context, bucketName string) (*blob.Bucket, error)

// Provider implements the storage.Provider object operations on top of a
// BucketOpener. Concrete providers embed it and add their own Setup.
type Provider struct {
	name          string
	privateBucket string
	publicBucket  string
	open          BucketOpener
}

// New returns a Provider whose every bucket access goes through open.
func New(name, privateBucket, publicBucket string, open BucketOpener) *Provider {
	return &Provider{
		name:          name,
		privateBucket: privateBucket,
		publicBucket:  publicBucket,
		open:          open,
	}
}

func (p *Provider) Name() string { return p.name }

func (p *Provider) PublicBucket() string { return p.publicBucket }

func (p *Provider) PrivateBucket() string { return p.privateBucket }

func (p *Provider) GetBucket(isPublic bool) string {
	if isPublic {
		return p.publicBucket
	}
	return p.privateBucket
}

// Init opens bucketName on the provider's backend.
func (p *Provider) Init(ctx context.Context, bucketName string) (*blob.Bucket, error) {
	if p.open == nil {
		return nil, errors.New("storage provider has no bucket opener")
	}
	return p.open(ctx, bucketName)
}

// UploadFile copies sourcePath to inBucketPath. It reports true without writing
// when the object already exists (content-addressed paths make that a duplicate).
func (p *Provider) UploadFile(ctx context.Context, bucketName string, sourcePath types.Path, inBucketPath types.Path) (bool, error) {
	bucket, err := p.Init(ctx, bucketName)
	if err != nil {
		return false, fmt.Errorf("open bucket %q: %w", bucketName, err)
	}
	defer util.CloseAndLogOnError(ctx, bucket)

	key := string(inBucketPath)
	exists, err := bucket.Exists(ctx, key)
	if err != nil {
		return false, fmt.Errorf("check object %q: %w", key, err)
	}
	if exists {
		return true, nil
	}

	src, err := os.Open(string(sourcePath))
	if err != nil {
		return false, fmt.Errorf("open upload source: %w", err)
	}
	defer util.CloseAndLogOnError(ctx, src)

	// Cancelling writeCtx before Close aborts the write, so a failed copy never
	// leaves a truncated object behind under a content-addressed key.
	writeCtx, cancelWrite := context.WithCancel(ctx)
	defer cancelWrite()

	w, err := bucket.NewWriter(writeCtx, key, nil)
	if err != nil {
		return false, fmt.Errorf("create object writer %q: %w", key, err)
	}

	if _, err = w.ReadFrom(src); err != nil {
		cancelWrite()
		_ = w.Close()
		return false, fmt.Errorf("write object %q: %w", key, err)
	}

	// Remote backends commit the object on Close; its error is the upload result.
	if err = w.Close(); err != nil {
		return false, fmt.Errorf("commit object %q: %w", key, err)
	}

	return false, nil
}

// DownloadFile opens a reader for inBucketPath. The returned finaliser releases
// both the reader and the bucket and must be called once reading is done.
func (p *Provider) DownloadFile(ctx context.Context, bucketName string, inBucketPath types.Path) (io.Reader, func(), error) {
	bucket, err := p.Init(ctx, bucketName)
	if err != nil {
		return nil, nil, fmt.Errorf("open bucket %q: %w", bucketName, err)
	}

	r, err := bucket.NewReader(ctx, string(inBucketPath), nil)
	if err != nil {
		util.CloseAndLogOnError(ctx, bucket)
		return nil, nil, fmt.Errorf("open object %q: %w", inBucketPath, err)
	}

	return r, func() {
		util.CloseAndLogOnError(ctx, r)
		util.CloseAndLogOnError(ctx, bucket)
	}, nil
}

// DeleteFile removes inBucketPath from the bucket. An object that is already
// gone counts as deleted, so a retried delete succeeds.
func (p *Provider) DeleteFile(ctx context.Context, bucketName string, inBucketPath types.Path) error {
	bucket, err := p.Init(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("open bucket %q: %w", bucketName, err)
	}
	defer util.CloseAndLogOnError(ctx, bucket)

	if err = bucket.Delete(ctx, string(inBucketPath)); err != nil && gcerrors.Code(err) != gcerrors.NotFound {
		return fmt.Errorf("delete object %q: %w", inBucketPath, err)
	}
	return nil
}
