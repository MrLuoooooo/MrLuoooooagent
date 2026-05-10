package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/goagentpro/internal/model"
	"github.com/yourusername/goagentpro/internal/service"
	"go.uber.org/zap"
)

// DocumentHandler handles document management via the DocumentService.
type DocumentHandler struct {
	svc    *service.DocumentService
	logger *zap.Logger
}

// NewDocumentHandler creates a DocumentHandler.
func NewDocumentHandler(svc *service.DocumentService, logger *zap.Logger) *DocumentHandler {
	return &DocumentHandler{svc: svc, logger: logger}
}

// UploadDocument handles POST /api/v1/documents.
func (h *DocumentHandler) UploadDocument(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Err(400, "file is required"))
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, "read file failed"))
		return
	}
	if len(data) == 0 {
		c.JSON(http.StatusBadRequest, model.Err(400, "empty file"))
		return
	}

	ids, err := h.svc.Ingest(c.Request.Context(), data)
	if err != nil {
		h.logger.Error("ingest failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.OK(model.UploadDocumentResponse{
		DocumentIDs: ids, ChunkCount: len(ids), Status: "processed",
	}))
}

// DeleteDocument handles DELETE /api/v1/documents/:document_id.
func (h *DocumentHandler) DeleteDocument(c *gin.Context) {
	docID := c.Param("document_id")
	c.JSON(http.StatusOK, model.OK(model.DeleteDocumentResponse{
		DocumentID: docID, Status: "deleted",
	}))
}

// ListDocuments handles GET /api/v1/documents.
func (h *DocumentHandler) ListDocuments(c *gin.Context) {
	c.JSON(http.StatusOK, model.OK(model.ListDocumentsResponse{
		Total: 0, Documents: []model.DocumentItem{},
	}))
}
