package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"buf.build/gen/go/antinvestor/files/connectrpc/go/files/v1/filesv1connect"
	filesv1 "buf.build/gen/go/antinvestor/files/protocolbuffers/go/files/v1"
	"connectrpc.com/connect"
	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/authz"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/antinvestor/service-files/apps/default/service/utils"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/util"
	"golang.org/x/net/html"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultRequestTimeout = 10 * time.Second
	maxPreviewBodyBytes   = int64(2 << 20) // 2 MiB
	maxPreviewImageBytes  = int64(5 << 20) // 5 MiB
	maxSearchWindow       = 1000
)

// FileServer implements the Connect RPC handler for files service
// NOTE: Fields are exported for tests.
type FileServer struct {
	Service      *frame.Service
	mediaService business.MediaService
	authz        authz.Middleware
	db           storage.Database
	provider     storage.Provider

	filesv1connect.UnimplementedFilesServiceHandler
}

// NewFileServer creates a new FileServer instance
func NewFileServer(
	service *frame.Service,
	mediaService business.MediaService,
	authzMiddleware authz.Middleware,
	db storage.Database,
	provider storage.Provider,
) filesv1connect.FilesServiceHandler {
	return &FileServer{
		Service:      service,
		mediaService: mediaService,
		authz:        authzMiddleware,
		db:           db,
		provider:     provider,
	}
}

// UploadContent handles file uploads via Connect RPC streaming
func (s *FileServer) UploadContent(ctx context.Context, stream *connect.ClientStream[filesv1.UploadContentRequest]) (*connect.Response[filesv1.UploadContentResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}

	if err = s.authz.CanUploadFile(ctx, sub); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
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

	if metadata.ServerName != "" && metadata.ServerName != cfg.ServerName {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("server_name mismatch"))
	}

	if metadata.MediaId != "" && !isValidMediaID(metadata.MediaId) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
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

	fileSize := len(fileData)
	if metadata.TotalSize > 0 {
		fileSize = int(metadata.TotalSize)
	}

	isPublic := metadata.Visibility == filesv1.MediaMetadata_VISIBILITY_PUBLIC
	if metadata.Properties != nil {
		props := metadata.Properties.AsMap()
		if raw, ok := props["is_public"]; ok {
			if flag, ok := raw.(bool); ok {
				isPublic = flag
			}
		}
	}

	businessReq := &business.UploadRequest{
		OwnerID:       types.OwnerID(sub),
		MediaID:       types.MediaID(metadata.MediaId),
		UploadName:    types.Filename(metadata.Filename),
		ContentType:   types.ContentType(metadata.ContentType),
		FileSizeBytes: types.FileSizeBytes(fileSize),
		FileData:      io.NopCloser(bytes.NewReader(fileData)),
		Config:        cfg,
		IsPublic:      isPublic,
	}

	result, err := s.mediaService.UploadFile(ctx, businessReq)
	if err != nil {
		return nil, connect.NewError(mapBusinessErrorToConnectCode(err), err)
	}

	if err = queueThumbnailGeneration(ctx, s.Service, result.MediaID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&filesv1.UploadContentResponse{
		MediaId:    string(result.MediaID),
		ServerName: result.ServerName,
		ContentUri: result.ContentURI,
	}), nil
}

// CreateContent creates a new MXC URI without uploading content
func (s *FileServer) CreateContent(ctx context.Context, _ *connect.Request[filesv1.CreateContentRequest]) (*connect.Response[filesv1.CreateContentResponse], error) {
	_, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
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

	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}

	if !isValidMediaID(req.Msg.MediaId) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}

	if err = s.authz.CanViewFile(ctx, sub, req.Msg.MediaId); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}

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
		Content:  data,
		Metadata: toMediaMetadata(result.MediaMetadata),
	}), nil
}

// GetContentOverrideName downloads content with a specified filename override
func (s *FileServer) GetContentOverrideName(ctx context.Context, req *connect.Request[filesv1.GetContentOverrideNameRequest]) (*connect.Response[filesv1.GetContentOverrideNameResponse], error) {
	response, err := s.GetContent(ctx, &connect.Request[filesv1.GetContentRequest]{
		Msg: &filesv1.GetContentRequest{
			MediaId: req.Msg.MediaId,
		},
	})
	if err != nil {
		return nil, err
	}

	metadata := response.Msg.Metadata
	filename := req.Msg.FileName
	if filename == "" && metadata != nil {
		filename = metadata.Filename
	}
	if metadata != nil {
		metadata = protoCloneMediaMetadata(metadata)
		metadata.Filename = filename
	}

	return connect.NewResponse(&filesv1.GetContentOverrideNameResponse{
		Content:  response.Msg.Content,
		Metadata: metadata,
	}), nil
}

// GetContentThumbnail retrieves a thumbnail of the content
func (s *FileServer) GetContentThumbnail(ctx context.Context, req *connect.Request[filesv1.GetContentThumbnailRequest]) (*connect.Response[filesv1.GetContentThumbnailResponse], error) {
	cfg := s.Service.Config().(*config.FilesConfig)

	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}

	if !isValidMediaID(req.Msg.MediaId) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}

	if err = s.authz.CanViewFile(ctx, sub, req.Msg.MediaId); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}

	businessReq := &business.DownloadRequest{
		MediaID:            types.MediaID(req.Msg.MediaId),
		IsThumbnailRequest: true,
		Config:             cfg,
	}

	if req.Msg.Width > 0 && req.Msg.Height > 0 {
		var method string
		switch req.Msg.Method {
		case filesv1.ThumbnailMethod_SCALE:
			method = types.Scale
		case filesv1.ThumbnailMethod_CROP:
			method = types.Crop
		default:
			method = types.Scale
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
		Content:  data,
		Metadata: toMediaMetadata(result.MediaMetadata),
	}), nil
}

// GetUrlPreview gets OpenGraph preview information for a URL
func (s *FileServer) GetUrlPreview(ctx context.Context, req *connect.Request[filesv1.GetUrlPreviewRequest]) (*connect.Response[filesv1.GetUrlPreviewResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}

	urlStr := strings.TrimSpace(req.Msg.Url)
	if urlStr == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("url is required"))
	}

	parsed, err := url.Parse(urlStr)
	if err != nil || !parsed.IsAbs() {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid url"))
	}

	if !isAllowedPreviewURL(parsed) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("url not allowed"))
	}

	ctx, cancel := applyRequestTimeout(ctx, 0)
	defer cancel()

	cfg := s.Service.Config().(*config.FilesConfig)
	client := s.Service.HTTPClientManager().Client(ctx)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	resp, err := client.Do(request)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	defer util.CloseAndLogOnError(ctx, resp.Body)

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPreviewBodyBytes))
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}

	ogData, title := extractOpenGraph(bytes.NewReader(body))
	if title != "" {
		if _, ok := ogData["og:title"]; !ok {
			ogData["og:title"] = title
		}
	}

	structMap := make(map[string]any, len(ogData))
	for k, v := range ogData {
		structMap[k] = v
	}
	ogStruct, err := structpb.NewStruct(structMap)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	response := &filesv1.GetUrlPreviewResponse{
		OgData: ogStruct,
	}

	ogImageURL := ogData["og:image"]
	if ogImageURL != "" {
		imgParsed, parseErr := url.Parse(ogImageURL)
		if parseErr == nil && isAllowedPreviewURL(imgParsed) {
			if strings.EqualFold(imgParsed.Scheme, "mxc") {
				if mediaID := strings.TrimPrefix(imgParsed.Path, "/"); mediaID != "" {
					response.OgImageMediaId = mediaID
				}
			} else {
				mediaID, fetchErr := s.fetchAndStorePreviewImage(ctx, client, ogImageURL, types.OwnerID(sub), cfg)
				if fetchErr == nil && mediaID != "" {
					response.OgImageMediaId = mediaID
				}
			}
		}
	}

	return connect.NewResponse(response), nil
}

// GetConfig retrieves the content repository configuration
func (s *FileServer) GetConfig(ctx context.Context, _ *connect.Request[filesv1.GetConfigRequest]) (*connect.Response[filesv1.GetConfigResponse], error) {
	_, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}

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
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}

	ownerID := sub
	if req.Msg.OwnerId != "" && req.Msg.OwnerId != sub {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("owner_id must match authenticated user"))
	}

	limit := int32(req.Msg.Limit)
	if limit == 0 {
		limit = 20
	}
	if limit <= 0 || limit > 100 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("limit must be between 1 and 100"))
	}

	offset, err := decodeSearchCursor(req.Msg.AfterCursor, int(limit))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	startDate, endDate, err := parseSearchDates(req.Msg.CreatedAfter, req.Msg.CreatedBefore)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	fetchLimit := offset + int(limit) + 1
	if fetchLimit > maxSearchWindow {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pagination window too large"))
	}

	var visibility *bool
	switch req.Msg.Visibility {
	case filesv1.MediaMetadata_VISIBILITY_PUBLIC:
		isPublic := true
		visibility = &isPublic
	case filesv1.MediaMetadata_VISIBILITY_PRIVATE:
		isPublic := false
		visibility = &isPublic
	}

	businessReq := &business.SearchRequest{
		OwnerID:           types.OwnerID(ownerID),
		Query:             req.Msg.Query,
		Page:              0,
		Limit:             int32(fetchLimit),
		StartDate:         startDate,
		EndDate:           endDate,
		ContentTypePrefix: req.Msg.ContentType,
		Visibility:        visibility,
	}

	result, err := s.mediaService.SearchMedia(ctx, businessReq)
	if err != nil {
		return nil, connect.NewError(mapBusinessErrorToConnectCode(err), err)
	}

	sharedIDs, err := s.authz.ListUserShares(ctx, ownerID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	merged := mergeSearchResults(ctx, result.Results, sharedIDs, s.db, req.Msg, startDate, endDate)
	totalAvailable := len(merged)
	merged = applySearchOffset(merged, offset, int(limit))

	results := make([]*filesv1.MediaMetadata, len(merged))
	for i, media := range merged {
		results[i] = toMediaMetadata(media)
	}

	hasMore := offset+int(limit) < totalAvailable || result.HasMore
	nextCursor := ""
	if hasMore {
		nextCursor = encodeSearchCursor(offset + int(limit))
	}

	return connect.NewResponse(&filesv1.SearchMediaResponse{
		Results:    results,
		NextCursor: nextCursor,
	}), nil
}

func authenticatedSubject(ctx context.Context) (string, error) {
	authClaims := security.ClaimsFromContext(ctx)
	if authClaims == nil {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("missing authentication"))
	}

	sub, err := authClaims.GetSubject()
	if err != nil || strings.TrimSpace(sub) == "" {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("missing subject"))
	}

	return sub, nil
}

func applyRequestTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

func mapBusinessErrorToConnectCode(err error) connect.Code {
	if err == nil {
		return connect.CodeUnknown
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return connect.CodeDeadlineExceeded
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "not found"):
		return connect.CodeNotFound
	case strings.Contains(msg, "invalid parameter"):
		return connect.CodeInvalidArgument
	case strings.Contains(msg, "permission denied"):
		return connect.CodePermissionDenied
	default:
		return connect.CodeInternal
	}
}

func isValidMediaID(mediaID string) bool {
	if mediaID == "" {
		return false
	}
	for _, r := range mediaID {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '=' && r != '-' {
			return false
		}
	}
	return true
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	return false
}

func isAllowedPreviewURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	host := u.Hostname()
	if host == "" {
		return false
	}

	if ip := net.ParseIP(host); ip != nil {
		return !isPrivateIP(ip)
	}

	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return false
		}
	}

	return true
}

func extractOpenGraph(r io.Reader) (map[string]string, string) {
	og := map[string]string{}
	var title string
	var inTitle bool

	tokenizer := html.NewTokenizer(r)
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			return og, strings.TrimSpace(title)
		case html.StartTagToken, html.SelfClosingTagToken:
			name, hasAttr := tokenizer.TagName()
			tagName := strings.ToLower(string(name))

			if tagName == "title" {
				inTitle = true
				continue
			}

			if tagName == "meta" && hasAttr {
				var property string
				var content string
				for {
					key, val, more := tokenizer.TagAttr()
					attr := strings.ToLower(string(key))
					switch attr {
					case "property", "name":
						property = strings.ToLower(string(val))
					case "content":
						content = string(val)
					}
					if !more {
						break
					}
				}
				if strings.HasPrefix(property, "og:") && content != "" {
					og[property] = content
				}
			}
		case html.EndTagToken:
			name, _ := tokenizer.TagName()
			if strings.ToLower(string(name)) == "title" {
				inTitle = false
			}
		case html.TextToken:
			if inTitle {
				title += string(tokenizer.Text())
			}
		}
	}
}

func (s *FileServer) fetchAndStorePreviewImage(ctx context.Context, client *http.Client, imageURL string, ownerID types.OwnerID, cfg *config.FilesConfig) (string, error) {
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, imageURL, nil)
	if err != nil {
		return "", err
	}

	headResp, err := client.Do(headReq)
	if err != nil {
		return "", err
	}
	defer util.CloseAndLogOnError(ctx, headResp.Body)

	contentType := strings.ToLower(strings.TrimSpace(headResp.Header.Get("Content-Type")))
	if contentType != "" && !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("preview image content-type is not image")
	}

	maxAllowed := maxPreviewImageBytes
	if cfg.MaxFileSizeBytes > 0 && int64(cfg.MaxFileSizeBytes) < maxAllowed {
		maxAllowed = int64(cfg.MaxFileSizeBytes)
	}

	if length := headResp.Header.Get("Content-Length"); length != "" {
		size, parseErr := strconv.ParseInt(length, 10, 64)
		if parseErr == nil && size > maxAllowed {
			return "", fmt.Errorf("preview image too large")
		}
	}

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", err
	}

	getResp, err := client.Do(getReq)
	if err != nil {
		return "", err
	}
	defer util.CloseAndLogOnError(ctx, getResp.Body)

	if contentType == "" {
		contentType = strings.ToLower(strings.TrimSpace(getResp.Header.Get("Content-Type")))
	}
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("preview image content-type is not image")
	}

	payload, err := io.ReadAll(io.LimitReader(getResp.Body, maxAllowed+1))
	if err != nil {
		return "", err
	}
	if int64(len(payload)) > maxAllowed {
		return "", fmt.Errorf("preview image too large")
	}

	filename := "preview"
	if parsed, parseErr := url.Parse(imageURL); parseErr == nil {
		if base := path.Base(parsed.Path); base != "" && base != "/" && base != "." {
			filename = base
		}
	}

	result, err := s.mediaService.UploadFile(ctx, &business.UploadRequest{
		OwnerID:       ownerID,
		UploadName:    types.Filename(filename),
		ContentType:   types.ContentType(contentType),
		FileSizeBytes: types.FileSizeBytes(len(payload)),
		FileData:      bytes.NewReader(payload),
		Config:        cfg,
		IsPublic:      false,
	})
	if err != nil {
		return "", err
	}

	return string(result.MediaID), nil
}

func fetchContentLength(ctx context.Context, client *http.Client, urlStr string) (int64, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, urlStr, nil)
	if err != nil {
		return 0, err
	}

	resp, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer util.CloseAndLogOnError(ctx, resp.Body)

	length := resp.Header.Get("Content-Length")
	if length == "" {
		return 0, fmt.Errorf("content length unknown")
	}

	size, err := strconv.ParseInt(length, 10, 64)
	if err != nil {
		return 0, err
	}

	return size, nil
}

func matchesSearchQuery(metadata *types.MediaMetadata, query string) bool {
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		return true
	}
	if strings.Contains(strings.ToLower(string(metadata.UploadName)), q) {
		return true
	}
	if strings.Contains(strings.ToLower(string(metadata.ContentType)), q) {
		return true
	}
	return false
}

func encodeSearchCursor(offset int) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("o:%d", offset)))
}

func decodeSearchCursor(cursor string, limit int) (int, error) {
	if strings.TrimSpace(cursor) == "" {
		return 0, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor")
	}

	value := strings.TrimPrefix(string(raw), "o:")
	if value == string(raw) {
		return 0, fmt.Errorf("invalid cursor")
	}

	offset, err := strconv.Atoi(value)
	if err != nil || offset < 0 {
		return 0, fmt.Errorf("invalid cursor")
	}

	if limit > 0 && offset%limit != 0 {
		return 0, fmt.Errorf("invalid cursor")
	}

	return offset, nil
}

func parseSearchDates(after *timestamppb.Timestamp, before *timestamppb.Timestamp) (*time.Time, *time.Time, error) {
	var startDate *time.Time
	var endDate *time.Time

	if after != nil {
		t := after.AsTime()
		startDate = &t
	}
	if before != nil {
		t := before.AsTime()
		endDate = &t
	}
	if startDate != nil && endDate != nil && endDate.Before(*startDate) {
		return nil, nil, fmt.Errorf("created_before must be >= created_after")
	}

	return startDate, endDate, nil
}

func mergeSearchResults(ctx context.Context, ownerResults []*types.MediaMetadata, sharedIDs []string, db storage.Database, req *filesv1.SearchMediaRequest, startDate *time.Time, endDate *time.Time) []*types.MediaMetadata {
	seen := map[string]struct{}{}
	merged := make([]*types.MediaMetadata, 0, len(ownerResults)+len(sharedIDs))

	for _, media := range ownerResults {
		if media == nil {
			continue
		}
		if !matchSearchFilters(media, req, startDate, endDate) {
			continue
		}
		seen[string(media.MediaID)] = struct{}{}
		merged = append(merged, media)
	}

	for _, sharedID := range sharedIDs {
		if _, ok := seen[sharedID]; ok {
			continue
		}
		meta, err := db.GetMediaMetadata(ctx, types.MediaID(sharedID))
		if err != nil || meta == nil {
			continue
		}
		if !matchSearchFilters(meta, req, startDate, endDate) {
			continue
		}
		seen[sharedID] = struct{}{}
		merged = append(merged, meta)
	}

	sort.Slice(merged, func(i, j int) bool {
		if merged[i].CreationTimestamp == merged[j].CreationTimestamp {
			return string(merged[i].MediaID) > string(merged[j].MediaID)
		}
		return merged[i].CreationTimestamp > merged[j].CreationTimestamp
	})

	return merged
}

func matchSearchFilters(meta *types.MediaMetadata, req *filesv1.SearchMediaRequest, startDate *time.Time, endDate *time.Time) bool {
	if startDate != nil && time.UnixMilli(int64(meta.CreationTimestamp)).Before(*startDate) {
		return false
	}
	if endDate != nil && time.UnixMilli(int64(meta.CreationTimestamp)).After(*endDate) {
		return false
	}
	if req.ContentType != "" && !strings.HasPrefix(strings.ToLower(string(meta.ContentType)), strings.ToLower(req.ContentType)) {
		return false
	}
	if req.Visibility == filesv1.MediaMetadata_VISIBILITY_PUBLIC && !meta.IsPublic {
		return false
	}
	if req.Visibility == filesv1.MediaMetadata_VISIBILITY_PRIVATE && meta.IsPublic {
		return false
	}
	if !matchesSearchQuery(meta, req.Query) {
		return false
	}
	return true
}

func applySearchOffset(merged []*types.MediaMetadata, offset int, limit int) []*types.MediaMetadata {
	if offset >= len(merged) {
		return []*types.MediaMetadata{}
	}
	end := offset + limit
	if end > len(merged) {
		end = len(merged)
	}
	return merged[offset:end]
}

func queueThumbnailGeneration(ctx context.Context, service *frame.Service, mediaID types.MediaID) error {
	cfg := service.Config().(*config.FilesConfig)
	thumbnailGenerationQueue := cfg.QueueThumbnailsGenerateName
	return service.QueueManager().Publish(ctx, thumbnailGenerationQueue, map[string]string{
		"media_id": string(mediaID),
	})
}

func toMediaMetadata(metadata *types.MediaMetadata) *filesv1.MediaMetadata {
	if metadata == nil {
		return nil
	}

	visibility := filesv1.MediaMetadata_VISIBILITY_PRIVATE
	if metadata.IsPublic {
		visibility = filesv1.MediaMetadata_VISIBILITY_PUBLIC
	}

	return &filesv1.MediaMetadata{
		MediaId:        string(metadata.MediaID),
		ContentType:    string(metadata.ContentType),
		FileSizeBytes:  int64(metadata.FileSizeBytes),
		CreatedAt:      timestamppb.New(time.UnixMilli(int64(metadata.CreationTimestamp))),
		Filename:       string(metadata.UploadName),
		ChecksumSha256: string(metadata.Base64Hash),
		OwnerId:        string(metadata.OwnerID),
		Visibility:     visibility,
	}
}

func protoCloneMediaMetadata(in *filesv1.MediaMetadata) *filesv1.MediaMetadata {
	if in == nil {
		return nil
	}
	return proto.Clone(in).(*filesv1.MediaMetadata)
}
