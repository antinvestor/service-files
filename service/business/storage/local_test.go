package storage

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestProviderLocal_UploadFile(t *testing.T) {

	ctx := context.Background()

	provider, err := GetStorageProvider(ctx, "LOCAL")
	if err != nil {
		t.Errorf("A file provider has issues instantiating : %v", err)
	}

	bucketName := "/tmp/test"

	err = os.MkdirAll(bucketName, 0755)
	if err != nil {
		t.Errorf(" Couldn't make directory : %v", err)
		return
	}
	fileName := fmt.Sprintf("ts_%d", time.Now().Nanosecond())
	fileContent := []byte("Testing messages randomly")

	_, err = provider.UploadFile(ctx, bucketName, fileName, "txt", fileContent)
	if err != nil {
		t.Errorf(" Upload file experienced issues : %v", err)
	}

	content, err := provider.DownloadFile(ctx, bucketName, fileName, "txt")
	if err != nil {
		t.Errorf(" Download file experienced issues : %v", err)
	}

	if !bytes.Equal(fileContent, content) {
		t.Error("The contents of our file are not matching")
	}

}
