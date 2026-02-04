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

type subscriptionBusiness struct {
	dbPool pool.Pool
}

func (s *subscriptionBusiness) AddSubscription(ctx context.Context, message *propertyv1.Subscription) (*propertyv1.Subscription, error) {

	propertyRepo := repository.NewPropertyRepository(s.dbPool)
	subscriptionRepo := repository.NewSubscriptionRepository(s.dbPool)

	property, err := propertyRepo.GetByID(ctx, message.GetPropertyId())
	if err != nil {
		return nil, err
	}

	subscription := models.Subscription{
		PropertyID: property.GetID(),
		ProfileID:  message.GetProfileId(),
		Role:       message.GetRole(),
		Extra:      models.StructToJSONMap(message.GetExtra()),
		ExpiresAt:  message.GetExpiresAt().AsTime(),
	}

	if subscription.ValidXID(message.GetId()) {
		subscription.ID = message.GetId()
	} else {
		subscription.GenID(ctx)
	}

	err = subscriptionRepo.Save(ctx, &subscription)
	if err != nil {
		return nil, err
	}

	return subscription.ToApi(), nil
}

func (s *subscriptionBusiness) ListSubscription(ctx context.Context, request *propertyv1.ListSubscriptionRequest, stream *connect.ServerStream[propertyv1.ListSubscriptionResponse]) error {

	propertyRepo := repository.NewPropertyRepository(s.dbPool)
	subscriptionRepo := repository.NewSubscriptionRepository(s.dbPool)

	property, err := propertyRepo.GetByID(ctx, request.GetPropertyId())
	if err != nil {
		return err
	}

	subscriptionList, err := subscriptionRepo.GetByPropertyID(ctx, property.GetID(), request.GetQuery())
	if err != nil {
		return err
	}

	responses := make([]*propertyv1.Subscription, 0, len(subscriptionList))
	for _, subscription := range subscriptionList {
		responses = append(responses, subscription.ToApi())
	}

	if len(responses) > 0 {
		err = stream.Send(&propertyv1.ListSubscriptionResponse{
			Data: responses,
		})
		if err != nil {
			slog.Info("ListSubscription -- unable to send a result", "error", err)
		}
	}

	return nil
}

func (s *subscriptionBusiness) DeleteSubscription(ctx context.Context, id string) error {

	subscriptionRepo := repository.NewSubscriptionRepository(s.dbPool)

	subscription, err := subscriptionRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	return subscriptionRepo.Delete(ctx, subscription.GetID())
}
