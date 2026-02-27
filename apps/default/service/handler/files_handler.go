package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
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
	maxInMemoryFileBytes  = int64(1536 << 10) // 1.5 MiB hard in-memory cap
	maxPreviewBodyBytes   = maxInMemoryFileBytes
	maxPreviewImageBytes  = maxInMemoryFileBytes
	maxSearchWindow       = 1000
	// Memory limits for content endpoints to prevent OOM
	maxContentBytes       = maxInMemoryFileBytes
	maxThumbnailBytes     = maxInMemoryFileBytes
	contentReadBufferSize = 64 << 10 // 64 KiB buffer for reading content
	maxMultipartPartBytes = maxInMemoryFileBytes
	maxBatchGetItems      = 16
	maxBatchDeleteItems   = 500
	maxBatchResponseBytes = int64(8 << 20)
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
	if err = validateUploadMetadata(metadata, cfg); err != nil {
		return nil, err
	}

	tempFile, err := os.CreateTemp("", "files-upload-*")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer func() {
		_ = os.Remove(tempFile.Name())
	}()

	bytesReceived, err := readUploadChunksToTempFile(stream, tempFile, int64(cfg.MaxFileSizeBytes))
	if err != nil {
		_ = tempFile.Close()
		return nil, err
	}

	if metadata.TotalSize > 0 && bytesReceived != metadata.TotalSize {
		_ = tempFile.Close()
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("metadata total_size does not match received bytes"))
	}

	if _, err = tempFile.Seek(0, io.SeekStart); err != nil {
		_ = tempFile.Close()
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer util.CloseAndLogOnError(ctx, tempFile)

	isPublic := resolveUploadVisibility(metadata)

	businessReq := &business.UploadRequest{
		OwnerID:       types.OwnerID(sub),
		MediaID:       types.MediaID(metadata.MediaId),
		UploadName:    types.Filename(metadata.Filename),
		ContentType:   types.ContentType(metadata.ContentType),
		FileSizeBytes: types.FileSizeBytes(bytesReceived),
		FileData:      tempFile,
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

	storedMeta, _ := s.db.GetMediaMetadata(ctx, result.MediaID)

	return connect.NewResponse(&filesv1.UploadContentResponse{
		MediaId:    string(result.MediaID),
		ServerName: result.ServerName,
		ContentUri: result.ContentURI,
		Metadata:   toMediaMetadata(storedMeta),
	}), nil
}

func validateUploadMetadata(metadata *filesv1.UploadMetadata, cfg *config.FilesConfig) error {
	if metadata == nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("metadata is required"))
	}
	if metadata.ServerName != "" && metadata.ServerName != cfg.ServerName {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("server_name mismatch"))
	}
	if metadata.MediaId != "" && !isValidMediaID(metadata.MediaId) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}
	if cfg.MaxFileSizeBytes > 0 && metadata.TotalSize > int64(cfg.MaxFileSizeBytes) {
		return connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("invalid parameter: HTTP Content-Length is greater than the maximum allowed upload size (%v)", cfg.MaxFileSizeBytes))
	}
	return nil
}

func readUploadChunksToTempFile(stream *connect.ClientStream[filesv1.UploadContentRequest], tempFile *os.File, maxAllowed int64) (int64, error) {
	bytesReceived := int64(0)
	for stream.Receive() {
		msg := stream.Msg()
		chunk := msg.GetChunk()
		if chunk == nil {
			continue
		}
		if int64(len(chunk)) > maxInMemoryFileBytes {
			return 0, connect.NewError(connect.CodeFailedPrecondition,
				fmt.Errorf("chunk exceeds maximum in-memory size of %d bytes", maxInMemoryFileBytes))
		}
		bytesReceived += int64(len(chunk))
		if maxAllowed > 0 && bytesReceived > maxAllowed {
			return 0, connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("invalid parameter: HTTP Content-Length is greater than the maximum allowed upload size (%v)", maxAllowed))
		}
		if _, err := tempFile.Write(chunk); err != nil {
			return 0, connect.NewError(connect.CodeInternal, err)
		}
	}
	if err := stream.Err(); err != nil {
		return 0, connect.NewError(connect.CodeInternal, err)
	}
	return bytesReceived, nil
}

func resolveUploadVisibility(metadata *filesv1.UploadMetadata) bool {
	isPublic := metadata.Visibility == filesv1.MediaMetadata_VISIBILITY_PUBLIC
	if metadata.Properties == nil {
		return isPublic
	}
	props := metadata.Properties.AsMap()
	if raw, ok := props["is_public"]; ok {
		if flag, ok := raw.(bool); ok {
			return flag
		}
	}
	return isPublic
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

	// Check file size before reading to prevent OOM
	if result.MediaMetadata != nil && result.MediaMetadata.FileSizeBytes > types.FileSizeBytes(maxContentBytes) {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("file size %d exceeds maximum allowed %d bytes", result.MediaMetadata.FileSizeBytes, maxContentBytes))
	}

	// Use a size-limited reader to prevent memory issues
	limitedReader := &io.LimitedReader{R: result.FileData, N: maxContentBytes + 1}
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check if we hit the limit
	if limitedReader.N <= 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("file size exceeds maximum allowed %d bytes", maxContentBytes))
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

func (s *FileServer) HeadContent(ctx context.Context, req *connect.Request[filesv1.HeadContentRequest]) (*connect.Response[filesv1.HeadContentResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	mediaID := req.Msg.GetMediaId()
	if !isValidMediaID(mediaID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}
	if err = s.authz.CanViewFile(ctx, sub, mediaID); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}

	metadata, err := s.db.GetMediaMetadata(ctx, types.MediaID(mediaID))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if metadata == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("media not found"))
	}

	return connect.NewResponse(&filesv1.HeadContentResponse{
		Metadata: toMediaMetadata(metadata),
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

	// Check file size before reading to prevent OOM
	// Thumbnails should be small, so use a strict limit
	if result.MediaMetadata != nil && result.MediaMetadata.FileSizeBytes > types.FileSizeBytes(maxThumbnailBytes) {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("thumbnail size %d exceeds maximum allowed %d bytes", result.MediaMetadata.FileSizeBytes, maxThumbnailBytes))
	}

	// Use a size-limited reader to prevent memory issues
	limitedReader := &io.LimitedReader{R: result.FileData, N: maxThumbnailBytes + 1}
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Check if we hit the limit
	if limitedReader.N <= 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("thumbnail size exceeds maximum allowed %d bytes", maxThumbnailBytes))
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

type userUsageStore interface {
	GetUserUsage(ctx context.Context, ownerID types.OwnerID) (int64, int, error)
}

type latestStorageStats interface {
	TotalBytes() int64
	FileCount() int
	UserCount() int
	PublicBytes() int64
	PrivateBytes() int64
}

type storageStatsStore interface {
	GetLatestStats(ctx context.Context) (latestStorageStats, error)
}

type deleteStore interface {
	DeleteMedia(ctx context.Context, mediaID types.MediaID) error
}

type multipartStore interface {
	StoreUpload(ctx context.Context, upload interface {
		GetID() string
		GetOwnerID() string
		GetMediaID() string
		GetUploadName() string
		GetContentType() string
		GetTotalSize() int64
		GetPartSize() int64
		GetPartCount() int
		GetUploadState() string
		GetExpiresAt() *time.Time
	}) error
	GetUpload(ctx context.Context, uploadID string) (interface {
		ID() string
		OwnerID() string
		MediaID() string
		UploadName() string
		ContentType() string
		TotalSize() int64
		PartSize() int64
		PartCount() int
		UploadedParts() int
		UploadState() string
		ExpiresAt() *time.Time
	}, error)
	UpdateUploadState(ctx context.Context, uploadID string, state string) error
	DeleteUpload(ctx context.Context, uploadID string) error
	StorePart(ctx context.Context, part interface {
		GetID() string
		GetUploadID() string
		GetPartNumber() int
		GetEtag() string
		GetSize() int64
		GetContentHash() string
		GetStoragePath() string
	}) error
	GetParts(ctx context.Context, uploadID string) ([]interface {
		ID() string
		UploadID() string
		PartNumber() int
		Etag() string
		Size() int64
		ContentHash() string
		StoragePath() string
	}, error)
	GetPart(ctx context.Context, uploadID string, partNumber int) (interface {
		ID() string
		UploadID() string
		PartNumber() int
		Etag() string
		Size() int64
		ContentHash() string
		StoragePath() string
	}, error)
}

type versionStore interface {
	GetVersion(ctx context.Context, mediaID string, versionNumber int) (interface {
		ID() string
		MediaID() string
		VersionNumber() int
		ContentHash() string
		FileSize() int64
		UploadName() string
		ContentType() string
		StoragePath() string
		CreatedAt() time.Time
	}, error)
	GetVersionsPaginated(ctx context.Context, mediaID string, limit, offset int) ([]interface {
		ID() string
		MediaID() string
		VersionNumber() int
		ContentHash() string
		FileSize() int64
		UploadName() string
		ContentType() string
		CreatedAt() time.Time
		CreatedBy() string
	}, int, error)
	RestoreMediaToVersion(ctx context.Context, mediaID string, versionNumber int, restoredBy string) (*types.MediaMetadata, error)
}

type patchStore interface {
	UpdateMediaMetadata(ctx context.Context, mediaID types.MediaID, updates map[string]any) (*types.MediaMetadata, error)
}

type retentionStore interface {
	ApplyRetention(ctx context.Context, retention interface {
		GetMediaID() string
		GetPolicyID() string
		GetExpiresAt() *time.Time
		GetIsLocked() bool
	}) error
	GetRetention(ctx context.Context, mediaID string) (interface {
		MediaID() string
		PolicyID() string
		AppliedAt() time.Time
		ExpiresAt() *time.Time
		IsLocked() bool
	}, error)
	RemoveRetention(ctx context.Context, mediaID string) error
	GetPolicy(ctx context.Context, policyID string) (interface {
		ID() string
		Name() string
		Description() string
		RetentionDays() int
		IsDefault() bool
		IsSystem() bool
		OwnerID() string
	}, error)
	ListPolicies(ctx context.Context, ownerID string, limit, offset int) ([]interface {
		ID() string
		Name() string
		Description() string
		RetentionDays() int
		IsDefault() bool
		IsSystem() bool
		OwnerID() string
	}, int, error)
}

type multipartUploadRequest struct {
	id          string
	ownerID     string
	mediaID     string
	uploadName  string
	contentType string
	totalSize   int64
	partSize    int64
	partCount   int
	uploadState string
	expiresAt   *time.Time
}

func (m multipartUploadRequest) GetID() string          { return m.id }
func (m multipartUploadRequest) GetOwnerID() string     { return m.ownerID }
func (m multipartUploadRequest) GetMediaID() string     { return m.mediaID }
func (m multipartUploadRequest) GetUploadName() string  { return m.uploadName }
func (m multipartUploadRequest) GetContentType() string { return m.contentType }
func (m multipartUploadRequest) GetTotalSize() int64    { return m.totalSize }
func (m multipartUploadRequest) GetPartSize() int64     { return m.partSize }
func (m multipartUploadRequest) GetPartCount() int      { return m.partCount }
func (m multipartUploadRequest) GetUploadState() string { return m.uploadState }
func (m multipartUploadRequest) GetExpiresAt() *time.Time {
	return m.expiresAt
}

type multipartPartRequest struct {
	id          string
	uploadID    string
	partNumber  int
	etag        string
	size        int64
	contentHash string
	storagePath string
}

func (m multipartPartRequest) GetID() string          { return m.id }
func (m multipartPartRequest) GetUploadID() string    { return m.uploadID }
func (m multipartPartRequest) GetPartNumber() int     { return m.partNumber }
func (m multipartPartRequest) GetEtag() string        { return m.etag }
func (m multipartPartRequest) GetSize() int64         { return m.size }
func (m multipartPartRequest) GetContentHash() string { return m.contentHash }
func (m multipartPartRequest) GetStoragePath() string { return m.storagePath }

type fileRetentionRequest struct {
	mediaID   string
	policyID  string
	expiresAt *time.Time
	isLocked  bool
}

func (r fileRetentionRequest) GetMediaID() string       { return r.mediaID }
func (r fileRetentionRequest) GetPolicyID() string      { return r.policyID }
func (r fileRetentionRequest) GetExpiresAt() *time.Time { return r.expiresAt }
func (r fileRetentionRequest) GetIsLocked() bool        { return r.isLocked }

func (s *FileServer) GetUserUsage(ctx context.Context, req *connect.Request[filesv1.GetUserUsageRequest]) (*connect.Response[filesv1.GetUserUsageResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}

	targetUser := sub
	if requested := strings.TrimSpace(req.Msg.GetUserId()); requested != "" {
		if requested != sub {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("user_id must match authenticated user"))
		}
		targetUser = requested
	}

	usageRepo, ok := s.db.(userUsageStore)
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("user usage is not available"))
	}

	totalBytes, totalFiles, err := usageRepo.GetUserUsage(ctx, types.OwnerID(targetUser))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	now := time.Now().UTC()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)

	return connect.NewResponse(&filesv1.GetUserUsageResponse{
		Usage: &filesv1.UsageStats{
			TotalFiles: int64(totalFiles),
			TotalBytes: totalBytes,
		},
		PeriodStart: timestamppb.New(periodStart),
		PeriodEnd:   timestamppb.New(periodEnd),
	}), nil
}

func (s *FileServer) GetStorageStats(ctx context.Context, _ *connect.Request[filesv1.GetStorageStatsRequest]) (*connect.Response[filesv1.GetStorageStatsResponse], error) {
	if _, err := authenticatedSubject(ctx); err != nil {
		return nil, err
	}

	var totalBytes int64
	var totalFiles int64
	var totalUsers int64

	statsRepo, ok := s.db.(storageStatsStore)
	if ok {
		stats, err := statsRepo.GetLatestStats(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if stats != nil {
			totalBytes = stats.TotalBytes()
			totalFiles = int64(stats.FileCount())
			totalUsers = int64(stats.UserCount())
		}
	}

	return connect.NewResponse(&filesv1.GetStorageStatsResponse{
		TotalBytes: totalBytes,
		TotalFiles: totalFiles,
		TotalUsers: totalUsers,
	}), nil
}

func (s *FileServer) GetSignedUploadUrl(ctx context.Context, req *connect.Request[filesv1.GetSignedUploadUrlRequest]) (*connect.Response[filesv1.GetSignedUploadUrlResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	if !isValidMediaID(req.Msg.GetMediaId()) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}
	if err = s.authz.CanUploadFile(ctx, sub); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	expiresAt, err := resolveURLExpiry(req.Msg.GetExpiresSeconds())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	uploadURL, err := s.signedFileURL(ctx, "upload", req.Msg.GetMediaId(), sub, expiresAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&filesv1.GetSignedUploadUrlResponse{UploadUrl: uploadURL}), nil
}

func (s *FileServer) GetSignedDownloadUrl(ctx context.Context, req *connect.Request[filesv1.GetSignedDownloadUrlRequest]) (*connect.Response[filesv1.GetSignedDownloadUrlResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	mediaID := req.Msg.GetMediaId()
	if !isValidMediaID(mediaID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}
	if err = s.authz.CanViewFile(ctx, sub, mediaID); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	expiresAt, err := resolveURLExpiry(req.Msg.GetExpiresSeconds())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	downloadURL, err := s.signedFileURL(ctx, "download", mediaID, sub, expiresAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&filesv1.GetSignedDownloadUrlResponse{DownloadUrl: downloadURL}), nil
}

func (s *FileServer) CreateMultipartUpload(ctx context.Context, req *connect.Request[filesv1.CreateMultipartUploadRequest]) (*connect.Response[filesv1.CreateMultipartUploadResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.authz.CanUploadFile(ctx, sub); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	if req.Msg.GetTotalSize() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("total_size must be greater than zero"))
	}
	cfg := s.Service.Config().(*config.FilesConfig)
	if cfg.MaxFileSizeBytes > 0 && req.Msg.GetTotalSize() > int64(cfg.MaxFileSizeBytes) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("total_size exceeds configured max upload size"))
	}
	if strings.TrimSpace(req.Msg.GetFilename()) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("filename is required"))
	}
	ownerID := sub
	store, ok := s.db.(multipartStore)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("multipart storage is unavailable"))
	}
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)
	if req.Msg.GetExpiresAt() != nil {
		exp := req.Msg.GetExpiresAt().AsTime().UTC()
		if !exp.After(now) {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("expires_at must be in the future"))
		}
		expiresAt = exp
	}
	partCount := int(math.Ceil(float64(req.Msg.GetTotalSize()) / float64(maxMultipartPartBytes)))
	if partCount <= 0 {
		partCount = 1
	}
	uploadID := utils.GenerateRandomString(32)
	mediaID := utils.GenerateRandomString(32)
	upload := multipartUploadRequest{
		id:          uploadID,
		ownerID:     ownerID,
		mediaID:     mediaID,
		uploadName:  req.Msg.GetFilename(),
		contentType: req.Msg.GetContentType(),
		totalSize:   req.Msg.GetTotalSize(),
		partSize:    maxMultipartPartBytes,
		partCount:   partCount,
		uploadState: "pending",
		expiresAt:   &expiresAt,
	}
	if err = store.StoreUpload(ctx, upload); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&filesv1.CreateMultipartUploadResponse{UploadId: uploadID}), nil
}

func (s *FileServer) UploadMultipartPart(ctx context.Context, req *connect.Request[filesv1.UploadMultipartPartRequest]) (*connect.Response[filesv1.UploadMultipartPartResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	store, upload, err := s.getOwnedUpload(ctx, req.Msg.GetUploadId(), sub)
	if err != nil {
		return nil, err
	}
	if upload.UploadState() != "pending" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("upload is not pending"))
	}
	partNumber := int(req.Msg.GetPartNumber())
	if partNumber <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("part_number must be greater than zero"))
	}
	partContent := req.Msg.GetContent()
	if len(partContent) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("content is required"))
	}
	if int64(len(partContent)) > maxMultipartPartBytes {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("part size exceeds %d bytes in-memory limit", maxMultipartPartBytes))
	}
	existingPart, getErr := store.GetPart(ctx, upload.ID(), partNumber)
	if getErr == nil && existingPart != nil {
		return connect.NewResponse(&filesv1.UploadMultipartPartResponse{
			Etag:       existingPart.Etag(),
			PartNumber: int32(existingPart.PartNumber()),
			Size:       existingPart.Size(),
		}), nil
	}

	tmpFile, err := os.CreateTemp("", "multipart-part-*")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()
	if _, err = tmpFile.Write(partContent); err != nil {
		_ = tmpFile.Close()
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	_ = tmpFile.Close()

	sum := sha256.Sum256(partContent)
	etag := hex.EncodeToString(sum[:])
	storagePath := filepath.ToSlash(filepath.Join("multipart", upload.ID(), fmt.Sprintf("part-%06d", partNumber)))
	bucket := s.provider.GetBucket(false)
	_, err = s.provider.UploadFile(ctx, bucket, types.Path(tmpFile.Name()), types.Path(storagePath))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	part := multipartPartRequest{
		id:          utils.GenerateRandomString(32),
		uploadID:    upload.ID(),
		partNumber:  partNumber,
		etag:        etag,
		size:        int64(len(partContent)),
		contentHash: etag,
		storagePath: storagePath,
	}
	if err = store.StorePart(ctx, part); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&filesv1.UploadMultipartPartResponse{
		Etag:       etag,
		PartNumber: int32(partNumber),
		Size:       int64(len(partContent)),
	}), nil
}

func (s *FileServer) CompleteMultipartUpload(ctx context.Context, req *connect.Request[filesv1.CompleteMultipartUploadRequest]) (*connect.Response[filesv1.CompleteMultipartUploadResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	store, upload, err := s.getOwnedUpload(ctx, req.Msg.GetUploadId(), sub)
	if err != nil {
		return nil, err
	}
	if upload.UploadState() != "pending" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("upload is not pending"))
	}
	if len(req.Msg.GetParts()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("parts are required"))
	}

	parts, err := store.GetParts(ctx, upload.ID())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	partByNumber := make(map[int]struct {
		etag string
		path string
	}, len(parts))
	for _, p := range parts {
		partByNumber[p.PartNumber()] = struct {
			etag string
			path string
		}{etag: p.Etag(), path: p.StoragePath()}
	}
	sort.Slice(req.Msg.Parts, func(i, j int) bool {
		return req.Msg.Parts[i].GetPartNumber() < req.Msg.Parts[j].GetPartNumber()
	})

	assembled, err := os.CreateTemp("", "multipart-assembled-*")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer func() {
		_ = os.Remove(assembled.Name())
	}()
	totalWritten := int64(0)
	for _, reqPart := range req.Msg.GetParts() {
		entry, ok := partByNumber[int(reqPart.GetPartNumber())]
		if !ok {
			_ = assembled.Close()
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("missing part %d", reqPart.GetPartNumber()))
		}
		if entry.etag != reqPart.GetEtag() {
			_ = assembled.Close()
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("etag mismatch for part %d", reqPart.GetPartNumber()))
		}
		reader, cleanup, dlErr := s.provider.DownloadFile(ctx, s.provider.GetBucket(false), types.Path(entry.path))
		if dlErr != nil {
			_ = assembled.Close()
			return nil, connect.NewError(connect.CodeInternal, dlErr)
		}
		n, copyErr := io.CopyBuffer(assembled, reader, make([]byte, contentReadBufferSize))
		cleanup()
		if copyErr != nil {
			_ = assembled.Close()
			return nil, connect.NewError(connect.CodeInternal, copyErr)
		}
		totalWritten += n
	}
	if _, err = assembled.Seek(0, io.SeekStart); err != nil {
		_ = assembled.Close()
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer util.CloseAndLogOnError(ctx, assembled)

	cfg := s.Service.Config().(*config.FilesConfig)
	result, err := s.mediaService.UploadFile(ctx, &business.UploadRequest{
		OwnerID:       types.OwnerID(upload.OwnerID()),
		MediaID:       types.MediaID(upload.MediaID()),
		UploadName:    types.Filename(upload.UploadName()),
		ContentType:   types.ContentType(upload.ContentType()),
		FileSizeBytes: types.FileSizeBytes(totalWritten),
		FileData:      assembled,
		Config:        cfg,
		IsPublic:      false,
	})
	if err != nil {
		return nil, connect.NewError(mapBusinessErrorToConnectCode(err), err)
	}
	if err = store.UpdateUploadState(ctx, upload.ID(), "completed"); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	metadata, err := s.db.GetMediaMetadata(ctx, result.MediaID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&filesv1.CompleteMultipartUploadResponse{
		Metadata: toMediaMetadata(metadata),
	}), nil
}

func (s *FileServer) AbortMultipartUpload(ctx context.Context, req *connect.Request[filesv1.AbortMultipartUploadRequest]) (*connect.Response[filesv1.AbortMultipartUploadResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	store, upload, err := s.getOwnedUpload(ctx, req.Msg.GetUploadId(), sub)
	if err != nil {
		return nil, err
	}
	if err = store.UpdateUploadState(ctx, upload.ID(), "aborted"); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err = store.DeleteUpload(ctx, upload.ID()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&filesv1.AbortMultipartUploadResponse{Aborted: true}), nil
}

func (s *FileServer) ListMultipartParts(ctx context.Context, req *connect.Request[filesv1.ListMultipartPartsRequest]) (*connect.Response[filesv1.ListMultipartPartsResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	store, _, err := s.getOwnedUpload(ctx, req.Msg.GetUploadId(), sub)
	if err != nil {
		return nil, err
	}
	parts, err := store.GetParts(ctx, req.Msg.GetUploadId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	respParts := make([]*filesv1.ListMultipartPartsResponse_Part, 0, len(parts))
	for _, part := range parts {
		respParts = append(respParts, &filesv1.ListMultipartPartsResponse_Part{
			PartNumber: int32(part.PartNumber()),
			Etag:       part.Etag(),
			Size:       part.Size(),
		})
	}
	return connect.NewResponse(&filesv1.ListMultipartPartsResponse{
		Parts: respParts,
	}), nil
}

func (s *FileServer) DeleteContent(ctx context.Context, req *connect.Request[filesv1.DeleteContentRequest]) (*connect.Response[filesv1.DeleteContentResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	mediaID := req.Msg.GetMediaId()
	if !isValidMediaID(mediaID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}
	if err = s.authz.CanDeleteFile(ctx, sub, mediaID); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	deleter, ok := s.db.(deleteStore)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete operation unavailable"))
	}
	if err = deleter.DeleteMedia(ctx, types.MediaID(mediaID)); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&filesv1.DeleteContentResponse{Success: true}), nil
}

func (s *FileServer) BatchGetContent(ctx context.Context, req *connect.Request[filesv1.BatchGetContentRequest]) (*connect.Response[filesv1.BatchGetContentResponse], error) {
	if _, err := authenticatedSubject(ctx); err != nil {
		return nil, err
	}
	if len(req.Msg.GetMediaIds()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("media_ids must not be empty"))
	}
	if len(req.Msg.GetMediaIds()) > maxBatchGetItems {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("media_ids must not exceed %d", maxBatchGetItems))
	}
	results := make([]*filesv1.BatchGetContentResponse_ContentResult, 0, len(req.Msg.GetMediaIds()))
	totalBytes := int64(0)
	budgetExceeded := false
	for _, mediaID := range req.Msg.GetMediaIds() {
		if budgetExceeded {
			results = append(results, &filesv1.BatchGetContentResponse_ContentResult{
				MediaId: mediaID,
				Result: &filesv1.BatchGetContentResponse_ContentResult_Error{
					Error: "batch response memory limit exceeded",
				},
			})
			continue
		}
		contentResp, err := s.GetContent(ctx, connect.NewRequest(&filesv1.GetContentRequest{MediaId: mediaID}))
		if err != nil {
			results = append(results, &filesv1.BatchGetContentResponse_ContentResult{
				MediaId: mediaID,
				Result: &filesv1.BatchGetContentResponse_ContentResult_Error{
					Error: err.Error(),
				},
			})
			continue
		}
		contentBytes := int64(len(contentResp.Msg.GetContent()))
		if totalBytes+contentBytes > maxBatchResponseBytes {
			budgetExceeded = true
			results = append(results, &filesv1.BatchGetContentResponse_ContentResult{
				MediaId: mediaID,
				Result: &filesv1.BatchGetContentResponse_ContentResult_Error{
					Error: "batch response memory limit exceeded",
				},
			})
			continue
		}
		totalBytes += contentBytes
		results = append(results, &filesv1.BatchGetContentResponse_ContentResult{
			MediaId: mediaID,
			Result: &filesv1.BatchGetContentResponse_ContentResult_Content{
				Content: contentResp.Msg,
			},
		})
	}
	return connect.NewResponse(&filesv1.BatchGetContentResponse{Results: results}), nil
}

func (s *FileServer) BatchDeleteContent(ctx context.Context, req *connect.Request[filesv1.BatchDeleteContentRequest]) (*connect.Response[filesv1.BatchDeleteContentResponse], error) {
	if _, err := authenticatedSubject(ctx); err != nil {
		return nil, err
	}
	if len(req.Msg.GetMediaIds()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("media_ids must not be empty"))
	}
	if len(req.Msg.GetMediaIds()) > maxBatchDeleteItems {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("media_ids must not exceed %d", maxBatchDeleteItems))
	}
	results := make([]*filesv1.BatchDeleteContentResponse_DeleteResult, 0, len(req.Msg.GetMediaIds()))
	for _, mediaID := range req.Msg.GetMediaIds() {
		_, err := s.DeleteContent(ctx, connect.NewRequest(&filesv1.DeleteContentRequest{
			MediaId:    mediaID,
			HardDelete: req.Msg.GetHardDelete(),
		}))
		if err != nil {
			results = append(results, &filesv1.BatchDeleteContentResponse_DeleteResult{
				MediaId: mediaID,
				Success: false,
				Error:   err.Error(),
			})
			continue
		}
		results = append(results, &filesv1.BatchDeleteContentResponse_DeleteResult{
			MediaId: mediaID,
			Success: true,
		})
	}
	return connect.NewResponse(&filesv1.BatchDeleteContentResponse{Results: results}), nil
}

func (s *FileServer) GetVersions(ctx context.Context, req *connect.Request[filesv1.GetVersionsRequest]) (*connect.Response[filesv1.GetVersionsResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.authz.CanViewFile(ctx, sub, req.Msg.GetMediaId()); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	versionsDB, ok := s.db.(versionStore)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("version storage is unavailable"))
	}
	limit := 20
	offset := 0
	if cursor := req.Msg.GetCursor(); cursor != nil {
		if cursor.GetLimit() > 0 {
			limit = int(cursor.GetLimit())
		}
		if cursor.GetPage() != "" {
			offset, _ = strconv.Atoi(cursor.GetPage())
		}
	}
	if limit > 100 {
		limit = 100
	}
	versions, _, err := versionsDB.GetVersionsPaginated(ctx, req.Msg.GetMediaId(), limit, offset)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	respVersions := make([]*filesv1.FileVersion, 0, len(versions))
	latest := int64(0)
	for _, v := range versions {
		if int64(v.VersionNumber()) > latest {
			latest = int64(v.VersionNumber())
		}
		respVersions = append(respVersions, &filesv1.FileVersion{
			Version:        int64(v.VersionNumber()),
			MediaId:        v.MediaID(),
			CreatedAt:      timestamppb.New(v.CreatedAt()),
			CreatedBy:      v.CreatedBy(),
			SizeBytes:      v.FileSize(),
			ChecksumSha256: v.ContentHash(),
		})
	}
	var nextCursor *commonv1.PageCursor
	if len(versions) == limit {
		nextCursor = &commonv1.PageCursor{Limit: int32(limit), Page: strconv.Itoa(offset + limit)}
	}
	return connect.NewResponse(&filesv1.GetVersionsResponse{
		Versions:      respVersions,
		LatestVersion: latest,
		NextCursor:    nextCursor,
	}), nil
}

func (s *FileServer) RestoreVersion(ctx context.Context, req *connect.Request[filesv1.RestoreVersionRequest]) (*connect.Response[filesv1.RestoreVersionResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.authz.CanEditFile(ctx, sub, req.Msg.GetMediaId()); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	if req.Msg.GetVersion() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("version must be greater than zero"))
	}
	versionsDB, ok := s.db.(versionStore)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("version storage is unavailable"))
	}
	metadata, err := versionsDB.RestoreMediaToVersion(ctx, req.Msg.GetMediaId(), int(req.Msg.GetVersion()), sub)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&filesv1.RestoreVersionResponse{Metadata: toMediaMetadata(metadata)}), nil
}

func (s *FileServer) SetRetentionPolicy(ctx context.Context, req *connect.Request[filesv1.SetRetentionPolicyRequest]) (*connect.Response[filesv1.SetRetentionPolicyResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.authz.CanEditFile(ctx, sub, req.Msg.GetMediaId()); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	if strings.TrimSpace(req.Msg.GetPolicyId()) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("policy_id is required"))
	}
	retStore, ok := s.db.(retentionStore)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("retention store is unavailable"))
	}
	policy, err := retStore.GetPolicy(ctx, req.Msg.GetPolicyId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !policy.IsSystem() && policy.OwnerID() != sub {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("policy is not owned by caller"))
	}
	var expiresAt *time.Time
	if policy.RetentionDays() >= 0 {
		exp := time.Now().UTC().AddDate(0, 0, policy.RetentionDays())
		expiresAt = &exp
	}
	_ = retStore.RemoveRetention(ctx, req.Msg.GetMediaId())
	if err = retStore.ApplyRetention(ctx, fileRetentionRequest{
		mediaID:   req.Msg.GetMediaId(),
		policyID:  policy.ID(),
		expiresAt: expiresAt,
		isLocked:  false,
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&filesv1.SetRetentionPolicyResponse{Success: true}), nil
}

func (s *FileServer) GetRetentionPolicy(ctx context.Context, req *connect.Request[filesv1.GetRetentionPolicyRequest]) (*connect.Response[filesv1.GetRetentionPolicyResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	if err = s.authz.CanViewFile(ctx, sub, req.Msg.GetMediaId()); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	retStore, ok := s.db.(retentionStore)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("retention store is unavailable"))
	}
	retention, err := retStore.GetRetention(ctx, req.Msg.GetMediaId())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("retention policy not found"))
	}
	policy, err := retStore.GetPolicy(ctx, retention.PolicyID())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	mode := filesv1.RetentionPolicy_MODE_DELETE
	if policy.RetentionDays() < 0 {
		mode = filesv1.RetentionPolicy_MODE_ARCHIVE
	}
	var expires *timestamppb.Timestamp
	if retention.ExpiresAt() != nil {
		expires = timestamppb.New(*retention.ExpiresAt())
	}
	return connect.NewResponse(&filesv1.GetRetentionPolicyResponse{
		Policy: &filesv1.RetentionPolicy{
			PolicyId:      policy.ID(),
			Name:          policy.Name(),
			RetentionDays: int64(policy.RetentionDays()),
			Mode:          mode,
		},
		ExpiresAt: expires,
	}), nil
}

func (s *FileServer) ListRetentionPolicies(ctx context.Context, _ *connect.Request[filesv1.ListRetentionPoliciesRequest]) (*connect.Response[filesv1.ListRetentionPoliciesResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	retStore, ok := s.db.(retentionStore)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("retention store is unavailable"))
	}
	policies, _, err := retStore.ListPolicies(ctx, sub, 200, 0)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	items := make([]*filesv1.RetentionPolicy, 0, len(policies))
	for _, policy := range policies {
		mode := filesv1.RetentionPolicy_MODE_DELETE
		if policy.RetentionDays() < 0 {
			mode = filesv1.RetentionPolicy_MODE_ARCHIVE
		}
		items = append(items, &filesv1.RetentionPolicy{
			PolicyId:      policy.ID(),
			Name:          policy.Name(),
			RetentionDays: int64(policy.RetentionDays()),
			Mode:          mode,
		})
	}
	return connect.NewResponse(&filesv1.ListRetentionPoliciesResponse{Policies: items}), nil
}

// SearchMedia searches for media files matching specified criteria
func (s *FileServer) SearchMedia(ctx context.Context, req *connect.Request[filesv1.SearchMediaRequest]) (*connect.Response[filesv1.SearchMediaResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}

	ownerID := sub
	if req.Msg.GetOwnerId() != "" && req.Msg.GetOwnerId() != sub {
		return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("owner_id must match authenticated user"))
	}

	var limit int32 = 20
	var afterCursor string
	if cursor := req.Msg.GetCursor(); cursor != nil {
		if cursor.GetLimit() > 0 {
			limit = cursor.GetLimit()
		}
		afterCursor = cursor.GetPage()
	}
	if limit <= 0 || limit > 100 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("limit must be between 1 and 100"))
	}

	offset, err := decodeSearchCursor(afterCursor, int(limit))
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
	var nextCursor *commonv1.PageCursor
	if hasMore {
		nextCursor = &commonv1.PageCursor{
			Limit: limit,
			Page:  encodeSearchCursor(offset + int(limit)),
		}
	}

	return connect.NewResponse(&filesv1.SearchMediaResponse{
		Results:    results,
		NextCursor: nextCursor,
	}), nil
}

func resolveURLExpiry(expiresSeconds int64) (time.Time, error) {
	if expiresSeconds <= 0 {
		expiresSeconds = 300
	}
	if expiresSeconds > 86400 {
		return time.Time{}, fmt.Errorf("expires_seconds must be <= 86400")
	}
	return time.Now().UTC().Add(time.Duration(expiresSeconds) * time.Second), nil
}

func (s *FileServer) signedFileURL(_ context.Context, purpose, mediaID, sub string, expiresAt time.Time) (string, error) {
	cfg := s.Service.Config().(*config.FilesConfig)
	base := strings.TrimSpace(cfg.FileAccessServerUrl)
	if base == "" {
		base = "https://" + cfg.ServerName
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if baseURL.Scheme == "" {
		baseURL.Scheme = "https"
	}
	baseURL.Path = path.Join(baseURL.Path, "/signed", purpose)

	secret := strings.TrimSpace(cfg.CsrfSecret)
	if secret == "" {
		secret = strings.TrimSpace(cfg.EnvStorageEncryptionPhrase)
	}
	if secret == "" {
		return "", fmt.Errorf("signed URL secret is not configured")
	}

	expUnix := expiresAt.Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%s|%s|%s|%d", purpose, mediaID, sub, expUnix)
	sig := hex.EncodeToString(mac.Sum(nil))

	q := baseURL.Query()
	q.Set("media_id", mediaID)
	q.Set("sub", sub)
	q.Set("exp", strconv.FormatInt(expUnix, 10))
	q.Set("sig", sig)
	baseURL.RawQuery = q.Encode()
	return baseURL.String(), nil
}

func (s *FileServer) getOwnedUpload(ctx context.Context, uploadID, subject string) (multipartStore, interface {
	ID() string
	OwnerID() string
	MediaID() string
	UploadName() string
	ContentType() string
	TotalSize() int64
	PartSize() int64
	PartCount() int
	UploadedParts() int
	UploadState() string
	ExpiresAt() *time.Time
}, error) {
	store, ok := s.db.(multipartStore)
	if !ok {
		return nil, nil, connect.NewError(connect.CodeInternal, fmt.Errorf("multipart storage is unavailable"))
	}
	upload, err := store.GetUpload(ctx, uploadID)
	if err != nil {
		return nil, nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("upload not found"))
	}
	if upload.OwnerID() != subject {
		return nil, nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("upload does not belong to caller"))
	}
	if expiresAt := upload.ExpiresAt(); expiresAt != nil && !expiresAt.After(time.Now().UTC()) {
		return nil, nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("upload has expired"))
	}
	return store, upload, nil
}

func (s *FileServer) GetMultipartUpload(ctx context.Context, req *connect.Request[filesv1.GetMultipartUploadRequest]) (*connect.Response[filesv1.GetMultipartUploadResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	store, upload, err := s.getOwnedUpload(ctx, req.Msg.GetUploadId(), sub)
	if err != nil {
		return nil, err
	}
	var uploadedSize int64
	parts, partsErr := store.GetParts(ctx, upload.ID())
	if partsErr == nil {
		for _, p := range parts {
			uploadedSize += p.Size()
		}
	}
	return connect.NewResponse(&filesv1.GetMultipartUploadResponse{
		MediaId:       upload.MediaID(),
		Filename:      upload.UploadName(),
		TotalSize:     upload.TotalSize(),
		UploadedSize:  uploadedSize,
		PartsUploaded: int32(upload.UploadedParts()),
		UploadState:   uploadStateToProto(upload.UploadState()),
	}), nil
}

func (s *FileServer) PatchContent(ctx context.Context, req *connect.Request[filesv1.PatchContentRequest]) (*connect.Response[filesv1.PatchContentResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	mediaID := req.Msg.GetMediaId()
	if !isValidMediaID(mediaID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}
	if err = s.authz.CanEditFile(ctx, sub, mediaID); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	pStore, ok := s.db.(patchStore)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("patch operation unavailable"))
	}

	updates := make(map[string]any)
	if filename := req.Msg.GetFilename(); filename != "" {
		updates["name"] = filename
	}
	if vis := req.Msg.GetVisibility(); vis != filesv1.MediaMetadata_VISIBILITY_UNSPECIFIED {
		updates["public"] = vis == filesv1.MediaMetadata_VISIBILITY_PUBLIC
	}

	updated, err := pStore.UpdateMediaMetadata(ctx, types.MediaID(mediaID), updates)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&filesv1.PatchContentResponse{
		Metadata: toMediaMetadata(updated),
	}), nil
}

func (s *FileServer) FinalizeSignedUpload(ctx context.Context, req *connect.Request[filesv1.FinalizeSignedUploadRequest]) (*connect.Response[filesv1.FinalizeSignedUploadResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	mediaID := req.Msg.GetMediaId()
	if !isValidMediaID(mediaID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}
	if err = s.authz.CanEditFile(ctx, sub, mediaID); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	metadata, err := s.db.GetMediaMetadata(ctx, types.MediaID(mediaID))
	if err != nil || metadata == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("media not found"))
	}

	pStore, ok := s.db.(patchStore)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("patch operation unavailable"))
	}

	updates := make(map[string]any)
	if checksum := req.Msg.GetChecksumSha256(); checksum != "" {
		updates["hash"] = checksum
	}
	if sizeBytes := req.Msg.GetSizeBytes(); sizeBytes > 0 {
		updates["size"] = sizeBytes
	}
	updated, err := pStore.UpdateMediaMetadata(ctx, types.MediaID(mediaID), updates)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&filesv1.FinalizeSignedUploadResponse{
		Metadata: toMediaMetadata(updated),
	}), nil
}

func (s *FileServer) GrantAccess(ctx context.Context, req *connect.Request[filesv1.GrantAccessRequest]) (*connect.Response[filesv1.GrantAccessResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	mediaID := req.Msg.GetMediaId()
	if !isValidMediaID(mediaID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}
	grant := req.Msg.GetGrant()
	if grant == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("grant is required"))
	}
	role := accessRoleToString(grant.GetRole())
	if role == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid access role"))
	}
	if err = s.authz.GrantFileAccess(ctx, sub, mediaID, grant.GetPrincipalId(), role); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	return connect.NewResponse(&filesv1.GrantAccessResponse{Success: true}), nil
}

func (s *FileServer) RevokeAccess(ctx context.Context, req *connect.Request[filesv1.RevokeAccessRequest]) (*connect.Response[filesv1.RevokeAccessResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	mediaID := req.Msg.GetMediaId()
	if !isValidMediaID(mediaID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}
	if err = s.authz.RevokeFileAccess(ctx, sub, mediaID, req.Msg.GetPrincipalId()); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	return connect.NewResponse(&filesv1.RevokeAccessResponse{Success: true}), nil
}

func (s *FileServer) ListAccess(ctx context.Context, req *connect.Request[filesv1.ListAccessRequest]) (*connect.Response[filesv1.ListAccessResponse], error) {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return nil, err
	}
	mediaID := req.Msg.GetMediaId()
	if !isValidMediaID(mediaID) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}
	if err = s.authz.CanViewFile(ctx, sub, mediaID); err != nil {
		return nil, connect.NewError(connect.CodePermissionDenied, err)
	}
	grants, err := s.authz.ListFileAccessGrants(ctx, sub, mediaID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	limit := 20
	offset := 0
	if cursor := req.Msg.GetCursor(); cursor != nil {
		if cursor.GetLimit() > 0 {
			limit = int(cursor.GetLimit())
		}
		if cursor.GetPage() != "" {
			offset, _ = strconv.Atoi(cursor.GetPage())
		}
	}
	if limit > 100 {
		limit = 100
	}

	end := offset + limit
	if end > len(grants) {
		end = len(grants)
	}
	var pageGrants []*filesv1.AccessGrant
	if offset < len(grants) {
		for _, g := range grants[offset:end] {
			ag := &filesv1.AccessGrant{}
			ag.SetPrincipalId(g.PrincipalID)
			ag.SetRole(stringToAccessRole(g.Role))
			ag.SetPrincipalType(filesv1.PrincipalType_PRINCIPAL_TYPE_USER)
			pageGrants = append(pageGrants, ag)
		}
	}

	var nextCursor *commonv1.PageCursor
	if end < len(grants) {
		nextCursor = &commonv1.PageCursor{Limit: int32(limit), Page: strconv.Itoa(end)}
	}

	return connect.NewResponse(&filesv1.ListAccessResponse{
		Grants:     pageGrants,
		NextCursor: nextCursor,
	}), nil
}

func (s *FileServer) DownloadContent(ctx context.Context, req *connect.Request[filesv1.DownloadContentRequest], stream *connect.ServerStream[filesv1.DownloadContentResponse]) error {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return err
	}
	mediaID := req.Msg.GetMediaId()
	if !isValidMediaID(mediaID) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}
	if err = s.authz.CanViewFile(ctx, sub, mediaID); err != nil {
		return connect.NewError(connect.CodePermissionDenied, err)
	}

	cfg := s.Service.Config().(*config.FilesConfig)
	result, err := s.mediaService.DownloadFile(ctx, &business.DownloadRequest{
		MediaID: types.MediaID(mediaID),
		Config:  cfg,
	})
	if err != nil {
		return connect.NewError(mapBusinessErrorToConnectCode(err), err)
	}
	defer util.CloseAndLogOnError(ctx, result.FileData)

	buf := make([]byte, contentReadBufferSize)
	for {
		n, readErr := result.FileData.Read(buf)
		if n > 0 {
			if sendErr := stream.Send(&filesv1.DownloadContentResponse{Data: buf[:n]}); sendErr != nil {
				return connect.NewError(connect.CodeInternal, sendErr)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return connect.NewError(connect.CodeInternal, readErr)
		}
	}
}

func (s *FileServer) DownloadContentRange(ctx context.Context, req *connect.Request[filesv1.DownloadContentRangeRequest], stream *connect.ServerStream[filesv1.DownloadContentRangeResponse]) error {
	sub, err := authenticatedSubject(ctx)
	if err != nil {
		return err
	}
	mediaID := req.Msg.GetMediaId()
	if !isValidMediaID(mediaID) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid media id"))
	}

	start := req.Msg.GetStart()
	end := req.Msg.GetEnd()
	if start < 0 {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("start must be >= 0"))
	}
	if end > 0 && end < start {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("end must be >= start"))
	}

	if err = s.authz.CanViewFile(ctx, sub, mediaID); err != nil {
		return connect.NewError(connect.CodePermissionDenied, err)
	}

	cfg := s.Service.Config().(*config.FilesConfig)
	result, err := s.mediaService.DownloadFile(ctx, &business.DownloadRequest{
		MediaID: types.MediaID(mediaID),
		Config:  cfg,
	})
	if err != nil {
		return connect.NewError(mapBusinessErrorToConnectCode(err), err)
	}
	defer util.CloseAndLogOnError(ctx, result.FileData)

	if start > 0 {
		if _, err = io.CopyN(io.Discard, result.FileData, start); err != nil {
			if errors.Is(err, io.EOF) {
				return connect.NewError(connect.CodeOutOfRange, fmt.Errorf("start offset beyond file size"))
			}
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	var remaining int64
	if end > 0 {
		remaining = end - start
	} else {
		remaining = math.MaxInt64
	}

	buf := make([]byte, contentReadBufferSize)
	for remaining > 0 {
		toRead := int64(len(buf))
		if toRead > remaining {
			toRead = remaining
		}
		n, readErr := result.FileData.Read(buf[:toRead])
		if n > 0 {
			remaining -= int64(n)
			if sendErr := stream.Send(&filesv1.DownloadContentRangeResponse{Data: buf[:n]}); sendErr != nil {
				return connect.NewError(connect.CodeInternal, sendErr)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return connect.NewError(connect.CodeInternal, readErr)
		}
	}
	return nil
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
		Visibility:     visibility,
	}
}

func accessRoleToString(role filesv1.AccessRole) string {
	switch role {
	case filesv1.AccessRole_ACCESS_ROLE_READER:
		return "viewer"
	case filesv1.AccessRole_ACCESS_ROLE_WRITER:
		return "editor"
	case filesv1.AccessRole_ACCESS_ROLE_OWNER:
		return "owner"
	default:
		return ""
	}
}

func stringToAccessRole(role string) filesv1.AccessRole {
	switch role {
	case "viewer":
		return filesv1.AccessRole_ACCESS_ROLE_READER
	case "editor":
		return filesv1.AccessRole_ACCESS_ROLE_WRITER
	case "owner":
		return filesv1.AccessRole_ACCESS_ROLE_OWNER
	default:
		return filesv1.AccessRole_ACCESS_ROLE_UNSPECIFIED
	}
}

func uploadStateToProto(state string) filesv1.MultipartUploadState {
	switch state {
	case "pending":
		return filesv1.MultipartUploadState_MULTIPART_UPLOAD_STATE_UPLOADING
	case "completed":
		return filesv1.MultipartUploadState_MULTIPART_UPLOAD_STATE_COMPLETED
	case "aborted":
		return filesv1.MultipartUploadState_MULTIPART_UPLOAD_STATE_ABORTED
	default:
		return filesv1.MultipartUploadState_MULTIPART_UPLOAD_STATE_UNSPECIFIED
	}
}

func protoCloneMediaMetadata(in *filesv1.MediaMetadata) *filesv1.MediaMetadata {
	if in == nil {
		return nil
	}
	return proto.Clone(in).(*filesv1.MediaMetadata)
}
