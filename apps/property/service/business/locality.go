package business

import (
	"context"
	"errors"

	propertyv1 "buf.build/gen/go/antinvestor/property/protocolbuffers/go/property/v1"
	"github.com/antinvestor/service-files/apps/property/service/models"
	"github.com/antinvestor/service-files/apps/property/service/repository"
	"github.com/pitabwire/frame/datastore/pool"
	"github.com/twpayne/go-geom"
	"github.com/twpayne/go-geom/encoding/geojson"
)

type localityBusiness struct {
	dbPool pool.Pool
}

func (l *localityBusiness) AddLocality(ctx context.Context, message *propertyv1.Locality) (*propertyv1.Locality, error) {

	locality := models.Locality{
		Name:        message.GetName(),
		Description: message.GetDescription(),
		ParentID:    message.GetParentId(),
		Extra:       models.StructToJSONMap(message.GetExtras()),
	}

	switch v := message.GetFeature().(type) {
	case *propertyv1.Locality_Boundary:
		var geoJsonFeature geom.T
		err := geojson.Unmarshal([]byte(v.Boundary), &geoJsonFeature)
		if err != nil {
			return nil, err
		}

		_, ok := geoJsonFeature.(*geom.Polygon)
		if !ok {
			return nil, errors.New("supplied geometry is not a polygon")
		}

		locality.Boundary = []byte(v.Boundary)
		locality.Point = []byte(`{}`)

	case *propertyv1.Locality_Point:
		var geoJsonFeature geom.T
		err := geojson.Unmarshal([]byte(v.Point), &geoJsonFeature)
		if err != nil {
			return nil, err
		}

		_, ok := geoJsonFeature.(*geom.Point)
		if !ok {
			return nil, errors.New("supplied geometry is not a point")
		}

		locality.Boundary = []byte(`{}`)
		locality.Point = []byte(v.Point)
	}

	if locality.ValidXID(message.GetId()) {
		locality.ID = message.GetId()
	} else {
		locality.GenID(ctx)
	}

	localityRepo := repository.NewLocalityRepository(l.dbPool)
	err := localityRepo.Save(ctx, &locality)
	if err != nil {
		return nil, err
	}

	return locality.ToApi(), nil
}

func (l *localityBusiness) DeleteLocality(ctx context.Context, id string) error {

	localityRepo := repository.NewLocalityRepository(l.dbPool)

	locality, err := localityRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	return localityRepo.Delete(ctx, locality.GetID())
}
