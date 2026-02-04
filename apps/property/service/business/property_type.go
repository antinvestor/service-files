package business

import (
	"context"
	"log/slog"

	propertyv1 "buf.build/gen/go/antinvestor/property/protocolbuffers/go/property/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-files/apps/property/service/models"
	"github.com/antinvestor/service-files/apps/property/service/repository"
	"github.com/pitabwire/frame/datastore/pool"
)

type propertyTypeBusiness struct {
	dbPool pool.Pool
}

func (pt *propertyTypeBusiness) AddPropertyType(ctx context.Context, message *propertyv1.PropertyType) (*propertyv1.PropertyType, error) {

	propertyTypeRepo := repository.NewPropertyTypeRepository(pt.dbPool)

	propertyType := models.PropertyType{
		Name:        message.GetName(),
		Description: message.GetDescription(),
		Extra:       models.StructToJSONMap(message.GetExtra()),
	}

	if propertyType.ValidXID(message.GetId()) {
		propertyType.ID = message.GetId()
	} else {
		propertyType.GenID(ctx)
	}

	err := propertyTypeRepo.Save(ctx, &propertyType)
	if err != nil {
		return nil, err
	}

	return propertyType.ToApi(), nil
}

func (pt *propertyTypeBusiness) ListPropertyType(ctx context.Context, request *propertyv1.ListPropertyTypeRequest, stream *connect.ServerStream[propertyv1.ListPropertyTypeResponse]) error {

	propertyTypeRepo := repository.NewPropertyTypeRepository(pt.dbPool)

	propertyTypeList, err := propertyTypeRepo.GetAllByQuery(ctx, request.GetQuery())
	if err != nil {
		return err
	}

	responses := make([]*propertyv1.PropertyType, 0, len(propertyTypeList))
	for _, propertyType := range propertyTypeList {
		responses = append(responses, propertyType.ToApi())
	}

	if len(responses) > 0 {
		err = stream.Send(&propertyv1.ListPropertyTypeResponse{
			Data: responses,
		})
		if err != nil {
			slog.Info("ListPropertyType -- unable to send a result", "error", err)
		}
	}

	return nil
}
