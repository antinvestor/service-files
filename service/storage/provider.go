package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/types"
	"github.com/antinvestor/service-files/service/utils"
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
}

// UploadFileWithHashCheck checks for hash collisions when moving a temporary file to its final path based on metadata
// The final path is based on the hash of the file.
// If the final path exists and the file size matches, the file does not need to be moved.
// In error cases where the file is not a duplicate, the caller may decide to remove the final path.
// Returns the final path of the file, whether it is a duplicate and an error.
func UploadFileWithHashCheck(ctx context.Context, provider Provider, tmpDir types.Path, mediaMetadata *types.MediaMetadata, absBasePath config.Path, logger *util.LogEntry) (types.Path, bool, error) {
	// Note: in all error and success cases, we need to remove the temporary directory
	defer utils.RemoveDir(tmpDir, logger)
	duplicate := false
	finalPath, err := utils.GetPathFromBase64Hash(mediaMetadata.Base64Hash, absBasePath)
	if err != nil {
		return "", false, fmt.Errorf("failed to get file path from metadata: %w", err)
	}

	uploadBucket := provider.GetBucket(mediaMetadata.IsPublic)

	duplicate, err = provider.UploadFile(ctx, uploadBucket,
		types.Path(filepath.Join(string(tmpDir), "content")),
		types.Path(finalPath),
	)
	if err != nil {
		return "", duplicate, fmt.Errorf("failed to move file to final destination (%v): %w", finalPath, err)
	}
	return types.Path(finalPath), duplicate, nil
}
