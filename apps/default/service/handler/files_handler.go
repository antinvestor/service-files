package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"buf.build/gen/go/antinvestor/files/connectrpc/go/files/v1/filesv1connect"
	filesv1 "buf.build/gen/go/antinvestor/files/protocolbuffers/go/files/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/service/utils"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/util"
	"google.golang.org/protobuf/types/known/structpb"
)

// FileServer implements the Connect RPC handler for files service
type FileServer struct {
	Service      *frame.Service
	mediaService business.MediaService
	db           storage.Database
	provider     storage.Provider

	filesv1connect.UnimplementedFilesServiceHandler
}

// NewFileServer creates a new FileServer instance
func NewFileServer(
	service *frame.Service,
	mediaService business.MediaService,
	db storage.Database,
	provider storage.Provider,
) filesv1connect.FilesServiceHandler {
	return &FileServer{
		Service:      service,
		mediaService: mediaService,
		db:           db,
		provider:     provider,
	}
}

// UploadContent handles file uploads via Connect RPC streaming
func (s *FileServer) UploadContent(ctx context.Context, stream *connect.ClientStream[filesv1.UploadContentRequest]) (*connect.Response[filesv1.UploadContentResponse], error) {
	authClaims := security.ClaimsFromContext(ctx)
	if authClaims == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	sub, err := authClaims.GetSubject()
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	cfg := s.Service.Config().(*config.FilesConfig)

	if !stream.Receive() {
		if err = stream.Err(); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("no metadata received"))
	}

	req := stream.Msg()
	metadata := req.GetMetadata()
	if metadata == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("metadata is required"))
	}

	var fileData []byte
	for stream.Receive() {
		req := stream.Msg()
		if chunk := req.GetChunk(); chunk != nil {
			fileData = append(fileData, chunk...)
		}
	}

	if err = stream.Err(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	businessReq := &business.UploadRequest{
		OwnerID:       types.OwnerID(sub),
		UploadName:    types.Filename(metadata.Filename),
		ContentType:   types.ContentType(metadata.ContentType),
		FileSizeBytes: types.FileSizeBytes(len(fileData)),
		FileData:      io.NopCloser(bytes.NewReader(fileData)),
		Config:        cfg,
	}

	result, err := s.mediaService.UploadFile(ctx, businessReq)
	if err != nil {
		return nil, connect.NewError(mapBusinessErrorToConnectCode(err), err)
	}

	return connect.NewResponse(&filesv1.UploadContentResponse{
		MediaId:    string(result.MediaID),
		ServerName: result.ServerName,
		ContentUri: result.ContentURI,
	}), nil
}

// CreateContent creates a new MXC URI without uploading content
func (s *FileServer) CreateContent(ctx context.Context, req *connect.Request[filesv1.CreateContentRequest]) (*connect.Response[filesv1.CreateContentResponse], error) {
	authClaims := security.ClaimsFromContext(ctx)
	if authClaims == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	_, err := authClaims.GetSubject()
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	mediaID := utils.GenerateRandomString(32)

	cfg := s.Service.Config().(*config.FilesConfig)

	return connect.NewResponse(&filesv1.CreateContentResponse{
		MediaId:    mediaID,
		ServerName: cfg.ServerName,
		ContentUri: fmt.Sprintf("mxc://%s/%s", cfg.ServerName, mediaID),
	}), nil
}

// GetContent downloads content from the content repository
func (s *FileServer) GetContent(ctx context.Context, req *connect.Request[filesv1.GetContentRequest]) (*connect.Response[filesv1.GetContentResponse], error) {
	cfg := s.Service.Config().(*config.FilesConfig)

	businessReq := &business.DownloadRequest{
		MediaID:            types.MediaID(req.Msg.MediaId),
		IsThumbnailRequest: false,
		Config:             cfg,
	}

	result, err := s.mediaService.DownloadFile(ctx, businessReq)
	if err != nil {
		return nil, connect.NewError(mapBusinessErrorToConnectCode(err), err)
	}
	defer util.CloseAndLogOnError(ctx, result.FileData)

	data, err := io.ReadAll(result.FileData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&filesv1.GetContentResponse{
		Content:     data,
		ContentType: result.ContentType,
		Filename:    result.Filename,
	}), nil
}

// GetContentOverrideName downloads content with a specified filename override
func (s *FileServer) GetContentOverrideName(ctx context.Context, req *connect.Request[filesv1.GetContentOverrideNameRequest]) (*connect.Response[filesv1.GetContentOverrideNameResponse], error) {
	response, err := s.GetContent(ctx, &connect.Request[filesv1.GetContentRequest]{
		Msg: &filesv1.GetContentRequest{
			ServerName: req.Msg.ServerName,
			MediaId:    req.Msg.MediaId,
		},
	})
	if err != nil {
		return nil, err
	}

	filename := req.Msg.FileName
	if filename == "" {
		filename = response.Msg.Filename
	}

	return connect.NewResponse(&filesv1.GetContentOverrideNameResponse{
		Content:     response.Msg.Content,
		ContentType: response.Msg.ContentType,
		Filename:    filename,
	}), nil
}

// GetContentThumbnail retrieves a thumbnail of the content
func (s *FileServer) GetContentThumbnail(ctx context.Context, req *connect.Request[filesv1.GetContentThumbnailRequest]) (*connect.Response[filesv1.GetContentThumbnailResponse], error) {
	cfg := s.Service.Config().(*config.FilesConfig)

	businessReq := &business.DownloadRequest{
		MediaID:            types.MediaID(req.Msg.MediaId),
		IsThumbnailRequest: true,
		Config:             cfg,
	}

	if req.Msg.Width > 0 && req.Msg.Height > 0 {
		var method string
		switch req.Msg.Method {
		case filesv1.ThumbnailMethod_SCALE:
			method = "scale"
		case filesv1.ThumbnailMethod_CROP:
			method = "crop"
		default:
			method = "scale"
		}

		businessReq.ThumbnailSize = &types.ThumbnailSize{
			Width:        int(req.Msg.Width),
			Height:       int(req.Msg.Height),
			ResizeMethod: method,
		}
	}

	result, err := s.mediaService.DownloadFile(ctx, businessReq)
	if err != nil {
		return nil, connect.NewError(mapBusinessErrorToConnectCode(err), err)
	}
	defer util.CloseAndLogOnError(ctx, result.FileData)

	data, err := io.ReadAll(result.FileData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&filesv1.GetContentThumbnailResponse{
		Content:     data,
		ContentType: result.ContentType,
	}), nil
}

// GetUrlPreview gets OpenGraph preview information for a URL
func (s *FileServer) GetUrlPreview(ctx context.Context, req *connect.Request[filesv1.GetUrlPreviewRequest]) (*connect.Response[filesv1.GetUrlPreviewResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// GetConfig retrieves the content repository configuration
func (s *FileServer) GetConfig(ctx context.Context, req *connect.Request[filesv1.GetConfigRequest]) (*connect.Response[filesv1.GetConfigResponse], error) {
	cfg := s.Service.Config().(*config.FilesConfig)

	extra := &structpb.Struct{
		Fields: make(map[string]*structpb.Value),
	}

	return connect.NewResponse(&filesv1.GetConfigResponse{
		MaxUploadBytes: int64(cfg.MaxFileSizeBytes),
		Extra:          extra,
	}), nil
}

// SearchMedia searches for media files matching specified criteria
func (s *FileServer) SearchMedia(ctx context.Context, req *connect.Request[filesv1.SearchMediaRequest]) (*connect.Response[filesv1.SearchMediaResponse], error) {
	authClaims := security.ClaimsFromContext(ctx)
	if authClaims == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	sub, err := authClaims.GetSubject()
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	ownerID := sub
	if req.Msg.OwnerId != "" && req.Msg.OwnerId != sub {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("owner_id must match authenticated user"))
	}

	var startDate *time.Time
	var endDate *time.Time

	if req.Msg.StartDate != "" {
		t, err := time.Parse(time.RFC3339, req.Msg.StartDate)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid start_date"))
		}
		startDate = &t
	}

	if req.Msg.EndDate != "" {
		t, err := time.Parse(time.RFC3339, req.Msg.EndDate)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid end_date"))
		}
		endDate = &t
	}

	if startDate != nil && endDate != nil && endDate.Before(*startDate) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("end_date must be >= start_date"))
	}

	limit := int32(req.Msg.Limit)
	if limit == 0 {
		limit = 20
	}

	businessReq := &business.SearchRequest{
		OwnerID:   types.OwnerID(ownerID),
		Query:     req.Msg.Query,
		Page:      int32(req.Msg.Page),
		Limit:     limit,
		ParentID:  types.MediaID(req.Msg.ParentId),
		StartDate: startDate,
		EndDate:   endDate,
	}

	result, err := s.mediaService.SearchMedia(ctx, businessReq)
	if err != nil {
		return nil, connect.NewError(mapBusinessErrorToConnectCode(err), err)
	}

	results := make([]*filesv1.MediaMetadata, len(result.Results))
	for i, media := range result.Results {
		results[i] = &filesv1.MediaMetadata{
			MediaId: string(media.MediaID),

			ServerName:        string(media.ServerName),
			ContentType:       string(media.ContentType),
			FileSizeBytes:     int64(media.FileSizeBytes),
			CreationTimestamp: int64(media.CreationTimestamp),
			UploadName:        string(media.UploadName),
			Base64Hash:        string(media.Base64Hash),
			OwnerId:           string(media.OwnerID),
		}
	}

	return connect.NewResponse(&filesv1.SearchMediaResponse{
		Results: results,
		Total:   int32(result.Count),
		Page:    int32(result.Page),
		HasMore: result.HasMore,
	}), nil
}
