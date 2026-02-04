package repository

import (
	"context"
	"errors"

	"github.com/antinvestor/service-files/apps/property/service/models"
	"github.com/pitabwire/frame/datastore/pool"
	"gorm.io/gorm"
)

type LocalityRepository interface {
	GetByID(ctx context.Context, id string) (*models.Locality, error)
	Save(ctx context.Context, locality *models.Locality) error
	Delete(ctx context.Context, id string) error
}

type localityRepository struct {
	dbPool pool.Pool
}

func NewLocalityRepository(dbPool pool.Pool) LocalityRepository {
	return &localityRepository{dbPool: dbPool}
}

func (repo *localityRepository) GetByID(ctx context.Context, id string) (*models.Locality, error) {
	locality := models.Locality{}
	err := repo.dbPool.DB(ctx, true).First(&locality, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &locality, nil
}

func (repo *localityRepository) Save(ctx context.Context, locality *models.Locality) error {
	err := repo.dbPool.DB(ctx, false).Save(locality).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repo.dbPool.DB(ctx, false).Create(locality).Error
	}
	return err
}

func (repo *localityRepository) Delete(ctx context.Context, id string) error {
	locality, err := repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	return repo.dbPool.DB(ctx, false).Delete(locality).Error
}
