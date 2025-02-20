package repository

import (
	"context"
	"github.com/antinvestor/service-files/service/storage/models"
	"github.com/antinvestor/service-files/service/types"
	"github.com/pitabwire/frame"
	"strconv"
)

type MediaRepository interface {
	GetByID(ctx context.Context, id types.MediaID) (*models.MediaMetadata, error)
	GetByHash(ctx context.Context, ownerId types.OwnerID, hash types.Base64Hash) (*models.MediaMetadata, error)
	GetByParentID(ctx context.Context, parentId types.MediaID) ([]*models.MediaMetadata, error)
	GetByParentIDAndThumbnailSize(ctx context.Context, parentId types.MediaID, thumbnailSize *types.ThumbnailSize) (*models.MediaMetadata, error)
	GetByOwnerID(ctx context.Context, ownerId types.OwnerID, query string, page int32, limit int32) ([]*models.MediaMetadata, error)
	Save(ctx context.Context, file *models.MediaMetadata) error
	Delete(ctx context.Context, id types.MediaID) error
}

func NewMediaRepository(service *frame.Service) MediaRepository {
	fileRepo := mediaRepository{
		service: service,
	}
	return &fileRepo
}

type mediaRepository struct {
	service *frame.Service
}

func (mr *mediaRepository) GetByID(ctx context.Context, id types.MediaID) (*models.MediaMetadata, error) {
	file := &models.MediaMetadata{}
	err := mr.service.DB(ctx, true).First(file, " id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (mr *mediaRepository) GetByHash(ctx context.Context, ownerId types.OwnerID, hash types.Base64Hash) (*models.MediaMetadata, error) {
	file := &models.MediaMetadata{}
	err := mr.service.DB(ctx, true).First(file, "owner_id = ? AND hash = ?", string(ownerId), string(hash)).Error
	if err != nil {
		return nil, err
	}

	return file, nil
}

func (mr *mediaRepository) GetByParentID(ctx context.Context, parentId types.MediaID) ([]*models.MediaMetadata, error) {
	var media []*models.MediaMetadata
	err := mr.service.DB(ctx, true).Where(" parent_id @@@ ?", string(parentId)).Find(&media).Error
	if err != nil {
		return nil, err
	}

	return media, nil
}

func (mr *mediaRepository) GetByParentIDAndThumbnailSize(ctx context.Context, parentId types.MediaID, thumbnailSize *types.ThumbnailSize) (*models.MediaMetadata, error) {
	media := &models.MediaMetadata{}
	tx := mr.service.DB(ctx, true).Where(" parent_id @@@ ? ", string(parentId))
	if thumbnailSize != nil {
		tx = tx.Where("id  @@@ paradedb.match( 'properties.h', ?) "+
			"AND id  @@@ paradedb.match( 'properties.w', ?) "+
			"AND id  @@@ paradedb.match( 'properties.m', ?)",
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
	tx := mr.service.DB(ctx, true).Where(" owner_id = ? ", string(ownerId))

	if query != "" {
		tx = tx.Where("name = ?", query)
	}

	tx = tx.Offset(int(page))
	tx = tx.Limit(int(limit))

	err := tx.Find(&fileList).Error
	if err != nil {
		return nil, err
	}

	return fileList, nil
}

func (mr *mediaRepository) Save(ctx context.Context, file *models.MediaMetadata) error {
	return mr.service.DB(ctx, false).Save(file).Error
}

func (mr *mediaRepository) Delete(ctx context.Context, id types.MediaID) error {

	file, err := mr.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return mr.service.DB(ctx, false).Delete(file).Error
}
