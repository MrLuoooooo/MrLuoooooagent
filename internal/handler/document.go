package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

// DocumentHandler 管文档的上传、列表和删除。
type DocumentHandler struct {
	svc    *service.DocumentService
	logger *zap.Logger
}

// NewDocumentHandler —
func NewDocumentHandler(svc *service.DocumentService, logger *zap.Logger) *DocumentHandler {
	return &DocumentHandler{svc: svc, logger: logger}
}

// UploadDocument 接收文件上传。
func (h *DocumentHandler) UploadDocument(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
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

	filename := header.Filename

	ids, err := h.svc.Ingest(c.Request.Context(), data, filename)
	if err != nil {
		h.logger.Error("ingest failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.OK(model.UploadDocumentResponse{
		DocumentIDs: ids, ChunkCount: len(ids), Status: "processed",
	}))
}

// DeleteDocument 按 ID 删文档。
func (h *DocumentHandler) DeleteDocument(c *gin.Context) {
	docID := c.Param("document_id")
	if err := h.svc.DeleteDocument(c.Request.Context(), docID); err != nil {
		h.logger.Error("delete document", zap.String("id", docID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.Err(500, "删除文档失败"))
		return
	}
	c.JSON(http.StatusOK, model.OK(model.DeleteDocumentResponse{
		DocumentID: docID, Status: "deleted",
	}))
}

// ListDocuments 列全部文档。
func (h *DocumentHandler) ListDocuments(c *gin.Context) {
	docs, err := h.svc.ListDocuments(c.Request.Context())
	if err != nil {
		h.logger.Error("list documents", zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.Err(500, "获取文档列表失败"))
		return
	}

	items := make([]model.DocumentItem, len(docs))
	for i, d := range docs {
		items[i] = model.DocumentItem{
			DocumentID: d.ID,
			Content:    d.Content,
			ChunkCount: d.ChunkCount,
			CreatedAt:  d.CreatedAt,
		}
	}
	c.JSON(http.StatusOK, model.OK(model.ListDocumentsResponse{
		Total: len(items), Documents: items,
	}))
}
