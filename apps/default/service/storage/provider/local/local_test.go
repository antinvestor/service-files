package local_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/internal/tests"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/tests/testdef"
	"github.com/stretchr/testify/suite"
)

type LocalProviderTestSuite struct {
	tests.BaseTestSuite
}

func TestLocalProviderTestSuite(t *testing.T) {
	suite.Run(t, new(LocalProviderTestSuite))
}

func (suite *LocalProviderTestSuite) TestProviderLocal_UploadFile() {
	testCases := []struct {
		name       string
		bucketName string
		data       []byte
	}{
		{
			name:       "upload and download file successfully",
			bucketName: "/tmp/test",
			data:       []byte("This is some sample data written to the temporary file."),
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *testdef.DependancyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.Background()

				cfg, err := frame.ConfigFromEnv[config.FilesConfig]()
				if err != nil {
					t.Errorf("Could not get file config : %v", err)
				}

				storageProvider, err := provider.GetStorageProvider(ctx, &cfg)
				if err != nil {
					t.Errorf("A file storageProvider has issues instantiating : %v", err)
				}

				tmpFile, err := os.CreateTemp("", "example-*.txt")
				if err != nil {
					t.Errorf("Error creating temp file: %v", err)
					return
				}
				defer os.Remove(tmpFile.Name()) // Ensure file cleanup

				// Write some data to the file
				if _, err = tmpFile.Write(tc.data); err != nil {
					t.Errorf("Error writing to temp file: %v", err)
				}

				// Flush data by closing the file
				if err = tmpFile.Close(); err != nil {
					t.Errorf("Error closing temp file: %v", err)
				}

				err = os.MkdirAll(tc.bucketName, 0755)
				if err != nil {
					t.Errorf(" Couldn't make directory : %v", err)
					return
				}
				fileName := fmt.Sprintf("ts_%d.txt", time.Now().Nanosecond())

				_, err = storageProvider.UploadFile(ctx, tc.bucketName, types.Path(tmpFile.Name()), types.Path(fileName))
				if err != nil {
					t.Errorf(" Upload file experienced issues : %v", err)
				}

				reader, finisher, err := storageProvider.DownloadFile(ctx, tc.bucketName, types.Path(fileName))
				if err != nil {
					t.Errorf(" Download file experienced issues : %v", err)
				}
				defer finisher()

				var buf bytes.Buffer
				_, err = io.Copy(&buf, reader)
				if err != nil {
					t.Errorf("Download file experienced issues: %v", err)
				}
				content := buf.Bytes()

				if !bytes.Equal(tc.data, content) {
					t.Error("The contents of our file are not matching")
				}

				_ = os.Remove(filepath.Join(tc.bucketName, fileName))
			})
		}
	})
}
