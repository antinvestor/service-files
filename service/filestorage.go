package service

import (
	"context"
	"fmt"
	"github.com/opentracing/opentracing-go"
	"path/filepath"
)

// FileUpload - Abstract way to upload a file to any implemented storage provider
func FileUpload(ctx context.Context, spanContext opentracing.SpanContext, env *Env, isPublic bool, subscriptionID string, hash string, extension string, contents []byte) (bucket string, result string, err error) {

	traceName := fmt.Sprintf("File Upload to %s", env.StrorageProvider.Name())
	childSpan := opentracing.GlobalTracer().StartSpan(traceName, opentracing.ChildOf(spanContext))
	defer childSpan.Finish()

	filePathName := filepath.Join(subscriptionID, hash)

	if !isPublic {
		hashedContent, err := Encrypt(contents, env.EncryptionPhrase)
		if err != nil {
			return env.StrorageProvider.PrivateBucket(), "", err
		}

		result, err := env.StrorageProvider.UploadFile(ctx, env.StrorageProvider.PrivateBucket(), filePathName, extension, hashedContent)
		return env.StrorageProvider.PrivateBucket(), result, err
	}

	result, err = env.StrorageProvider.UploadFile(ctx, env.StrorageProvider.PublicBucket(), filePathName, extension, contents)
	return env.StrorageProvider.PublicBucket(), result, err
}

// FileDownload - Abstract way to download a file from any implemented storage provider
func FileDownload(ctx context.Context, spanContext opentracing.SpanContext, env *Env, file File) ([]byte, error) {

	traceName := fmt.Sprintf("File Download to %s", env.StrorageProvider.Name())
	childSpan := opentracing.GlobalTracer().StartSpan(traceName, opentracing.ChildOf(spanContext))
	defer childSpan.Finish()

	storageBucket := env.StrorageProvider.PrivateBucket()
	if file.Public {
		storageBucket = env.StrorageProvider.PublicBucket()
	}

	filePathName := filepath.Join(file.SubscriptionID, file.Hash)

	contents, err := env.StrorageProvider.DownloadFile(ctx, storageBucket, filePathName, file.Ext)
	if err != nil {
		return nil, err
	}

	if file.Public {
		return contents, nil
	}

	return Decrypt(contents, env.EncryptionPhrase)
}
