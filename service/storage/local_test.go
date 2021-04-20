package storage

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestProviderLocal_UploadFile(t *testing.T) {

	ctx := context.Background()

	provider, err := GetStorageProvider(ctx, "LOCAL")
	assert.NoError(t, err, "A file provider should not have issues instantiating")

	bucketName := "/tmp/test"

	os.MkdirAll(bucketName, 0755)
	fileName := fmt.Sprintf("ts_%d", time.Now().Nanosecond())
	fileContent := []byte("Testing messages randomly")

	_, err = provider.UploadFile(ctx, bucketName , fileName,"txt", fileContent )
	assert.NoError(t, err, "File upload shouldn't have issues")

	content, err := provider.DownloadFile(ctx, bucketName, fileName, "txt")
	assert.NoError(t, err, "Error obtaining the contents of our file")

	assert.Equal(t,fileContent, content, "The contents of our file are not matching" )

}
