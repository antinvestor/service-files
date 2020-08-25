package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"time"

	"cloud.google.com/go/storage"
)

type ProviderGCS struct {
	name          string
	privateBucket string
	publicBucket  string

	projectID string
}

func (provider *ProviderGCS) Name() string {
	return provider.name
}

func (provider *ProviderGCS) PublicBucket() string {
	return provider.publicBucket
}

func (provider *ProviderGCS) PrivateBucket() string {
	return provider.privateBucket
}

func (provider *ProviderGCS) Init(ctx context.Context) (interface{}, error) {

	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (provider *ProviderGCS) UploadFile(ctx context.Context, bucket string, pathName string, extension string, contents []byte) (string, error) {

	clientObj, err := provider.Init(ctx)
	if err != nil {
		return "", err
	}

	client, ok := clientObj.(*storage.Client)
	if !ok {
		return "", errors.New("could not cast client object to GCS Object")
	}

	defer client.Close()

	wc := client.Bucket(bucket).Object(pathName).NewWriter(ctx)
	defer wc.Close()

	if _, err = io.Copy(wc, bytes.NewReader(contents)); err != nil {
		return "", err
	}

	return "", nil

}

func (provider *ProviderGCS) DownloadFile(ctx context.Context, bucket string, pathName string, extension string) ([]byte, error) {

	clientObj, err := provider.Init(ctx)
	if err != nil {
		return nil, err
	}

	client, ok := clientObj.(*storage.Client)
	if !ok {
		return nil, errors.New("could not cast client object to GCS Object")
	}

	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	rc, err := client.Bucket(bucket).Object(pathName).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := ioutil.ReadAll(rc)

	return data, nil
}
