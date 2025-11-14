package routing

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type UploadTestSuite struct {
	tests.BaseTestSuite
}

func TestUploadTestSuite(t *testing.T) {
	suite.Run(t, new(UploadTestSuite))
}

func (suite *UploadTestSuite) Test_uploadRequest_doUpload() {
	type fields struct {
		MediaMetadata *types.MediaMetadata
		Logger        *util.LogEntry
	}
	type args struct {
		ctx       context.Context
		ownerId   types.OwnerID
		reqReader io.Reader
		cfg       *config.FilesConfig
	}

	wd, err := os.Getwd()
	if err != nil {
		suite.T().Errorf("failed to get current working directory: %v", err)
	}

	maxSize := config.FileSizeBytes(8)
	logger := util.Log(suite.T().Context()).WithField("mediaapi", "test")
	testdataPath := filepath.Join(wd, "./testdata")

	cfg := &config.FilesConfig{
		MaxFileSizeBytes:  maxSize,
		BasePath:          config.Path(testdataPath),
		AbsBasePath:       config.Path(testdataPath),
		DynamicThumbnails: false,
	}

	testCases := []struct {
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

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tt := range testCases {
			t.Run(tt.name, func(t *testing.T) {
				ctx := tt.args.ctx

				// Get database connection from dependency
				_, svc, res := suite.CreateService(t, dep)
				db, err := connection.NewMediaDatabase(svc.WorkManager(), res.MediaRepository)
				assert.NoErrorf(t, err, "failed to open media database")

				var storageProvider storage.Provider
				storageProvider, err = provider.GetStorageProvider(ctx, tt.args.cfg)
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
	})
}
