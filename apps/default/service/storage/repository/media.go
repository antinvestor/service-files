package repository

import (
	"context"
	"strconv"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/pitabwire/frame/workerpool"
)

type MediaRepository interface {
	datastore.BaseRepository[*models.MediaMetadata]
	GetByHash(ctx context.Context, ownerId types.OwnerID, hash types.Base64Hash) (*models.MediaMetadata, error)
	GetByParentID(ctx context.Context, parentId types.MediaID) ([]*models.MediaMetadata, error)
	GetByParentIDAndThumbnailSize(ctx context.Context, parentId types.MediaID, thumbnailSize *types.ThumbnailSize) (*models.MediaMetadata, error)
	GetByOwnerID(ctx context.Context, ownerId types.OwnerID, query string, page int32, limit int32) ([]*models.MediaMetadata, error)
}

func NewMediaRepository(ctx context.Context, dbPool pool.Pool, workMan workerpool.Manager) MediaRepository {
	fileRepo := mediaRepository{
		BaseRepository: datastore.NewBaseRepository[*models.MediaMetadata](
			ctx, dbPool, workMan, func() *models.MediaMetadata { return &models.MediaMetadata{} },
		),
	}
	return &fileRepo
}

type mediaRepository struct {
	datastore.BaseRepository[*models.MediaMetadata]
}

func (mr *mediaRepository) GetByHash(ctx context.Context, ownerId types.OwnerID, hash types.Base64Hash) (*models.MediaMetadata, error) {
	file := &models.MediaMetadata{}
	err := mr.Pool().DB(ctx, true).First(file, "owner_id = ? AND hash = ?", string(ownerId), string(hash)).Error
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (mr *mediaRepository) GetByParentID(ctx context.Context, parentId types.MediaID) ([]*models.MediaMetadata, error) {
	var media []*models.MediaMetadata
	err := mr.Pool().DB(ctx, true).Where("parent_id = ?", string(parentId)).Find(&media).Error
	if err != nil {
		return nil, err
	}

	return media, nil
}

func (mr *mediaRepository) GetByParentIDAndThumbnailSize(ctx context.Context, parentId types.MediaID, thumbnailSize *types.ThumbnailSize) (*models.MediaMetadata, error) {
	media := &models.MediaMetadata{}
	tx := mr.Pool().DB(ctx, true).Where(" parent_id = ? ", string(parentId))
	if thumbnailSize != nil {
		tx = tx.Where("properties ->> 'h' = ?  "+
			"AND properties ->> 'w' = ?  "+
			"AND properties ->> 'm' = ? ",
			strconv.Itoa(thumbnailSize.Height), strconv.Itoa(thumbnailSize.Width), thumbnailSize.ResizeMethod)
	}

	err := tx.First(media).Error
	if err != nil {
		return nil, err
	}

	return media, nil
}

func (mr *mediaRepository) GetByOwnerID(ctx context.Context, ownerId types.OwnerID, query string, page int32, limit int32) ([]*models.MediaMetadata, error) {
	fileList := make([]*models.MediaMetadata, 0)
	tx := mr.Pool().DB(ctx, true).Where(" owner_id = ? ", string(ownerId))

	if query != "" {
		tx = tx.Where("name LIKE ?", "%"+query+"%")
	}

	// Use 0-based pagination: page 0 is the first page
	offset := page * limit
	tx = tx.Offset(int(offset))
	tx = tx.Limit(int(limit))

	err := tx.Find(&fileList).Error
	if err != nil {
		return nil, err
	}

	return fileList, nil
}
