package handler

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	redirectv1 "github.com/antinvestor/service-files/apps/redirect/gen/redirect/v1"
	"github.com/antinvestor/service-files/apps/redirect/gen/redirect/v1/redirectv1connect"
	"github.com/antinvestor/service-files/apps/redirect/service/business"
	"github.com/antinvestor/service-files/apps/redirect/service/models"
	"github.com/pitabwire/frame/datastore/pool"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
)

// RedirectServer implements the Connect RPC RedirectService handler.
type RedirectServer struct {
	DBPool     pool.Pool
	RedirectHd *RedirectHandler

	redirectv1connect.UnimplementedRedirectServiceHandler
}

func (s *RedirectServer) newLinkBusiness() (business.LinkBusiness, error) {
	return business.NewLinkBusiness(s.DBPool)
}

func (s *RedirectServer) newClickBusiness() (business.ClickBusiness, error) {
	return business.NewClickBusiness(s.DBPool)
}

func (s *RedirectServer) CreateLink(ctx context.Context, req *connect.Request[redirectv1.CreateLinkRequest]) (*connect.Response[redirectv1.CreateLinkResponse], error) {
	linkBiz, err := s.newLinkBusiness()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create link: %w", err))
	}

	msg := req.Msg.GetData()
	link := protoToLink(msg)

	result, err := linkBiz.CreateLink(ctx, link)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create link: %w", err))
	}

	return connect.NewResponse(&redirectv1.CreateLinkResponse{Data: linkToProto(result)}), nil
}

func (s *RedirectServer) GetLink(ctx context.Context, req *connect.Request[redirectv1.GetLinkRequest]) (*connect.Response[redirectv1.GetLinkResponse], error) {
	linkBiz, err := s.newLinkBusiness()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get link: %w", err))
	}

	result, err := linkBiz.GetLink(ctx, req.Msg.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("get link: %w", err))
	}

	return connect.NewResponse(&redirectv1.GetLinkResponse{Data: linkToProto(result)}), nil
}

func (s *RedirectServer) UpdateLink(ctx context.Context, req *connect.Request[redirectv1.UpdateLinkRequest]) (*connect.Response[redirectv1.UpdateLinkResponse], error) {
	linkBiz, err := s.newLinkBusiness()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update link: %w", err))
	}

	updates := make(map[string]any)
	msg := req.Msg
	if msg.GetDestinationUrl() != "" {
		updates["destination_url"] = msg.GetDestinationUrl()
	}
	if msg.GetTitle() != "" {
		updates["title"] = msg.GetTitle()
	}
	if msg.GetCampaign() != "" {
		updates["campaign"] = msg.GetCampaign()
	}
	if msg.GetSource() != "" {
		updates["source"] = msg.GetSource()
	}
	if msg.GetMedium() != "" {
		updates["medium"] = msg.GetMedium()
	}
	if msg.GetContent() != "" {
		updates["content"] = msg.GetContent()
	}
	if msg.GetTerm() != "" {
		updates["term"] = msg.GetTerm()
	}
	if msg.GetState() != redirectv1.LinkState_LINK_STATE_UNSPECIFIED {
		updates["state"] = models.LinkState(msg.GetState())
	}

	result, err := linkBiz.UpdateLink(ctx, msg.GetId(), updates)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("update link: %w", err))
	}

	s.RedirectHd.InvalidateCache(ctx, result.Slug)

	return connect.NewResponse(&redirectv1.UpdateLinkResponse{Data: linkToProto(result)}), nil
}

func (s *RedirectServer) DeleteLink(ctx context.Context, req *connect.Request[redirectv1.DeleteLinkRequest]) (*connect.Response[redirectv1.DeleteLinkResponse], error) {
	linkBiz, err := s.newLinkBusiness()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete link: %w", err))
	}

	link, _ := linkBiz.GetLink(ctx, req.Msg.GetId())

	if err := linkBiz.DeleteLink(ctx, req.Msg.GetId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete link: %w", err))
	}

	if link != nil {
		s.RedirectHd.InvalidateCache(ctx, link.Slug)
	}

	return connect.NewResponse(&redirectv1.DeleteLinkResponse{Success: true}), nil
}

func (s *RedirectServer) ListLinks(ctx context.Context, req *connect.Request[redirectv1.ListLinksRequest], stream *connect.ServerStream[redirectv1.ListLinksResponse]) error {
	linkBiz, err := s.newLinkBusiness()
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("list links: %w", err))
	}

	msg := req.Msg
	state := models.LinkState(msg.GetState())
	pageSize := int(msg.GetPageSize())

	links, err := linkBiz.ListLinks(ctx, msg.GetQuery(), msg.GetAffiliateId(), msg.GetCampaign(), state, pageSize, 0)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("list links: %w", err))
	}

	if len(links) > 0 {
		data := make([]*redirectv1.Link, 0, len(links))
		for i := range links {
			data = append(data, linkToProto(&links[i]))
		}
		if err := stream.Send(&redirectv1.ListLinksResponse{Data: data}); err != nil {
			return err
		}
	}

	return nil
}

func (s *RedirectServer) GetLinkStats(ctx context.Context, req *connect.Request[redirectv1.GetLinkStatsRequest]) (*connect.Response[redirectv1.GetLinkStatsResponse], error) {
	clickBiz, err := s.newClickBusiness()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get link stats: %w", err))
	}

	msg := req.Msg
	var startTime, endTime time.Time
	if msg.GetStartTime() != nil {
		startTime = msg.GetStartTime().AsTime()
	}
	if msg.GetEndTime() != nil {
		endTime = msg.GetEndTime().AsTime()
	}

	stats, err := clickBiz.GetStats(ctx, msg.GetLinkId(), startTime, endTime)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get link stats: %w", err))
	}

	return connect.NewResponse(&redirectv1.GetLinkStatsResponse{
		Data: &redirectv1.LinkStats{
			LinkId:           stats.LinkID,
			TotalClicks:      stats.TotalClicks,
			UniqueClicks:     stats.UniqueClicks,
			Referrers:        stats.Referrers,
			Countries:        stats.Countries,
			Devices:          stats.Devices,
			Browsers:         stats.Browsers,
			OperatingSystems: stats.OperatingSystems,
			ClicksPerDay:     stats.ClicksPerDay,
		},
	}), nil
}

func (s *RedirectServer) ListClicks(ctx context.Context, req *connect.Request[redirectv1.ListClicksRequest], stream *connect.ServerStream[redirectv1.ListClicksResponse]) error {
	clickBiz, err := s.newClickBusiness()
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("list clicks: %w", err))
	}

	msg := req.Msg
	var startTime, endTime time.Time
	if msg.GetStartTime() != nil {
		startTime = msg.GetStartTime().AsTime()
	}
	if msg.GetEndTime() != nil {
		endTime = msg.GetEndTime().AsTime()
	}

	clicks, err := clickBiz.ListClicks(ctx, msg.GetLinkId(), msg.GetAffiliateId(), startTime, endTime, int(msg.GetPageSize()), 0)
	if err != nil {
		return connect.NewError(connect.CodeInternal, fmt.Errorf("list clicks: %w", err))
	}

	if len(clicks) > 0 {
		data := make([]*redirectv1.Click, 0, len(clicks))
		for i := range clicks {
			data = append(data, clickToProto(&clicks[i]))
		}
		if err := stream.Send(&redirectv1.ListClicksResponse{Data: data}); err != nil {
			return err
		}
	}

	return nil
}

// --- Model ↔ Proto conversion ---

func linkToProto(l *models.Link) *redirectv1.Link {
	pb := &redirectv1.Link{
		Id:               l.GetID(),
		Slug:             l.Slug,
		DestinationUrl:   l.DestinationURL,
		Title:            l.Title,
		AffiliateId:      l.AffiliateID,
		Campaign:         l.Campaign,
		Source:           l.Source,
		Medium:           l.Medium,
		Content:          l.Content,
		Term:             l.Term,
		MaxClicks:        l.MaxClicks,
		State:            redirectv1.LinkState(l.State),
		ClickCount:       l.ClickCount,
		UniqueClickCount: l.UniqueClickCount,
		CreatedAt:        timestamppb.New(l.CreatedAt),
		ModifiedAt:       timestamppb.New(l.ModifiedAt),
	}
	if !l.ExpiresAt.IsZero() {
		pb.ExpiresAt = timestamppb.New(l.ExpiresAt)
	}
	if l.Tags != nil {
		pb.Tags, _ = structpb.NewStruct(map[string]any(l.Tags))
	}
	return pb
}

func protoToLink(pb *redirectv1.Link) *models.Link {
	link := &models.Link{
		Slug:           pb.GetSlug(),
		DestinationURL: pb.GetDestinationUrl(),
		Title:          pb.GetTitle(),
		AffiliateID:    pb.GetAffiliateId(),
		Campaign:       pb.GetCampaign(),
		Source:         pb.GetSource(),
		Medium:         pb.GetMedium(),
		Content:        pb.GetContent(),
		Term:           pb.GetTerm(),
		MaxClicks:      pb.GetMaxClicks(),
		State:          models.LinkState(pb.GetState()),
	}
	if pb.GetExpiresAt() != nil {
		link.ExpiresAt = pb.GetExpiresAt().AsTime()
	}
	if pb.GetTags() != nil {
		link.Tags = datatypes.JSONMap(pb.GetTags().AsMap())
	}
	return link
}

func clickToProto(c *models.Click) *redirectv1.Click {
	return &redirectv1.Click{
		Id:             c.GetID(),
		LinkId:         c.LinkID,
		AffiliateId:    c.AffiliateID,
		Slug:           c.Slug,
		IpAddress:      c.IPAddress,
		UserAgent:      c.UserAgent,
		Referer:        c.Referer,
		AcceptLanguage: c.AcceptLanguage,
		Country:        c.Country,
		City:           c.City,
		DeviceType:     redirectv1.DeviceType(c.DeviceType),
		Browser:        c.Browser,
		Os:             c.OS,
		CreatedAt:      timestamppb.New(c.CreatedAt),
	}
}
