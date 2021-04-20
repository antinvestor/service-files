package storage

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
)

type ProviderLocal struct {
	name string
	privateBucket string
	publicBucket string
}


func (provider *ProviderLocal) Name()string   {
	return provider.name
}

func (provider *ProviderLocal) PublicBucket()string   {
	return provider.publicBucket
}

func (provider *ProviderLocal) PrivateBucket()string   {
	return provider.privateBucket
}


func (provider *ProviderLocal)Init(ctx context.Context) (interface{}, error)   {
	return nil, nil
}


func (provider *ProviderLocal)UploadFile(ctx context.Context, bucket string, pathName string,  extension string, contents []byte) (string,error)   {

	fullPathName := filepath.Join(bucket, pathName, extension)

	//Ensure parent directories exist
	err := os.MkdirAll(filepath.Dir(fullPathName), 0755)
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile(fullPathName, contents, 0644)
	return fullPathName, err
}

func (provider *ProviderLocal)DownloadFile(ctx context.Context, bucket string, pathName string,  extension string) ([]byte, error)   {
	fullPathName := filepath.Join(bucket, pathName, extension)
	return ioutil.ReadFile(fullPathName)
}

