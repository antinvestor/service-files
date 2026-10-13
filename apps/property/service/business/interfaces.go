package business

import (
	"context"

	propertyv1connect "buf.build/gen/go/antinvestor/property/connectrpc/go/v2/v1/propertyv1connect"
	propertyv1 "buf.build/gen/go/antinvestor/property/protocolbuffers/go/v1"
	"github.com/pitabwire/frame/v3/datastore/pool"
)

type PropertyBusiness interface {
	CreateProperty(ctx context.Context, message *propertyv1.Property) (*propertyv1.Property, error)
	UpdateProperty(ctx context.Context, message *propertyv1.UpdatePropertyRequest) (*propertyv1.Property, error)
	DeleteProperty(ctx context.Context, id string) error
	StateOfProperty(ctx context.Context, id string) (*propertyv1.PropertyState, error)
	HistoryOfProperty(ctx context.Context, id string, stream propertyv1connect.PropertyServiceHistoryOfPropertyServerStream) error
	SearchProperty(ctx context.Context, request *propertyv1.SearchPropertyRequest, stream propertyv1connect.PropertyServiceSearchPropertyServerStream) error
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
	ListPropertyType(ctx context.Context, request *propertyv1.ListPropertyTypeRequest, stream propertyv1connect.PropertyServiceListPropertyTypeServerStream) error
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
	ListSubscription(ctx context.Context, request *propertyv1.ListSubscriptionRequest, stream propertyv1connect.PropertyServiceListSubscriptionServerStream) error
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
