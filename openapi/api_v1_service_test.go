package openapi_test

import (
	"context"
	"fmt"
	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/openapi"
	"github.com/antinvestor/service-files/service/business/storage_provider"
	"github.com/antinvestor/service-files/testsutil"
	"os"
	"testing"
)

func TestApiV1Service_AddFile(t *testing.T) {

	ctx, srv, cleanUpFunc, err := testsutil.GetTestService(context.TODO(), "AddFile")
	if err != nil {
		t.Errorf("Could not initialize service : %v", err)
		return
	}
	defer cleanUpFunc()

	cfg := srv.Config().(*config.FilesConfig)

	storageP, err := storage_provider.GetStorageProvider(ctx, cfg)
	if err != nil {
		t.Errorf("Could not get storage provider because : %v", err)
		return
	}

	path, _ := os.Getwd()
	fileName := fmt.Sprintf("%s/testing_data.txt", path)

	testFile, err := os.Create(fileName)
	if err != nil {
		t.Errorf("Could not create test file in : %v because : %v", path, err)
		return
	}

	_, err = testFile.WriteString("old\nfalcon\nsky\ncup\nforest\n")
	if err != nil {
		t.Errorf("Could not write to test file in : %v because : %v", fileName, err)
		return
	}

	apiService := openapi.NewApiV1Service(srv, storageP)
	response, err := apiService.AddFile(ctx, "access", "group", false, "testing.txt", testFile)
	if err != nil {
		t.Errorf("Could not add file because : %v", err)
		return
	}

	f, ok := response.Body.(openapi.File)
	if !ok {
		t.Errorf("response body is not instance of file")
	}

	if f.Name != "testing.txt" {
		t.Error("The file names don't match")
	}

}
