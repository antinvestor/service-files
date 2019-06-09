package service

import (
	"path/filepath"
	"github.com/opentracing/opentracing-go"
	"fmt"
)

// FileUpload - Abstract way to upload a file to any implemented storage provider
func FileUpload(env *Env, span opentracing.Span, isPublic bool, subscriptionID string, hash string, extension string, contents []byte) (bucket string, result string, err error) {

	traceName := fmt.Sprintf("File Upload to %s", env.StrorageProvider.Name())
	childSpan := opentracing.GlobalTracer().StartSpan(traceName, opentracing.ChildOf(span.Context()))
	defer childSpan.Finish()

	filePathName := filepath.Join(subscriptionID, hash)

	if !isPublic {
		hashedContent, err := Encrypt(contents, env.EncryptionPhrase)
		if err != nil {
			return env.StrorageProvider.PrivateBucket(), "", err
		}

		result, err := env.StrorageProvider.UploadFile(env.StrorageProvider.PrivateBucket(), filePathName, extension, hashedContent)
		return env.StrorageProvider.PrivateBucket(), result, err
	}

	result, err = env.StrorageProvider.UploadFile(env.StrorageProvider.PublicBucket(), filePathName, extension, contents)
	return env.StrorageProvider.PublicBucket(), result, err
}

// FileDownload - Abstract way to download a file from any implemented storage provider
func FileDownload(env *Env, span opentracing.Span, file File) ([]byte, error) {

	traceName := fmt.Sprintf("File Download to %s", env.StrorageProvider.Name())
	childSpan := opentracing.GlobalTracer().StartSpan(traceName, opentracing.ChildOf(span.Context()))
	defer childSpan.Finish()

	storageBucket := env.StrorageProvider.PrivateBucket()
	if file.Public {
		storageBucket = env.StrorageProvider.PublicBucket()
	}

	filePathName := filepath.Join(file.SubscriptionID, file.Hash)

	contents, err := env.StrorageProvider.DownloadFile(storageBucket, filePathName, file.Ext)
	if err != nil {
		return nil, err
	}

	if file.Public {
		return contents, nil
	}

	return Decrypt(contents, env.EncryptionPhrase)
}
