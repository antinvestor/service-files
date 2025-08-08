package repository

import (
	"context"
	"strconv"
	"time"

	"github.com/antinvestor/service-files/apps/default/service/storage/models"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/framedata"
)

type MediaRepository interface {
	GetByID(ctx context.Context, id types.MediaID) (*models.MediaMetadata, error)
	GetByHash(ctx context.Context, ownerId types.OwnerID, hash types.Base64Hash) (*models.MediaMetadata, error)
	GetByParentID(ctx context.Context, parentId types.MediaID) ([]*models.MediaMetadata, error)
	GetByParentIDAndThumbnailSize(ctx context.Context, parentId types.MediaID, thumbnailSize *types.ThumbnailSize) (*models.MediaMetadata, error)
	GetByOwnerID(ctx context.Context, ownerId types.OwnerID, query string, page int32, limit int32) ([]*models.MediaMetadata, error)
	Search(ctx context.Context, query *framedata.SearchQuery) (frame.JobResultPipe[[]*models.MediaMetadata], error)
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
	err := mr.service.DB(ctx, true).Where("parent_id = ?", string(parentId)).Find(&media).Error
	if err != nil {
		return nil, err
	}

	return media, nil
}

func (mr *mediaRepository) GetByParentIDAndThumbnailSize(ctx context.Context, parentId types.MediaID, thumbnailSize *types.ThumbnailSize) (*models.MediaMetadata, error) {
	media := &models.MediaMetadata{}
	tx := mr.service.DB(ctx, true).Where(" parent_id = ? ", string(parentId))
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
	tx := mr.service.DB(ctx, true).Where(" owner_id = ? ", string(ownerId))

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

func (mr *mediaRepository) Search(
	ctx context.Context,
	query *framedata.SearchQuery,
) (frame.JobResultPipe[[]*models.MediaMetadata], error) {
	return framedata.StableSearch[models.MediaMetadata](ctx, mr.service, query, func(
		ctx context.Context,
		query *framedata.SearchQuery,
	) ([]*models.MediaMetadata, error) {
		var metadataList []*models.MediaMetadata

		paginator := query.Pagination

		db := mr.service.DB(ctx, true).
			Limit(paginator.Limit).Offset(paginator.Offset)

		if query.Fields != nil {
			startAt, sok := query.Fields["start_date"]
			stopAt, stok := query.Fields["end_date"]
			if sok && startAt != nil && stok && stopAt != nil {
				startDate, ok1 := startAt.(*time.Time)
				endDate, ok2 := stopAt.(*time.Time)
				if ok1 && ok2 {
					db = db.Where(
						"created_at BETWEEN ? AND ? ",
						startDate.Format("2020-01-31T00:00:00Z"),
						endDate.Format("2020-01-31T00:00:00Z"),
					)
				}
			}

			parentID, pok := query.Fields["owner_id"]
			if pok {
				db = db.Where("owner_id = ?", parentID)
			}

			parentID, prok := query.Fields["parent_id"]
			if prok {
				db = db.Where("parent_id = ?", parentID)
			}
		}

		if query.Query != "" {
			searchTerm := "%" + query.Query + "%"

			db = db.Where(" name ILIKE ? OR search_properties @@ plainto_tsquery(?) ", searchTerm, query.Query)
		}

		err := db.Find(&metadataList).Error
		if err != nil {
			return nil, err
		}

		return metadataList, nil
	})
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
