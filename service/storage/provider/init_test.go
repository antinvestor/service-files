package provider

import (
	"context"
	"github.com/antinvestor/service-files/config"
	"github.com/antinvestor/service-files/service/storage/provider/local"
	"github.com/pitabwire/frame"
	"testing"
)

func TestGetStorageProvider(t *testing.T) {

	ctx := context.Background()

	cfg, err := frame.ConfigFromEnv[config.FilesConfig]()
	if err != nil {
		t.Errorf("Could not get file config : %v", err)
	}

	storageProvider, err := GetStorageProvider(ctx, &cfg)
	if err != nil {
		t.Errorf("A file storageProvider should has issues : %v", err)
	}

	_, ok := storageProvider.(*local.ProviderLocal)
	if !ok {
		t.Errorf("The storageProvider is supposed to be a local instance only")
	}

}
