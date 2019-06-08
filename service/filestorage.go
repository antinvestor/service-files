package service

import "path/filepath"

// FileUpload - Abstract way to upload a file to any implemented storage provider
func FileUpload(ctx *ContextV1, isPublic bool, subscriptionID string, hash string, extension string, contents []byte) (bucket string, result string, err error) {

	filePathName := filepath.Join(subscriptionID, hash)

	if !isPublic {
		hashedContent, err := Encrypt(contents, ctx.EncryptionPhrase)
		if err != nil {
			return ctx.StrorageProvider.PrivateBucket(), "", err
		}

		result, err := ctx.StrorageProvider.UploadFile(ctx.StrorageProvider.PrivateBucket(), filePathName, extension, hashedContent)
		return ctx.StrorageProvider.PrivateBucket(), result, err
	}

	result, err = ctx.StrorageProvider.UploadFile(ctx.StrorageProvider.PublicBucket(), filePathName, extension, contents)
	return ctx.StrorageProvider.PublicBucket(), result, err
}

// FileDownload - Abstract way to download a file from any implemented storage provider
func FileDownload(ctx *ContextV1, file File) ([]byte, error) {

	storageBucket := ctx.StrorageProvider.PrivateBucket()
	if file.Public {
		storageBucket = ctx.StrorageProvider.PublicBucket()
	}

	filePathName := filepath.Join(file.SubscriptionID, file.Hash)

	contents, err := ctx.StrorageProvider.DownloadFile(storageBucket, filePathName, file.Ext)
	if err != nil {
		return nil, err
	}

	if file.Public {
		return contents, nil
	}

	return Decrypt(contents, ctx.EncryptionPhrase)
}
