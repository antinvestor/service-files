package routing

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/authz"
	"github.com/antinvestor/service-files/apps/default/service/business"
	"github.com/antinvestor/service-files/apps/default/service/storage/connection"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/frame/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UploadRoutingTestSuite struct {
	tests.BaseTestSuite
}

func TestUploadRoutingTestSuite(t *testing.T) {
	suite.Run(t, new(UploadRoutingTestSuite))
}

func (suite *UploadRoutingTestSuite) TestUploadRequestValidate() {
	testCases := []struct {
		name     string
		size     types.FileSizeBytes
		filename string
		maxSize  config.FileSizeBytes
		wantCode int
	}{
		{
			name:     "too_large",
			size:     11,
			filename: "a.txt",
			maxSize:  10,
			wantCode: http.StatusRequestEntityTooLarge,
		},
		{
			name:     "filename_with_path_separator",
			size:     10,
			filename: "a/b.txt",
			maxSize:  100,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "valid_request",
			size:     10,
			filename: "b.txt",
			maxSize:  100,
			wantCode: 0,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			req := &uploadRequest{
				MediaMetadata: &types.MediaMetadata{
					FileSizeBytes: tc.size,
					UploadName:    types.Filename(tc.filename),
				},
			}

			res := req.Validate(tc.maxSize)
			if tc.wantCode == 0 {
				require.Nil(t, res)
				return
			}

			require.NotNil(t, res)
			assert.Equal(t, tc.wantCode, res.Code)
		})
	}
}

func (suite *UploadRoutingTestSuite) TestUploadRequestClose() {
	testCases := []struct {
		name    string
		req     *uploadRequest
		wantErr bool
	}{
		{
			name:    "nil_request",
			req:     nil,
			wantErr: false,
		},
		{
			name:    "no_close_function",
			req:     &uploadRequest{},
			wantErr: false,
		},
		{
			name: "close_function_called",
			req: &uploadRequest{
				closeFn: func() error { return nil },
			},
			wantErr: false,
		},
		{
			name: "close_function_error",
			req: &uploadRequest{
				closeFn: func() error { return io.EOF },
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			var err error
			if tc.req == nil {
				err = (*uploadRequest)(nil).Close()
			} else {
				err = tc.req.Close()
			}
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func (suite *UploadRoutingTestSuite) TestParseAndValidateRequest() {
	testCases := []struct {
		name          string
		contentLength int64
		contentType   string
		filename      string
		maxSize       config.FileSizeBytes
		wantErr       bool
	}{
		{
			name:          "valid_request",
			contentLength: 10,
			contentType:   "text/plain",
			filename:      "ok.txt",
			maxSize:       1024,
			wantErr:       false,
		},
		{
			name:          "too_large_payload",
			contentLength: 2048,
			contentType:   "text/plain",
			filename:      "ok.txt",
			maxSize:       10,
			wantErr:       true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "/upload", strings.NewReader("payload"))
			require.NoError(t, err)
			req.ContentLength = tc.contentLength
			req.Header.Set("Content-Type", tc.contentType)

			values := url.Values{}
			values.Set("filename", tc.filename)
			req.URL.RawQuery = values.Encode()

			result, resErr := parseAndValidateRequest(req, &config.FilesConfig{MaxFileSizeBytes: tc.maxSize}, "owner")
			if tc.wantErr {
				require.Nil(t, result)
				require.NotNil(t, resErr)
				return
			}

			require.NotNil(t, result)
			require.Nil(t, resErr)
			assert.Equal(t, types.ContentType(tc.contentType), result.MediaMetadata.ContentType)
			assert.Equal(t, types.OwnerID("owner"), result.MediaMetadata.OwnerID)
		})
	}
}

func (suite *UploadRoutingTestSuite) TestParseAndValidateMultipartRequest() {
	testCases := []struct {
		name                   string
		queryFilename          string
		formFilenameField      string
		filePartName           string
		filePartFilename       string
		filePartContentType    string
		includeSecondFilePart  bool
		expectErr              bool
		expectedUploadName     string
		expectedPartMimeType   string
		expectedResponseStatus int
	}{
		{
			name:                 "valid_multipart_uses_file_part",
			filePartName:         "file",
			filePartFilename:     "doc.txt",
			filePartContentType:  "text/plain",
			expectErr:            false,
			expectedUploadName:   "doc.txt",
			expectedPartMimeType: "text/plain",
		},
		{
			name:                 "query_filename_takes_precedence",
			queryFilename:        "from-query.txt",
			filePartName:         "file",
			filePartFilename:     "from-part.txt",
			filePartContentType:  "application/octet-stream",
			expectErr:            false,
			expectedUploadName:   "from-query.txt",
			expectedPartMimeType: "application/octet-stream",
		},
		{
			name:                 "filename_field_used_when_query_missing",
			formFilenameField:    "from-field.txt",
			filePartName:         "file",
			filePartFilename:     "from-part.txt",
			filePartContentType:  "application/json",
			expectErr:            false,
			expectedUploadName:   "from-field.txt",
			expectedPartMimeType: "application/json",
		},
		{
			name:                   "multiple_file_parts_rejected",
			filePartName:           "file",
			filePartFilename:       "a.txt",
			filePartContentType:    "text/plain",
			includeSecondFilePart:  true,
			expectErr:              true,
			expectedResponseStatus: http.StatusBadRequest,
		},
		{
			name:                   "missing_file_part_rejected",
			formFilenameField:      "name-only.txt",
			expectErr:              true,
			expectedResponseStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			if tc.formFilenameField != "" {
				require.NoError(t, writer.WriteField("filename", tc.formFilenameField))
			}

			if tc.filePartName != "" {
				partHeader := make(textproto.MIMEHeader)
				partHeader.Set("Content-Disposition", `form-data; name="`+tc.filePartName+`"; filename="`+tc.filePartFilename+`"`)
				if tc.filePartContentType != "" {
					partHeader.Set("Content-Type", tc.filePartContentType)
				}
				part, err := writer.CreatePart(partHeader)
				require.NoError(t, err)
				_, err = io.WriteString(part, "file-content")
				require.NoError(t, err)
			}

			if tc.includeSecondFilePart {
				part, err := writer.CreateFormFile("file2", "b.txt")
				require.NoError(t, err)
				_, err = io.WriteString(part, "another-content")
				require.NoError(t, err)
			}

			require.NoError(t, writer.Close())

			uploadURL := "/upload"
			if tc.queryFilename != "" {
				uploadURL += "?filename=" + url.QueryEscape(tc.queryFilename)
			}
			req := httptest.NewRequest(http.MethodPost, uploadURL, body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.ContentLength = int64(body.Len())

			result, resErr := parseAndValidateRequest(req, &config.FilesConfig{MaxFileSizeBytes: 1024 * 1024}, "owner")
			if tc.expectErr {
				require.Nil(t, result)
				require.NotNil(t, resErr)
				assert.Equal(t, tc.expectedResponseStatus, resErr.Code)
				return
			}

			require.NotNil(t, result)
			require.Nil(t, resErr)
			t.Cleanup(func() {
				_ = result.Close()
			})
			assert.Equal(t, types.OwnerID("owner"), result.MediaMetadata.OwnerID)
			assert.Equal(t, types.Filename(tc.expectedUploadName), result.MediaMetadata.UploadName)
			assert.Equal(t, types.ContentType(tc.expectedPartMimeType), result.MediaMetadata.ContentType)
			assert.NotNil(t, result.FileData)
		})
	}
}

func (suite *UploadRoutingTestSuite) TestParseAndValidateMultipartRequestInvalidBoundary() {
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("not-really-multipart"))
	req.Header.Set("Content-Type", "multipart/form-data")
	req.ContentLength = int64(len("not-really-multipart"))

	result, resErr := parseAndValidateRequest(req, &config.FilesConfig{MaxFileSizeBytes: 1024}, "owner")
	require.Nil(suite.T(), result)
	require.NotNil(suite.T(), resErr)
	assert.Equal(suite.T(), http.StatusBadRequest, resErr.Code)
}

func (suite *UploadRoutingTestSuite) TestUpload() {
	testCases := []struct {
		name           string
		subject        string
		filename       string
		body           string
		multipartBody  bool
		multipartField string
		multipartName  string
		multipartType  string
		wantCode       int
		wantErrKey     string
	}{
		{
			name:       "unauthenticated_rejected",
			filename:   "x.txt",
			body:       "content",
			wantCode:   http.StatusUnauthorized,
			wantErrKey: "error",
		},
		{
			name:       "authenticated_upload_handles_queue_failure",
			subject:    "@uploader:example.com",
			filename:   "x.txt",
			body:       "content",
			wantCode:   http.StatusInternalServerError,
			wantErrKey: "error",
		},
		{
			name:           "authenticated_multipart_upload_handles_queue_failure",
			subject:        "@uploader:example.com",
			filename:       "x.txt",
			body:           "multipart-content",
			multipartBody:  true,
			multipartField: "file",
			multipartName:  "x.txt",
			multipartType:  "text/plain",
			wantCode:       http.StatusInternalServerError,
			wantErrKey:     "error",
		},
		{
			name:       "invalid_filename_rejected",
			subject:    "@uploader:example.com",
			filename:   "a/b.txt",
			body:       "content",
			wantCode:   http.StatusBadRequest,
			wantErrKey: "error",
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, svc, res := suite.CreateService(t, dep)
				cfg := svc.Config().(*config.FilesConfig)

				db := &connection.Database{
					WorkManager:     svc.WorkManager(),
					MediaRepository: res.MediaRepository,
				}
				storageProvider, err := provider.GetStorageProvider(ctx, cfg)
				require.NoError(t, err)
				mediaService := business.NewMediaService(db, storageProvider)
				authzMiddleware := authz.NewMiddleware(svc.SecurityManager().GetAuthorizer(ctx), db)

				reqURL := "/v1/media/upload?filename=" + url.QueryEscape(tc.filename)
				req := httptest.NewRequest(http.MethodPost, reqURL, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "text/plain")
				req.ContentLength = int64(len(tc.body))

				if tc.multipartBody {
					mpBuffer := &bytes.Buffer{}
					mpWriter := multipart.NewWriter(mpBuffer)
					partHeader := make(textproto.MIMEHeader)
					partHeader.Set("Content-Disposition", `form-data; name="`+tc.multipartField+`"; filename="`+tc.multipartName+`"`)
					partHeader.Set("Content-Type", tc.multipartType)
					part, partErr := mpWriter.CreatePart(partHeader)
					require.NoError(t, partErr)
					_, partErr = io.WriteString(part, tc.body)
					require.NoError(t, partErr)
					require.NoError(t, mpWriter.Close())

					req = httptest.NewRequest(http.MethodPost, reqURL, mpBuffer)
					req.Header.Set("Content-Type", mpWriter.FormDataContentType())
					req.ContentLength = int64(mpBuffer.Len())
				}

				if tc.subject != "" {
					claims := &security.AuthenticationClaims{
						RegisteredClaims: jwt.RegisteredClaims{Subject: tc.subject},
					}
					req = req.WithContext(claims.ClaimsToContext(req.Context()))
				}

				resp := Upload(req, svc, db, storageProvider, mediaService, authzMiddleware)
				assert.Equal(t, tc.wantCode, resp.Code)
				assert.NotNil(t, resp.JSON)
				if tc.wantErrKey != "" {
					payload, ok := resp.JSON.(map[string]interface{})
					require.True(t, ok)
					assert.Contains(t, payload, tc.wantErrKey)
				}
			})
		}
	})
}
