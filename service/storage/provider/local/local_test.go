package local_test

import (
	"bytes"
	"context"
	"fmt"
	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/storage/provider"
	"github.com/antinvestor/service-files/service/types"
	"github.com/pitabwire/frame"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProviderLocal_UploadFile(t *testing.T) {

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
		fmt.Println("Error creating temp file:", err)
		return
	}
	defer os.Remove(tmpFile.Name()) // Ensure file cleanup

	// Write some data to the file
	data := []byte("This is some sample data written to the temporary file.")
	if _, err = tmpFile.Write(data); err != nil {
		t.Errorf("Error writing to temp file: %v", err)
	}

	// Flush data by closing the file
	if err = tmpFile.Close(); err != nil {
		t.Errorf("Error closing temp file: %v", err)
	}

	bucketName := "/tmp/test"

	err = os.MkdirAll(bucketName, 0755)
	if err != nil {
		t.Errorf(" Couldn't make directory : %v", err)
		return
	}
	fileName := fmt.Sprintf("ts_%d.txt", time.Now().Nanosecond())

	_, err = storageProvider.UploadFile(ctx, bucketName, types.Path(tmpFile.Name()), types.Path(fileName))
	if err != nil {
		t.Errorf(" Upload file experienced issues : %v", err)
	}

	reader, finisher, err := storageProvider.DownloadFile(ctx, bucketName, types.Path(fileName))
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

	if !bytes.Equal(data, content) {
		t.Error("The contents of our file are not matching")
	}

	_ = os.Remove(filepath.Join(bucketName, fileName))

}
