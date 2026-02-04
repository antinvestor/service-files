package handlers

import (
	"context"

	propertyv1connect "buf.build/gen/go/antinvestor/property/connectrpc/go/property/v1/propertyv1connect"
	propertyv1 "buf.build/gen/go/antinvestor/property/protocolbuffers/go/property/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-files/apps/property/service/business"
	"github.com/pitabwire/frame/datastore/pool"
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

func (server *PropertyServer) AddPropertyType(ctx context.Context, request *connect.Request[propertyv1.AddPropertyTypeRequest]) (*connect.Response[propertyv1.AddPropertyTypeResponse], error) {
	propertyTypeBusiness, err := server.newPropertyTypeBusiness()
	if err != nil {
		return nil, err
	}

	result, err := propertyTypeBusiness.AddPropertyType(ctx, request.Msg.GetData())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&propertyv1.AddPropertyTypeResponse{Data: result}), nil
}

func (server *PropertyServer) ListPropertyType(ctx context.Context, request *connect.Request[propertyv1.ListPropertyTypeRequest], stream *connect.ServerStream[propertyv1.ListPropertyTypeResponse]) error {
	propertyTypeBusiness, err := server.newPropertyTypeBusiness()
	if err != nil {
		return err
	}

	return propertyTypeBusiness.ListPropertyType(ctx, request.Msg, stream)
}

func (server *PropertyServer) AddLocality(ctx context.Context, request *connect.Request[propertyv1.AddLocalityRequest]) (*connect.Response[propertyv1.AddLocalityResponse], error) {
	localityBusiness, err := server.newLocalityBusiness()
	if err != nil {
		return nil, err
	}

	result, err := localityBusiness.AddLocality(ctx, request.Msg.GetData())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&propertyv1.AddLocalityResponse{Data: result}), nil
}

func (server *PropertyServer) DeleteLocality(ctx context.Context, request *connect.Request[propertyv1.DeleteLocalityRequest]) (*connect.Response[propertyv1.DeleteLocalityResponse], error) {
	localityBusiness, err := server.newLocalityBusiness()
	if err != nil {
		return nil, err
	}

	err = localityBusiness.DeleteLocality(ctx, request.Msg.GetId())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&propertyv1.DeleteLocalityResponse{Success: true}), nil
}

func (server *PropertyServer) CreateProperty(ctx context.Context, request *connect.Request[propertyv1.CreatePropertyRequest]) (*connect.Response[propertyv1.CreatePropertyResponse], error) {
	propertyBusiness, err := server.newPropertyBusiness()
	if err != nil {
		return nil, err
	}

	result, err := propertyBusiness.CreateProperty(ctx, request.Msg.GetData())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&propertyv1.CreatePropertyResponse{Data: result}), nil
}

func (server *PropertyServer) UpdateProperty(ctx context.Context, request *connect.Request[propertyv1.UpdatePropertyRequest]) (*connect.Response[propertyv1.UpdatePropertyResponse], error) {
	propertyBusiness, err := server.newPropertyBusiness()
	if err != nil {
		return nil, err
	}

	result, err := propertyBusiness.UpdateProperty(ctx, request.Msg)
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&propertyv1.UpdatePropertyResponse{Data: result}), nil
}

func (server *PropertyServer) DeleteProperty(ctx context.Context, request *connect.Request[propertyv1.DeletePropertyRequest]) (*connect.Response[propertyv1.DeletePropertyResponse], error) {
	propertyBusiness, err := server.newPropertyBusiness()
	if err != nil {
		return nil, err
	}

	err = propertyBusiness.DeleteProperty(ctx, request.Msg.GetId())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&propertyv1.DeletePropertyResponse{Success: true}), nil
}

func (server *PropertyServer) StateOfProperty(ctx context.Context, request *connect.Request[propertyv1.StateOfPropertyRequest]) (*connect.Response[propertyv1.StateOfPropertyResponse], error) {
	propertyBusiness, err := server.newPropertyBusiness()
	if err != nil {
		return nil, err
	}

	result, err := propertyBusiness.StateOfProperty(ctx, request.Msg.GetId())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&propertyv1.StateOfPropertyResponse{Data: result}), nil
}

func (server *PropertyServer) HistoryOfProperty(ctx context.Context, request *connect.Request[propertyv1.HistoryOfPropertyRequest], stream *connect.ServerStream[propertyv1.HistoryOfPropertyResponse]) error {
	propertyBusiness, err := server.newPropertyBusiness()
	if err != nil {
		return err
	}

	return propertyBusiness.HistoryOfProperty(ctx, request.Msg.GetId(), stream)
}

func (server *PropertyServer) SearchProperty(ctx context.Context, request *connect.Request[propertyv1.SearchPropertyRequest], stream *connect.ServerStream[propertyv1.SearchPropertyResponse]) error {
	propertyBusiness, err := server.newPropertyBusiness()
	if err != nil {
		return err
	}

	return propertyBusiness.SearchProperty(ctx, request.Msg, stream)
}

func (server *PropertyServer) ListSubscription(ctx context.Context, request *connect.Request[propertyv1.ListSubscriptionRequest], stream *connect.ServerStream[propertyv1.ListSubscriptionResponse]) error {
	subscriptionBusiness, err := server.newSubscriptionBusiness()
	if err != nil {
		return err
	}

	return subscriptionBusiness.ListSubscription(ctx, request.Msg, stream)
}

func (server *PropertyServer) AddSubscription(ctx context.Context, request *connect.Request[propertyv1.AddSubscriptionRequest]) (*connect.Response[propertyv1.AddSubscriptionResponse], error) {
	subscriptionBusiness, err := server.newSubscriptionBusiness()
	if err != nil {
		return nil, err
	}

	result, err := subscriptionBusiness.AddSubscription(ctx, request.Msg.GetData())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&propertyv1.AddSubscriptionResponse{Data: result}), nil
}

func (server *PropertyServer) DeleteSubscription(ctx context.Context, request *connect.Request[propertyv1.DeleteSubscriptionRequest]) (*connect.Response[propertyv1.DeleteSubscriptionResponse], error) {
	subscriptionBusiness, err := server.newSubscriptionBusiness()
	if err != nil {
		return nil, err
	}

	err = subscriptionBusiness.DeleteSubscription(ctx, request.Msg.GetId())
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&propertyv1.DeleteSubscriptionResponse{Success: true}), nil
}
