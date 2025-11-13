package routing

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/antinvestor/service-files/apps/default/service/types"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type DownloadTestSuite struct {
	tests.BaseTestSuite
}

func TestDownloadTestSuite(t *testing.T) {
	suite.Run(t, new(DownloadTestSuite))
}

func (suite *DownloadTestSuite) Test_dispositionFor() {
	testCases := []struct {
		name        string
		contentType types.ContentType
		expected    string
		description string
	}{
		{
			name:        "empty content type",
			contentType: types.ContentType(""),
			expected:    "attachment",
			description: "empty content type",
		},
		{
			name:        "image/svg",
			contentType: types.ContentType("image/svg"),
			expected:    "attachment",
			description: "image/svg",
		},
		{
			name:        "image/jpeg",
			contentType: types.ContentType("image/jpeg"),
			expected:    "inline",
			description: "image/jpg",
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := contentDispositionFor(tc.contentType)
				assert.Equal(t, tc.expected, result, tc.description)
			})
		}
	})
}

func (suite *DownloadTestSuite) Test_Multipart() {
	testCases := []struct {
		name         string
		responseBody string
		contentType  string
		maxSize      config.FileSizeBytes
	}{
		{
			name:         "plain text multipart response",
			responseBody: "This media is plain text. Maybe somebody used it as a paste bin.",
			contentType:  "text/plain",
			maxSize:      config.FileSizeBytes(1000),
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx := t.Context()
				r := &downloadRequest{
					MediaMetadata: &types.MediaMetadata{},
				}
				data := bytes.Buffer{}
				data.WriteString(tc.responseBody)

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					_, err := multipartResponse(w, r, tc.contentType, &data)
					assert.NoError(t, err)
				}))
				defer srv.Close()

				resp, err := srv.Client().Get(srv.URL)
				assert.NoError(t, err)
				defer util.CloseAndLogOnError(ctx, resp.Body)

				// contentLength is always 0, since there's no Content-Length header on the multipart part.
				_, reader, err := parseMultipartResponse(ctx, r, resp, tc.maxSize)
				assert.NoError(t, err)
				gotResponse, err := io.ReadAll(reader)
				assert.NoError(t, err)
				assert.Equal(t, tc.responseBody, string(gotResponse))
			})
		}
	})
}
