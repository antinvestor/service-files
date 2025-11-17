package repository

import (
	"context"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

type MediaAuditRepository interface {
	datastore.BaseRepository[*models.MediaAudit]
}

func NewMediaAuditRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) MediaAuditRepository {
	fileAuditRepo := fileAuditRepository{
		BaseRepository: datastore.NewBaseRepository[*models.MediaAudit](
			ctx, dbPool, workMan, func() *models.MediaAudit { return &models.MediaAudit{} },
		),
	}
	return &fileAuditRepo
}

type fileAuditRepository struct {
	datastore.BaseRepository[*models.MediaAudit]
}
