package handlers

import (
	"context"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	"buf.build/gen/go/antinvestor/ocr/connectrpc/go/v2/ocr/v1/ocrv1connect"
	ocrv1 "buf.build/gen/go/antinvestor/ocr/protocolbuffers/go/ocr/v1"
	"connectrpc.com/connect/v2"
	"github.com/antinvestor/service-files/apps/ocr/service/business"
)

type OCRServer struct {
	ocrv1connect.UnimplementedOCRServiceHandler
	ocrBusiness business.OCRBusiness
}

func NewOCRServer(ocrBusiness business.OCRBusiness) *OCRServer {
	return &OCRServer{
		ocrBusiness: ocrBusiness,
	}
}

func (os *OCRServer) Recognize(ctx context.Context, request *ocrv1.RecognizeRequest) (*ocrv1.RecognizeResponse, error) { //nolint:misspell
	response, err := os.ocrBusiness.Recognise(ctx, request)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return response, nil
}

func (os *OCRServer) Status(ctx context.Context, request *commonv1.StatusRequest) (*ocrv1.StatusResponse, error) {
	response, err := os.ocrBusiness.CheckProgress(ctx, request)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err.Error()).WithCause(err)
	}
	return response, nil
}
