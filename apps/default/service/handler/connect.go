package handler

import (
	"buf.build/gen/go/antinvestor/files/connectrpc/go/v2/files/v1/filesv1connect"
	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
)

// Connect message limits for FilesService.
//
// connect v2 caps every message at 4 MiB unless told otherwise. The limits
// apply to each protobuf message, not to a whole stream, so they follow the
// largest single message this service accepts or returns:
//
//   - Requests: file bytes arrive as UploadContent chunks or UploadMultipartPart
//     bodies, each capped at maxInMemoryFileBytes (1.5 MiB). The size of a whole
//     upload is bounded separately by FilesConfig.MaxFileSizeBytes, summed over
//     the stream.
//   - Responses: BatchGetContent returns up to maxBatchResponseBytes (8 MiB) of
//     content plus per-item metadata. Downloads stream in contentReadBufferSize
//     (64 KiB) messages.
//
// Callers reading BatchGetContent with a connect v2 client must raise their own
// client read limit above 4 MiB.
const (
	// connectReadOverheadBytes covers protobuf framing and the fields sent
	// alongside a chunk or part (metadata, ids, part numbers).
	connectReadOverheadBytes = 256 << 10
	// connectSendOverheadBytes covers the per-item metadata of a full
	// BatchGetContent response.
	connectSendOverheadBytes = 1 << 20

	// MaxConnectReadBytes is the largest request message FilesService reads.
	MaxConnectReadBytes = int(maxInMemoryFileBytes) + connectReadOverheadBytes
	// MaxConnectSendBytes is the largest response message FilesService sends.
	MaxConnectSendBytes = int(maxBatchResponseBytes) + connectSendOverheadBytes
)

// ConnectOptions returns the connecthttp options FilesService is mounted with.
func ConnectOptions() []connecthttp.Option {
	return []connecthttp.Option{
		connecthttp.WithReadMaxBytes(MaxConnectReadBytes),
		connecthttp.WithSendMaxBytes(MaxConnectSendBytes),
	}
}

// Mount registers impl on a Connect server with the given interceptors and
// mounts it on mux with ConnectOptions.
func Mount(mux connecthttp.ServeMux, impl filesv1connect.FilesServiceHandler, interceptors ...connect.ServerInterceptor) {
	server := connect.NewServer(interceptors...)
	filesv1connect.RegisterFilesServiceHandler(server, impl)
	connecthttp.Mount(mux, server, ConnectOptions()...)
}
