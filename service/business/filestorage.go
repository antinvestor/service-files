package business

import (
	"context"
	"github.com/antinvestor/service-files/service/business/storage"
	"github.com/antinvestor/service-files/service/models"
	"github.com/antinvestor/service-files/service/utils"
	"path/filepath"
)

// FileUpload - Abstract way to upload a file to any implemented storage provider
func FileUpload(ctx context.Context, storageProvider storage.Provider, encryptionPhrase string,
	isPublic bool, subscriptionID string, hash string, extension string, contents []byte) (bucket string, result string, err error) {

	filePathName := filepath.Join(subscriptionID, hash)

	if !isPublic {
		hashedContent, err := utils.Encrypt(contents, encryptionPhrase)
		if err != nil {
			return storageProvider.PrivateBucket(), "", err
		}

		result, err := storageProvider.UploadFile(ctx, storageProvider.PrivateBucket(), filePathName, extension, hashedContent)
		return storageProvider.PrivateBucket(), result, err
	}

	result, err = storageProvider.UploadFile(ctx, storageProvider.PublicBucket(), filePathName, extension, contents)
	return storageProvider.PublicBucket(), result, err
}

// FileDownload - Abstract way to download a file from any implemented storage provider
func FileDownload(ctx context.Context, storageProvider storage.Provider, encryptionPhrase string, file *models.File) ([]byte, error) {

	storageBucket := storageProvider.PrivateBucket()
	if file.Public {
		storageBucket = storageProvider.PublicBucket()
	}

	filePathName := filepath.Join(file.AccessID, file.Hash)

	contents, err := storageProvider.DownloadFile(ctx, storageBucket, filePathName, file.Ext)
	if err != nil {
		return nil, err
	}

	if file.Public {
		return contents, nil
	}

	return utils.Decrypt(contents, encryptionPhrase)
}
