package storage

import (
	"os"
	"path/filepath"
	"io/ioutil"
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


func (provider *ProviderLocal)Init() error   {
	return nil
}


func (provider *ProviderLocal)UploadFile(bucket string, pathName string,  extension string, contents []byte) (string,error)   {

	fullPathName := filepath.Join(bucket, pathName, extension)

	//Ensure parent directories exist
	os.MkdirAll(filepath.Dir(fullPathName), 0755)

	return "", ioutil.WriteFile(fullPathName, contents, 0644)
}

func (provider *ProviderLocal)DownloadFile(bucket string, pathName string,  extension string) ([]byte, error)   {
	fullPathName := filepath.Join(bucket, pathName, extension)
	return ioutil.ReadFile(fullPathName)
}

