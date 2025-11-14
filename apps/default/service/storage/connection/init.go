package connection

import (
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/pitabwire/frame/workerpool"
)

// NewMediaDatabase opens a database connection.
func NewMediaDatabase(workManager workerpool.Manager, mediaRepo repository.MediaRepository) (storage.Database, error) {

	return &Database{WorkManager: workManager, MediaRepository: mediaRepo}, nil
}
