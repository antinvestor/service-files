package datastore

import (
	"github.com/antinvestor/service-files/service/storage"
	"github.com/antinvestor/service-files/service/storage/repository"
	"github.com/pitabwire/frame"
)

// NewMediaDatabase opens a database connection.
func NewMediaDatabase(srv *frame.Service) (storage.Database, error) {
	mediaRepo := repository.NewMediaRepository(srv)
	return &Database{MediaRepository: mediaRepo}, nil
}
