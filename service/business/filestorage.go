package business

import (
	"context"
	"github.com/antinvestor/files/config"
	"github.com/antinvestor/files/service/business/storage"
	"github.com/antinvestor/files/service/models"
	"github.com/antinvestor/files/service/utils"
	"github.com/pitabwire/frame"
	"path/filepath"
)

// FileUpload - Abstract way to upload a file to any implemented storage provider
func FileUpload(ctx context.Context, isPublic bool, subscriptionID string, hash string, extension string, contents []byte) (bucket string, result string, err error) {

	storageProvider := ctx.Value(config.CtxStorageProviderKey).(storage.Provider)

	filePathName := filepath.Join(subscriptionID, hash)

	if !isPublic {
		encryptionPhrase := frame.GetEnv(config.EnvStorageEncryptionPhrase, "AES256Key-XihgT047PgfrbYZJB4Rf2K")
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
func FileDownload(ctx context.Context, file *models.File) ([]byte, error) {

	storageProvider := ctx.Value(config.CtxStorageProviderKey).(storage.Provider)

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

	encryptionPhrase := frame.GetEnv(config.EnvStorageEncryptionPhrase, "AES256Key-XihgT047PgfrbYZJB4Rf2K")
	return utils.Decrypt(contents, encryptionPhrase)
}
