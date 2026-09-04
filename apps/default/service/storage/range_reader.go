package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/util"
	"gocloud.dev/blob"
)

// BlobReadSeeker exposes a stored object as an io.ReadSeeker backed by blob range reads.
//
// It opens a range reader lazily on the first Read after a Seek, so serving a byte range
// via http.ServeContent costs a single ranged request against the bucket instead of
// streaming the whole object. Only plaintext objects can be served this way; encrypted
// objects must be decrypted sequentially through Provider.DownloadFile.
type BlobReadSeeker struct {
	ctx    context.Context
	bucket *blob.Bucket
	key    string
	size   int64

	offset int64
	reader *blob.Reader
}

// NewBlobReadSeeker opens bucketName on the provider and prepares a seekable view of key.
// size is the known object length (from media metadata); it is used to satisfy io.SeekEnd.
func NewBlobReadSeeker(ctx context.Context, provider Provider, bucketName string, key types.Path, size int64) (*BlobReadSeeker, error) {
	if size < 0 {
		return nil, fmt.Errorf("invalid object size %d", size)
	}
	bucket, err := provider.Init(ctx, bucketName)
	if err != nil {
		return nil, err
	}
	return &BlobReadSeeker{ctx: ctx, bucket: bucket, key: string(key), size: size}, nil
}

// Size returns the object length supplied at construction.
func (b *BlobReadSeeker) Size() int64 { return b.size }

// Read implements io.Reader.
func (b *BlobReadSeeker) Read(p []byte) (int, error) {
	if b.offset >= b.size {
		return 0, io.EOF
	}
	if b.reader == nil {
		r, err := b.bucket.NewRangeReader(b.ctx, b.key, b.offset, -1, nil)
		if err != nil {
			return 0, err
		}
		b.reader = r
	}
	n, err := b.reader.Read(p)
	b.offset += int64(n)
	return n, err
}

// Seek implements io.Seeker. Seeking discards any open range reader; the next Read reopens
// at the new offset.
func (b *BlobReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = b.offset + offset
	case io.SeekEnd:
		abs = b.size + offset
	default:
		return 0, errors.New("invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("negative position")
	}
	if abs != b.offset {
		b.closeReader()
	}
	b.offset = abs
	return abs, nil
}

// Close releases the range reader and the bucket handle.
func (b *BlobReadSeeker) Close() error {
	b.closeReader()
	return b.bucket.Close()
}

func (b *BlobReadSeeker) closeReader() {
	if b.reader != nil {
		util.CloseAndLogOnError(b.ctx, b.reader)
		b.reader = nil
	}
}
