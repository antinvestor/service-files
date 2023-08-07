package storage

import (
	"context"
	"github.com/antinvestor/files/config"
	"github.com/pitabwire/frame"
	"testing"
)

func TestGetStorageProvider(t *testing.T) {

	ctx := context.Background()

	var cfg config.FilesConfig
	err := frame.ConfigProcess("", &cfg)
	if err != nil {
		t.Errorf("Could not get file config : %v", err)
	}

	provider, err := GetStorageProvider(ctx, &cfg)
	if err != nil {
		t.Errorf("A file provider should has issues : %v", err)
	}

	_, ok := provider.(*ProviderLocal)
	if !ok {
		t.Errorf("The provider is supposed to be a local instance only")
	}

}
