package repository

import (
	"context"
	"errors"

	"github.com/antinvestor/service-files/apps/property/service/models"
	"github.com/pitabwire/frame/datastore/pool"
	"gorm.io/gorm"
)

type PropertyRepository interface {
	GetByID(ctx context.Context, id string) (*models.Property, error)
	Search(ctx context.Context, query string) ([]models.Property, error)
	Save(ctx context.Context, property *models.Property) error
	Delete(ctx context.Context, id string) error
}

type propertyRepository struct {
	dbPool pool.Pool
}

func NewPropertyRepository(dbPool pool.Pool) PropertyRepository {
	return &propertyRepository{dbPool: dbPool}
}

func (repo *propertyRepository) GetByID(ctx context.Context, id string) (*models.Property, error) {
	property := models.Property{}
	err := repo.dbPool.DB(ctx, true).First(&property, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &property, nil
}

func (repo *propertyRepository) Search(ctx context.Context, query string) ([]models.Property, error) {
	var properties []models.Property

	err := repo.dbPool.DB(ctx, true).Find(&properties,
		" id ILIKE ? OR name ILIKE ? OR description ILIKE ?",
		query, query, query).Error
	if err != nil {
		return nil, err
	}
	return properties, nil
}

func (repo *propertyRepository) Delete(ctx context.Context, id string) error {
	property, err := repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	return repo.dbPool.DB(ctx, false).Delete(property).Error
}

func (repo *propertyRepository) Save(ctx context.Context, property *models.Property) error {
	err := repo.dbPool.DB(ctx, false).Save(property).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repo.dbPool.DB(ctx, false).Create(property).Error
	}
	return err
}
