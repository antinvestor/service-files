package service

import (
	"context"
	"fmt"
	"github.com/antinvestor/files/config"
	"github.com/antinvestor/files/models"
	"github.com/antinvestor/files/service/storage"
	"github.com/opentracing/opentracing-go"
	"github.com/pitabwire/frame"
	"path/filepath"
)

// FileUpload - Abstract way to upload a file to any implemented storage provider
func FileUpload(ctx context.Context, spanContext opentracing.SpanContext, isPublic bool, subscriptionID string, hash string, extension string, contents []byte) (bucket string, result string, err error) {

	storageProvider := ctx.Value(config.CtxStorageProviderKey).(storage.Provider)

	traceName := fmt.Sprintf("File Upload to %s", storageProvider.Name())
	childSpan := opentracing.GlobalTracer().StartSpan(traceName, opentracing.ChildOf(spanContext))
	defer childSpan.Finish()

	filePathName := filepath.Join(subscriptionID, hash)

	if !isPublic {
		encryptionPhrase := frame.GetEnv("ENCRYPTION_PHRASE", "AES256Key-XihgT047PgfrbYZJB4Rf2K")
		hashedContent, err := Encrypt(contents, encryptionPhrase)
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
func FileDownload(ctx context.Context, spanContext opentracing.SpanContext, file models.File) ([]byte, error) {

	storageProvider := ctx.Value(config.CtxStorageProviderKey).(storage.Provider)

	traceName := fmt.Sprintf("File Download to %s", storageProvider.Name())
	childSpan := opentracing.GlobalTracer().StartSpan(traceName, opentracing.ChildOf(spanContext))
	defer childSpan.Finish()

	storageBucket := storageProvider.PrivateBucket()
	if file.Public {
		storageBucket = storageProvider.PublicBucket()
	}

	filePathName := filepath.Join(file.SubscriptionID, file.Hash)

	contents, err := storageProvider.DownloadFile(ctx, storageBucket, filePathName, file.Ext)
	if err != nil {
		return nil, err
	}

	if file.Public {
		return contents, nil
	}

	encryptionPhrase := frame.GetEnv("ENCRYPTION_PHRASE", "AES256Key-XihgT047PgfrbYZJB4Rf2K")
	return Decrypt(contents, encryptionPhrase)
}
