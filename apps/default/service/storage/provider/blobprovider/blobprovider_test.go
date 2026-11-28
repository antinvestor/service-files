package blobprovider_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/storage/provider/blobprovider"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob"
	"gocloud.dev/blob/fileblob"
)

// recordingOpener maps logical bucket names to directories and records which
// names were requested, so tests can prove every operation is routed through
// the opener supplied by the concrete provider rather than some other backend.
type recordingOpener struct {
	dirs   map[string]string
	opened []string
}

func (m *recordingOpener) open(_ context.Context, name string) (*blob.Bucket, error) {
	m.opened = append(m.opened, name)
	dir, ok := m.dirs[name]
	if !ok {
		return nil, errors.New("no such bucket: " + name)
	}
	return fileblob.OpenBucket(dir, nil)
}

// inspect opens an independent handle on dir for assertions.
func inspect(t *testing.T, dir string) *blob.Bucket {
	t.Helper()
	b, err := fileblob.OpenBucket(dir, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })
	return b
}

func writeSource(t *testing.T, data []byte) types.Path {
	t.Helper()
	path := filepath.Join(t.TempDir(), "src")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return types.Path(path)
}

func TestProvider_AllOperationsUseSuppliedOpener(t *testing.T) {
	ctx := t.Context()
	privDir, pubDir := t.TempDir(), t.TempDir()
	priv, pub := inspect(t, privDir), inspect(t, pubDir)
	op := &recordingOpener{dirs: map[string]string{"priv": privDir, "pub": pubDir}}
	p := blobprovider.New("MEM", "priv", "pub", op.open)

	testCases := []struct {
		name     string
		isPublic bool
		target   *blob.Bucket
		other    *blob.Bucket
	}{
		{name: "private", isPublic: false, target: priv, other: pub},
		{name: "public", isPublic: true, target: pub, other: priv},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte("content-" + tc.name)
			key := "hash/path/" + tc.name
			bucket := p.GetBucket(tc.isPublic)

			dup, err := p.UploadFile(ctx, bucket, writeSource(t, data), types.Path(key))
			require.NoError(t, err)
			require.False(t, dup)

			stored, err := tc.target.ReadAll(ctx, key)
			require.NoError(t, err)
			require.Equal(t, data, stored)
			exists, err := tc.other.Exists(ctx, key)
			require.NoError(t, err)
			require.False(t, exists)

			dup, err = p.UploadFile(ctx, bucket, writeSource(t, data), types.Path(key))
			require.NoError(t, err)
			require.True(t, dup)

			r, finish, err := p.DownloadFile(ctx, bucket, types.Path(key))
			require.NoError(t, err)
			got, err := io.ReadAll(r)
			finish()
			require.NoError(t, err)
			require.Equal(t, data, got)
		})
	}

	require.Equal(t, []string{"priv", "priv", "priv", "pub", "pub", "pub"}, op.opened)
}

func TestProvider_ErrorsPropagate(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	mem := inspect(t, dir)
	op := &recordingOpener{dirs: map[string]string{"ok": dir}}
	p := blobprovider.New("MEM", "ok", "ok", op.open)

	testCases := []struct {
		name string
		run  func() error
	}{
		{name: "upload_unknown_bucket", run: func() error {
			_, err := p.UploadFile(ctx, "missing", writeSource(t, []byte("x")), "k")
			return err
		}},
		{name: "upload_missing_source", run: func() error {
			_, err := p.UploadFile(ctx, "ok", types.Path(filepath.Join(t.TempDir(), "nope")), "k2")
			return err
		}},
		{name: "download_missing_object", run: func() error {
			_, _, err := p.DownloadFile(ctx, "ok", "absent")
			return err
		}},
		{name: "nil_opener", run: func() error {
			_, err := blobprovider.New("X", "a", "b", nil).Init(ctx, "a")
			return err
		}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Error(t, tc.run())
		})
	}

	exists, err := mem.Exists(ctx, "k2")
	require.NoError(t, err)
	require.False(t, exists, "failed upload must not leave an object behind")
}

func TestProvider_DeleteFile(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	mem := inspect(t, dir)
	op := &recordingOpener{dirs: map[string]string{"bkt": dir}}
	p := blobprovider.New("MEM", "bkt", "bkt", op.open)

	testCases := []struct {
		name    string
		bucket  string
		seed    bool
		wantErr bool
	}{
		{name: "removes_existing_object", bucket: "bkt", seed: true},
		{name: "missing_object_is_already_deleted", bucket: "bkt"},
		{name: "unknown_bucket_fails", bucket: "missing", wantErr: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := "a/b/" + tc.name + "/file"
			if tc.seed {
				_, err := p.UploadFile(ctx, tc.bucket, writeSource(t, []byte("x")), types.Path(key))
				require.NoError(t, err)
			}

			err := p.DeleteFile(ctx, tc.bucket, types.Path(key))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			exists, err := mem.Exists(ctx, key)
			require.NoError(t, err)
			require.False(t, exists, "object must be gone after DeleteFile")
		})
	}
}
