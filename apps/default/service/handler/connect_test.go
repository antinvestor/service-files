package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"buf.build/gen/go/antinvestor/files/connectrpc/go/v2/files/v1/filesv1connect"
	filesv1 "buf.build/gen/go/antinvestor/files/protocolbuffers/go/files/v1"
	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/pitabwire/frame/v3"
	"github.com/pitabwire/frame/v3/frametests/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// connect v2 rejects any message over 4 MiB by default.
const connectDefaultReadMaxBytes = 4 << 20

func TestConnectLimits(t *testing.T) {
	assert.Greater(t, MaxConnectReadBytes, int(maxInMemoryFileBytes),
		"the read limit must admit a full upload chunk or multipart part plus its fields")
	assert.Greater(t, MaxConnectSendBytes, int(maxBatchResponseBytes),
		"the send limit must admit a full BatchGetContent response")
	assert.Greater(t, MaxConnectSendBytes, connectDefaultReadMaxBytes)
}

// Test_FileServer_ConnectUploadOverDefaultReadLimit sends uploads over HTTP
// through Mount, the same wiring cmd/main.go uses.
func (suite *FileServerTestSuite) Test_FileServer_ConnectUploadOverDefaultReadLimit() {
	const userID = "@large-upload:example.com"

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		ctx, cfg, _, fileHandler := suite.setupFileServer(t, dep)
		// UploadContent queues thumbnail generation once the file is stored.
		fileHandler.Service.Init(ctx, frame.WithRegisterPublisher(
			cfg.QueueThumbnailsGenerateName, cfg.QueueThumbnailsGenerateURL))

		mux := http.NewServeMux()
		Mount(mux, fileHandler)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mux.ServeHTTP(w, r.WithContext(claimsCtx(r.Context(), userID)))
		}))
		defer server.Close()

		client := filesv1connect.NewFilesServiceClient(
			connect.NewClient(connecthttp.NewTransport(server.Client(), server.URL)))

		t.Run("chunked_upload_larger_than_4MiB", func(t *testing.T) {
			payload := make([]byte, 6<<20)
			for i := range payload {
				payload[i] = byte(i % 251)
			}
			require.Greater(t, len(payload), connectDefaultReadMaxBytes)
			require.GreaterOrEqual(t, int64(cfg.MaxFileSizeBytes), int64(len(payload)))

			stream, err := client.UploadContent(ctx)
			require.NoError(t, err)
			require.NoError(t, stream.Send(&filesv1.UploadContentRequest{
				Data: &filesv1.UploadContentRequest_Metadata{Metadata: &filesv1.UploadMetadata{
					Filename:    "large.bin",
					ContentType: "application/octet-stream",
					TotalSize:   int64(len(payload)),
				}},
			}))
			const chunkSize = 1 << 20
			for off := 0; off < len(payload); off += chunkSize {
				end := min(off+chunkSize, len(payload))
				require.NoError(t, stream.Send(&filesv1.UploadContentRequest{
					Data: &filesv1.UploadContentRequest_Chunk{Chunk: payload[off:end]},
				}))
			}
			resp, err := stream.CloseAndReceive()
			require.NoError(t, err)
			require.NotEmpty(t, resp.GetMediaId())
			assert.Equal(t, int64(len(payload)), resp.GetMetadata().GetFileSizeBytes())

			got := downloadAll(ctx, t, client, resp.GetMediaId())
			assert.True(t, bytes.Equal(payload, got), "downloaded %d bytes, want %d identical bytes", len(got), len(payload))
		})

		t.Run("single_message_over_read_limit", func(t *testing.T) {
			stream, err := client.UploadContent(ctx)
			require.NoError(t, err)
			require.NoError(t, stream.Send(&filesv1.UploadContentRequest{
				Data: &filesv1.UploadContentRequest_Metadata{Metadata: &filesv1.UploadMetadata{
					Filename:    "oversized.bin",
					ContentType: "application/octet-stream",
				}},
			}))
			// The server may reset the stream before the client finishes
			// sending; the outcome is read from CloseAndReceive.
			_ = stream.Send(&filesv1.UploadContentRequest{
				Data: &filesv1.UploadContentRequest_Chunk{Chunk: make([]byte, MaxConnectReadBytes+1)},
			})
			_, err = stream.CloseAndReceive()
			require.Error(t, err)
			assert.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))
		})
	})
}

func downloadAll(ctx context.Context, t *testing.T, client filesv1connect.FilesServiceClient, mediaID string) []byte {
	t.Helper()
	stream, err := client.DownloadContent(ctx, &filesv1.DownloadContentRequest{MediaId: mediaID})
	require.NoError(t, err)
	defer func() { _ = stream.Close() }()

	var buf bytes.Buffer
	for {
		msg, recvErr := stream.Receive()
		if errors.Is(recvErr, io.EOF) {
			return buf.Bytes()
		}
		require.NoError(t, recvErr)
		buf.Write(msg.GetData())
	}
}
