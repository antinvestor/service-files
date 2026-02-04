package business

import (
	"context"

	propertyv1 "buf.build/gen/go/antinvestor/property/protocolbuffers/go/property/v1"
	"connectrpc.com/connect"
	"github.com/pitabwire/frame/datastore/pool"
)

type PropertyBusiness interface {
	CreateProperty(ctx context.Context, message *propertyv1.Property) (*propertyv1.Property, error)
	UpdateProperty(ctx context.Context, message *propertyv1.UpdatePropertyRequest) (*propertyv1.Property, error)
	DeleteProperty(ctx context.Context, id string) error
	StateOfProperty(ctx context.Context, id string) (*propertyv1.PropertyState, error)
	HistoryOfProperty(ctx context.Context, id string, stream *connect.ServerStream[propertyv1.HistoryOfPropertyResponse]) error
	SearchProperty(ctx context.Context, request *propertyv1.SearchPropertyRequest, stream *connect.ServerStream[propertyv1.SearchPropertyResponse]) error
}

func NewPropertyBusiness(dbPool pool.Pool) (PropertyBusiness, error) {
	if dbPool == nil {
		return nil, ErrorInitializationFail
	}

	return &propertyBusiness{
		dbPool: dbPool,
	}, nil
}

type PropertyTypeBusiness interface {
	AddPropertyType(ctx context.Context, message *propertyv1.PropertyType) (*propertyv1.PropertyType, error)
	ListPropertyType(ctx context.Context, request *propertyv1.ListPropertyTypeRequest, stream *connect.ServerStream[propertyv1.ListPropertyTypeResponse]) error
}

func NewPropertyTypeBusiness(dbPool pool.Pool) (PropertyTypeBusiness, error) {
	if dbPool == nil {
		return nil, ErrorInitializationFail
	}

	return &propertyTypeBusiness{
		dbPool: dbPool,
	}, nil
}

type LocalityBusiness interface {
	AddLocality(ctx context.Context, message *propertyv1.Locality) (*propertyv1.Locality, error)
	DeleteLocality(ctx context.Context, id string) error
}

func NewLocalityBusiness(dbPool pool.Pool) (LocalityBusiness, error) {
	if dbPool == nil {
		return nil, ErrorInitializationFail
	}

	return &localityBusiness{
		dbPool: dbPool,
	}, nil
}

type SubscriptionBusiness interface {
	ListSubscription(ctx context.Context, request *propertyv1.ListSubscriptionRequest, stream *connect.ServerStream[propertyv1.ListSubscriptionResponse]) error
	AddSubscription(ctx context.Context, message *propertyv1.Subscription) (*propertyv1.Subscription, error)
	DeleteSubscription(ctx context.Context, id string) error
}

func NewSubscriptionBusiness(dbPool pool.Pool) (SubscriptionBusiness, error) {
	if dbPool == nil {
		return nil, ErrorInitializationFail
	}

	return &subscriptionBusiness{
		dbPool: dbPool,
	}, nil
}
