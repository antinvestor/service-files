package queue

import (
	"context"
	"encoding/json"
	"github.com/antinvestor/service-files/service/models"
	"github.com/antinvestor/service-files/service/repository"
	"github.com/pitabwire/frame"
)

type FileQueueHandler struct {
	Service *frame.Service
	repo    repository.FileRepository
}

func (fq *FileQueueHandler) Handle(ctx context.Context, _ map[string]string, payload []byte) error {

	file := &models.File{}
	err := json.Unmarshal(payload, file)
	if err != nil {
		return err
	}

	return fq.repo.Save(ctx, file)

}

func NewFileQueueHandler(service *frame.Service) FileQueueHandler {
	fileRepo := repository.NewFileRepository(service)
	return FileQueueHandler{service, fileRepo}
}

type FileAuditQueueHandler struct {
	Service *frame.Service
	repo    repository.FileAuditRepository
}

func (faq *FileAuditQueueHandler) Handle(ctx context.Context, _ map[string]string, payload []byte) error {

	auditFile := &models.FileAudit{}
	err := json.Unmarshal(payload, auditFile)
	if err != nil {
		return err
	}

	return faq.repo.Save(ctx, auditFile)

}

func NewFileAuditQueueHandler(service *frame.Service) FileQueueHandler {
	fileRepo := repository.NewFileRepository(service)
	return FileQueueHandler{service, fileRepo}
}
