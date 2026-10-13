package handlers

import (
	"context"

	propertyv1connect "buf.build/gen/go/antinvestor/property/connectrpc/go/v2/v1/propertyv1connect"
	propertyv1 "buf.build/gen/go/antinvestor/property/protocolbuffers/go/v1"
	"github.com/antinvestor/service-files/apps/property/service/business"
	"github.com/pitabwire/frame/v3/datastore/pool"
)

type PropertyServer struct {
	DBPool pool.Pool

	propertyv1connect.UnimplementedPropertyServiceHandler
}

func (server *PropertyServer) newPropertyBusiness() (business.PropertyBusiness, error) {
	return business.NewPropertyBusiness(server.DBPool)
}

func (server *PropertyServer) newPropertyTypeBusiness() (business.PropertyTypeBusiness, error) {
	return business.NewPropertyTypeBusiness(server.DBPool)
}

func (server *PropertyServer) newLocalityBusiness() (business.LocalityBusiness, error) {
	return business.NewLocalityBusiness(server.DBPool)
}

func (server *PropertyServer) newSubscriptionBusiness() (business.SubscriptionBusiness, error) {
	return business.NewSubscriptionBusiness(server.DBPool)
}

func (server *PropertyServer) AddPropertyType(ctx context.Context, request *propertyv1.AddPropertyTypeRequest) (*propertyv1.AddPropertyTypeResponse, error) {
	propertyTypeBusiness, err := server.newPropertyTypeBusiness()
	if err != nil {
		return nil, err
	}

	result, err := propertyTypeBusiness.AddPropertyType(ctx, request.GetData())
	if err != nil {
		return nil, err
	}

	return &propertyv1.AddPropertyTypeResponse{Data: result}, nil
}

func (server *PropertyServer) ListPropertyType(ctx context.Context, request *propertyv1.ListPropertyTypeRequest, stream propertyv1connect.PropertyServiceListPropertyTypeServerStream) error {
	propertyTypeBusiness, err := server.newPropertyTypeBusiness()
	if err != nil {
		return err
	}

	return propertyTypeBusiness.ListPropertyType(ctx, request, stream)
}

func (server *PropertyServer) AddLocality(ctx context.Context, request *propertyv1.AddLocalityRequest) (*propertyv1.AddLocalityResponse, error) {
	localityBusiness, err := server.newLocalityBusiness()
	if err != nil {
		return nil, err
	}

	result, err := localityBusiness.AddLocality(ctx, request.GetData())
	if err != nil {
		return nil, err
	}

	return &propertyv1.AddLocalityResponse{Data: result}, nil
}

func (server *PropertyServer) DeleteLocality(ctx context.Context, request *propertyv1.DeleteLocalityRequest) (*propertyv1.DeleteLocalityResponse, error) {
	localityBusiness, err := server.newLocalityBusiness()
	if err != nil {
		return nil, err
	}

	err = localityBusiness.DeleteLocality(ctx, request.GetId())
	if err != nil {
		return nil, err
	}

	return &propertyv1.DeleteLocalityResponse{Success: true}, nil
}

func (server *PropertyServer) CreateProperty(ctx context.Context, request *propertyv1.CreatePropertyRequest) (*propertyv1.CreatePropertyResponse, error) {
	propertyBusiness, err := server.newPropertyBusiness()
	if err != nil {
		return nil, err
	}

	result, err := propertyBusiness.CreateProperty(ctx, request.GetData())
	if err != nil {
		return nil, err
	}

	return &propertyv1.CreatePropertyResponse{Data: result}, nil
}

func (server *PropertyServer) UpdateProperty(ctx context.Context, request *propertyv1.UpdatePropertyRequest) (*propertyv1.UpdatePropertyResponse, error) {
	propertyBusiness, err := server.newPropertyBusiness()
	if err != nil {
		return nil, err
	}

	result, err := propertyBusiness.UpdateProperty(ctx, request)
	if err != nil {
		return nil, err
	}

	return &propertyv1.UpdatePropertyResponse{Data: result}, nil
}

func (server *PropertyServer) DeleteProperty(ctx context.Context, request *propertyv1.DeletePropertyRequest) (*propertyv1.DeletePropertyResponse, error) {
	propertyBusiness, err := server.newPropertyBusiness()
	if err != nil {
		return nil, err
	}

	err = propertyBusiness.DeleteProperty(ctx, request.GetId())
	if err != nil {
		return nil, err
	}

	return &propertyv1.DeletePropertyResponse{Success: true}, nil
}

func (server *PropertyServer) StateOfProperty(ctx context.Context, request *propertyv1.StateOfPropertyRequest) (*propertyv1.StateOfPropertyResponse, error) {
	propertyBusiness, err := server.newPropertyBusiness()
	if err != nil {
		return nil, err
	}

	result, err := propertyBusiness.StateOfProperty(ctx, request.GetId())
	if err != nil {
		return nil, err
	}

	return &propertyv1.StateOfPropertyResponse{Data: result}, nil
}

func (server *PropertyServer) HistoryOfProperty(ctx context.Context, request *propertyv1.HistoryOfPropertyRequest, stream propertyv1connect.PropertyServiceHistoryOfPropertyServerStream) error {
	propertyBusiness, err := server.newPropertyBusiness()
	if err != nil {
		return err
	}

	return propertyBusiness.HistoryOfProperty(ctx, request.GetId(), stream)
}

func (server *PropertyServer) SearchProperty(ctx context.Context, request *propertyv1.SearchPropertyRequest, stream propertyv1connect.PropertyServiceSearchPropertyServerStream) error {
	propertyBusiness, err := server.newPropertyBusiness()
	if err != nil {
		return err
	}

	return propertyBusiness.SearchProperty(ctx, request, stream)
}

func (server *PropertyServer) ListSubscription(ctx context.Context, request *propertyv1.ListSubscriptionRequest, stream propertyv1connect.PropertyServiceListSubscriptionServerStream) error {
	subscriptionBusiness, err := server.newSubscriptionBusiness()
	if err != nil {
		return err
	}

	return subscriptionBusiness.ListSubscription(ctx, request, stream)
}

func (server *PropertyServer) AddSubscription(ctx context.Context, request *propertyv1.AddSubscriptionRequest) (*propertyv1.AddSubscriptionResponse, error) {
	subscriptionBusiness, err := server.newSubscriptionBusiness()
	if err != nil {
		return nil, err
	}

	result, err := subscriptionBusiness.AddSubscription(ctx, request.GetData())
	if err != nil {
		return nil, err
	}

	return &propertyv1.AddSubscriptionResponse{Data: result}, nil
}

func (server *PropertyServer) DeleteSubscription(ctx context.Context, request *propertyv1.DeleteSubscriptionRequest) (*propertyv1.DeleteSubscriptionResponse, error) {
	subscriptionBusiness, err := server.newSubscriptionBusiness()
	if err != nil {
		return nil, err
	}

	err = subscriptionBusiness.DeleteSubscription(ctx, request.GetId())
	if err != nil {
		return nil, err
	}

	return &propertyv1.DeleteSubscriptionResponse{Success: true}, nil
}
