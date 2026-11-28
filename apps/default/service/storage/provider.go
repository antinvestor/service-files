package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/util"
	"gocloud.dev/blob"
)

type Provider interface {
	Name() string
	PrivateBucket() string
	PublicBucket() string
	GetBucket(isPublic bool) string
	Setup(ctx context.Context) error
	Init(ctx context.Context, bucketName string) (*blob.Bucket, error)
	UploadFile(ctx context.Context, bucket string, sourcePath types.Path, destinationPath types.Path) (bool, error)
	DownloadFile(ctx context.Context, bucket string, sourcePath types.Path) (io.Reader, func(), error)
	// DeleteFile removes the object; a missing object is not an error.
	DeleteFile(ctx context.Context, bucket string, path types.Path) error
}

// UploadFileWithHashCheck uploads sourcePath under the content's stable object
// key (see ContentObjectKey) in the bucket for its visibility, and records
// that location (bucket, key, provider) on mediaMetadata so it is persisted
// with the row and every later read and delete targets the same object.
// It reports whether the object already existed (a content-addressed duplicate).
func UploadFileWithHashCheck(ctx context.Context, provider Provider, sourcePath types.Path, mediaMetadata *types.MediaMetadata) (types.Path, bool, error) {
	key, err := ContentObjectKey(mediaMetadata)
	if err != nil {
		return "", false, err
	}

	bucket := provider.GetBucket(mediaMetadata.IsPublic)
	duplicate, err := provider.UploadFile(ctx, bucket, sourcePath, types.Path(key))
	if err != nil {
		return "", duplicate, fmt.Errorf("upload object %q to bucket %q: %w", key, bucket, err)
	}

	mediaMetadata.BucketName = bucket
	mediaMetadata.ObjectKey = key
	mediaMetadata.StorageProvider = provider.Name()
	return types.Path(key), duplicate, nil
}

// StoreMediaObject uploads sourcePath under mediaMetadata's object key and
// inserts its row. The insert runs under the object's advisory lock after
// re-checking the object is still present, so it cannot interleave with a
// ReleaseMediaObject that has decided the object is unreferenced: either the
// release sees this row and keeps the object, or it finishes first and the
// re-check writes the object again. If the insert fails, an object this call
// created is released again unless another row has come to reference it.
func StoreMediaObject(ctx context.Context, provider Provider, db MediaRepository, sourcePath types.Path, mediaMetadata *types.MediaMetadata) (bool, error) {
	key, duplicate, err := UploadFileWithHashCheck(ctx, provider, sourcePath, mediaMetadata)
	if err != nil {
		return duplicate, err
	}
	bucket := mediaMetadata.BucketName

	err = db.StoreMediaMetadataWithObjectLock(ctx, mediaMetadata, bucket, string(key), func(lockCtx context.Context) error {
		if _, upErr := provider.UploadFile(lockCtx, bucket, sourcePath, key); upErr != nil {
			return fmt.Errorf("re-check object %q: %w", key, upErr)
		}
		return nil
	})
	if err == nil {
		return duplicate, nil
	}

	if !duplicate {
		relErr := db.ReleaseMediaObject(ctx, mediaMetadata, bucket, string(key), false, func(relCtx context.Context) error {
			return provider.DeleteFile(relCtx, bucket, key)
		})
		if relErr != nil {
			util.Log(ctx).WithError(relErr).With("bucket", bucket, "key", key).Warn("failed to release object after metadata insert failed")
		}
	}
	return duplicate, fmt.Errorf("store media metadata: %w", err)
}
