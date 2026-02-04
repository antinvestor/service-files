package business

import (
	"context"
	"log/slog"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	propertyv1 "buf.build/gen/go/antinvestor/property/protocolbuffers/go/property/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-files/apps/property/service/models"
	"github.com/antinvestor/service-files/apps/property/service/repository"
	"github.com/pitabwire/frame/datastore/pool"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type propertyBusiness struct {
	dbPool pool.Pool
}

func (pb *propertyBusiness) ToApi(ctx context.Context, property *models.Property) (*propertyv1.Property, error) {

	apiProperty := propertyv1.Property{
		Id:          property.GetID(),
		ParentId:    property.ParentID,
		Name:        property.Name,
		Description: property.Description,
		Extra:       models.JsonMapToStruct(property.Extra),
		StartedAt:   timestamppb.New(property.StartedAt),
	}

	if property.LocalityID != "" {
		localityRepo := repository.NewLocalityRepository(pb.dbPool)
		locality, err := localityRepo.GetByID(ctx, property.LocalityID)
		if err != nil {
			return nil, err
		}
		apiProperty.Locality = locality.ToApi()
	}

	if property.PropertyTypeID != "" {
		pTypeRepo := repository.NewPropertyTypeRepository(pb.dbPool)
		propertyType, err := pTypeRepo.GetByID(ctx, property.PropertyTypeID)
		if err != nil {
			return nil, err
		}
		apiProperty.PropertyType = propertyType.ToApi()
	}

	return &apiProperty, nil
}

func (pb *propertyBusiness) CreateProperty(ctx context.Context, message *propertyv1.Property) (*propertyv1.Property, error) {

	propertyRepo := repository.NewPropertyRepository(pb.dbPool)

	property := models.Property{
		Name:           message.GetName(),
		ParentID:       message.GetParentId(),
		PropertyTypeID: message.GetPropertyType().GetId(),
		Description:    message.GetDescription(),
		Extra:          models.StructToJSONMap(message.GetExtra()),
		StartedAt:      message.GetStartedAt().AsTime(),
	}

	if property.ValidXID(message.GetId()) {
		property.ID = message.GetId()
	} else {
		property.GenID(ctx)
	}

	err := propertyRepo.Save(ctx, &property)
	if err != nil {
		return nil, err
	}

	propertyStateRepo := repository.NewPropertyStateRepository(pb.dbPool)

	propertyState := models.PropertyState{
		PropertyID: property.GetID(),
		State:      int32(commonv1.STATE_CREATED.Number()),
		Status:     int32(commonv1.STATUS_QUEUED.Number()),
		Name:       commonv1.STATE_CREATED.String(),
	}

	propertyState.GenID(ctx)

	err = propertyStateRepo.Save(ctx, &propertyState)
	if err != nil {
		return nil, err
	}

	return pb.ToApi(ctx, &property)
}

func (pb *propertyBusiness) UpdateProperty(ctx context.Context, message *propertyv1.UpdatePropertyRequest) (*propertyv1.Property, error) {

	propertyRepo := repository.NewPropertyRepository(pb.dbPool)
	property, err := propertyRepo.GetByID(ctx, message.GetId())
	if err != nil {
		return nil, err
	}

	if message.GetName() != "" {
		property.Name = message.GetName()
	}

	if message.GetDescription() != "" {
		property.Description = message.GetDescription()
	}

	if message.GetExtras() != nil {
		extras := models.JsonMapToStruct(property.Extra)
		if extras == nil {
			extras = message.GetExtras()
		} else {
			for key, val := range message.GetExtras().GetFields() {
				extras.Fields[key] = val
			}
		}
		property.Extra = models.StructToJSONMap(extras)
	}

	err = propertyRepo.Save(ctx, property)
	if err != nil {
		return nil, err
	}

	return pb.ToApi(ctx, property)
}

func (pb *propertyBusiness) DeleteProperty(ctx context.Context, id string) error {

	propertyRepo := repository.NewPropertyRepository(pb.dbPool)
	property, err := propertyRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	err = propertyRepo.Delete(ctx, property.GetID())
	if err != nil {
		return err
	}

	propertyStateRepo := repository.NewPropertyStateRepository(pb.dbPool)

	propertyState := models.PropertyState{
		PropertyID: property.GetID(),
		State:      int32(commonv1.STATE_DELETED.Number()),
		Status:     int32(commonv1.STATUS_QUEUED.Number()),
		Name:       commonv1.STATE_DELETED.String(),
	}

	propertyState.GenID(ctx)

	err = propertyStateRepo.Save(ctx, &propertyState)
	if err != nil {
		return err
	}

	property.StateID = propertyState.GetID()
	return propertyRepo.Save(ctx, property)
}

func (pb *propertyBusiness) StateOfProperty(ctx context.Context, id string) (*propertyv1.PropertyState, error) {

	propertyRepo := repository.NewPropertyRepository(pb.dbPool)
	property, err := propertyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	propertyStateRepo := repository.NewPropertyStateRepository(pb.dbPool)

	propertyState, err := propertyStateRepo.GetByPropertyID(ctx, property.GetID())
	if err != nil {
		return nil, err
	}

	return propertyState.ToApi(), nil
}

func (pb *propertyBusiness) HistoryOfProperty(ctx context.Context, id string, stream *connect.ServerStream[propertyv1.HistoryOfPropertyResponse]) error {

	propertyRepo := repository.NewPropertyRepository(pb.dbPool)
	property, err := propertyRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	propertyStateRepo := repository.NewPropertyStateRepository(pb.dbPool)

	propertyStateList, err := propertyStateRepo.GetAllByPropertyID(ctx, property.GetID())
	if err != nil {
		return err
	}

	responses := make([]*propertyv1.PropertyState, 0, len(propertyStateList))
	for _, propertyState := range propertyStateList {
		responses = append(responses, propertyState.ToApi())
	}

	if len(responses) > 0 {
		err = stream.Send(&propertyv1.HistoryOfPropertyResponse{
			Data: responses,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (pb *propertyBusiness) SearchProperty(ctx context.Context, request *propertyv1.SearchPropertyRequest, stream *connect.ServerStream[propertyv1.SearchPropertyResponse]) error {

	propertyRepo := repository.NewPropertyRepository(pb.dbPool)

	propertyList, err := propertyRepo.Search(ctx, request.GetQuery())
	if err != nil {
		return err
	}

	responses := make([]*propertyv1.Property, 0, len(propertyList))
	for _, property := range propertyList {
		apiProperty, convErr := pb.ToApi(ctx, &property)
		if convErr != nil {
			slog.Info("SearchProperty -- unable to convert a result", "error", convErr)
			continue
		}
		responses = append(responses, apiProperty)
	}

	if len(responses) > 0 {
		err = stream.Send(&propertyv1.SearchPropertyResponse{
			Data: responses,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
