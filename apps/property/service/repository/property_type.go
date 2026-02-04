package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/antinvestor/service-files/apps/property/service/models"
	"github.com/pitabwire/frame/datastore/pool"
	"gorm.io/gorm"
)

type PropertyTypeRepository interface {
	GetByID(ctx context.Context, id string) (*models.PropertyType, error)
	GetAllByQuery(ctx context.Context, query string) ([]models.PropertyType, error)
	Save(ctx context.Context, propertyType *models.PropertyType) error
}

type propertyTypeRepository struct {
	dbPool pool.Pool
}

func NewPropertyTypeRepository(dbPool pool.Pool) PropertyTypeRepository {
	return &propertyTypeRepository{dbPool: dbPool}
}

func (repo *propertyTypeRepository) GetByID(ctx context.Context, id string) (*models.PropertyType, error) {
	propertyType := models.PropertyType{}
	err := repo.dbPool.DB(ctx, true).First(&propertyType, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &propertyType, nil
}

func (repo *propertyTypeRepository) GetAllByQuery(ctx context.Context, query string) ([]models.PropertyType, error) {
	var propertyTypes []models.PropertyType

	if query == "" {
		err := repo.dbPool.DB(ctx, true).Find(&propertyTypes).Error
		if err != nil {
			return nil, err
		}
	} else {
		query = fmt.Sprintf("%%%s%%", query)
		err := repo.dbPool.DB(ctx, true).Find(&propertyTypes, "name iLike ?", query).Error
		if err != nil {
			return nil, err
		}
	}
	return propertyTypes, nil
}

func (repo *propertyTypeRepository) Save(ctx context.Context, propertyType *models.PropertyType) error {
	err := repo.dbPool.DB(ctx, false).Save(propertyType).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repo.dbPool.DB(ctx, false).Create(propertyType).Error
	}
	return err
}
