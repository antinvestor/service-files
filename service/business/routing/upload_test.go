package routing

import (
	"context"
	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/storage"
	"github.com/antinvestor/service-files/service/storage/datastore"
	"github.com/antinvestor/service-files/service/storage/provider"

	"github.com/antinvestor/service-files/service/types"
	"github.com/antinvestor/service-files/testsutil"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pitabwire/util"
	log "github.com/sirupsen/logrus"
)

func Test_uploadRequest_doUpload(t *testing.T) {
	type fields struct {
		MediaMetadata *types.MediaMetadata
		Logger        *log.Entry
	}
	type args struct {
		ctx       context.Context
		ownerId   types.OwnerID
		reqReader io.Reader
		cfg       *config.FilesConfig
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Errorf("failed to get current working directory: %v", err)
	}

	maxSize := config.FileSizeBytes(8)
	logger := log.New().WithField("mediaapi", "test")
	testdataPath := filepath.Join(wd, "./testdata")

	cfg := &config.FilesConfig{
		MaxFileSizeBytes:  maxSize,
		BasePath:          config.Path(testdataPath),
		AbsBasePath:       config.Path(testdataPath),
		DynamicThumbnails: false,
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *util.JSONResponse
	}{
		{
			name: "upload ok",
			args: args{
				ctx:       context.Background(),
				ownerId:   types.OwnerID("test"),
				reqReader: strings.NewReader("test"),
				cfg:       cfg,
			},
			fields: fields{
				Logger: logger,
				MediaMetadata: &types.MediaMetadata{
					MediaID:    "1337",
					UploadName: "test ok",
				},
			},
			want: nil,
		},
		{
			name: "upload ok (exact size)",
			args: args{
				ctx:       context.Background(),
				ownerId:   types.OwnerID("test"),
				reqReader: strings.NewReader("testtest"),
				cfg:       cfg,
			},
			fields: fields{
				Logger: logger,
				MediaMetadata: &types.MediaMetadata{
					MediaID:    "1338",
					UploadName: "test ok (exact size)",
				},
			},
			want: nil,
		},
		{
			name: "upload not ok",
			args: args{
				ctx:       context.Background(),
				ownerId:   types.OwnerID("test"),
				reqReader: strings.NewReader("test test test"),
				cfg:       cfg,
			},
			fields: fields{
				Logger: logger,
				MediaMetadata: &types.MediaMetadata{
					MediaID:    "1339",
					UploadName: "test fail",
				},
			},
			want: requestEntityTooLargeJSONResponse(maxSize),
		},
		{
			name: "upload ok with unlimited filesize",
			args: args{
				ctx:       context.Background(),
				ownerId:   types.OwnerID("test"),
				reqReader: strings.NewReader("test test test"),
				cfg: &config.FilesConfig{
					MaxFileSizeBytes:  config.FileSizeBytes(0),
					BasePath:          config.Path(testdataPath),
					AbsBasePath:       config.Path(testdataPath),
					DynamicThumbnails: false,
				},
			},
			fields: fields{
				Logger: logger,
				MediaMetadata: &types.MediaMetadata{
					MediaID:    "1339",
					UploadName: "test fail",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, srv, cleanUpFunc, err := testsutil.GetTestServiceWithConfig(tt.args.ctx, "upload", tt.args.cfg)
			assert.NoErrorf(t, err, "failed to get test service")
			defer cleanUpFunc()

			db, err := datastore.NewMediaDatabase(srv)
			assert.NoErrorf(t, err, "failed to open media database")

			var storageProvider storage.Provider
			storageProvider, err = provider.GetStorageProvider(ctx, cfg)
			assert.NoErrorf(t, err, "failed to get a storage storageProvider to use")

			r := &uploadRequest{
				MediaMetadata: tt.fields.MediaMetadata,
				Logger:        tt.fields.Logger,
			}
			if got := r.doUpload(ctx, tt.args.ownerId, tt.args.reqReader, tt.args.cfg, db, storageProvider); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("doUpload() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
