package handler

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

// DocumentHandler 管文档的上传、列表和删除。
type DocumentHandler struct {
	svc    *service.DocumentService
	cfg    *config.Config
	logger *zap.Logger
}

// NewDocumentHandler —
func NewDocumentHandler(svc *service.DocumentService, cfg *config.Config, logger *zap.Logger) *DocumentHandler {
	return &DocumentHandler{svc: svc, cfg: cfg, logger: logger}
}

// UploadDocument 接收文件上传。
// @Summary      上传文档
// @Description  上传文档文件（.pdf/.txt/.md/.docx/.xlsx/.pptx），自动提取文字、分块、向量化并存入知识库。最大 10MB。
// @Tags         文档
// @Accept       multipart/form-data
// @Produce      json
// @Param        file formData file true "上传文件（支持 .pdf / .docx / .xlsx / .pptx / .txt / .md）"
// @Success      200 {object} model.APIEnvelope{data=model.UploadDocumentResponse}
// @Failure      400 {object} model.APIEnvelope
// @Failure      500 {object} model.APIEnvelope
// @Router       /documents [post]
func (h *DocumentHandler) UploadDocument(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Err(400, "file is required"))
		return
	}
	defer file.Close()

	// Validate file extension
	filename := header.Filename
	ext := strings.ToLower(filepath.Ext(filename))
	if !h.isAllowedType(ext) {
		c.JSON(http.StatusBadRequest, model.Err(400,
			"不支持的文件类型: "+ext+"。支持: "+strings.Join(h.cfg.Document.AllowedTypes, ", ")))
		return
	}

	// Validate file size
	if h.cfg.Document.MaxFileSize > 0 && header.Size > h.cfg.Document.MaxFileSize {
		c.JSON(http.StatusBadRequest, model.Err(400,
			"文件过大（最大 "+formatSize(h.cfg.Document.MaxFileSize)+"）"))
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, "read file failed"))
		return
	}
	if len(data) == 0 {
		c.JSON(http.StatusBadRequest, model.Err(400, "empty file"))
		return
	}

	ids, err := h.svc.Ingest(c.Request.Context(), data, filename)
	if err != nil {
		h.logger.Error("ingest failed", zap.String("file", filename), zap.Error(err))
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.OK(model.UploadDocumentResponse{
		DocumentIDs: ids, ChunkCount: len(ids), Status: "processed",
	}))
}

func (h *DocumentHandler) isAllowedType(ext string) bool {
	// Always allow if no restrictions configured
	if len(h.cfg.Document.AllowedTypes) == 0 {
		return true
	}
	for _, t := range h.cfg.Document.AllowedTypes {
		if strings.EqualFold(t, ext) {
			return true
		}
	}
	return false
}

func formatSize(size int64) string {
	if size >= 1024*1024*1024 {
		return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
	}
	if size >= 1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	if size >= 1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%d B", size)
}

// DeleteDocument 按 ID 删文档。
// @Summary      删除文档
// @Description  按文档 ID 删除已上传的文档及其向量索引。
// @Tags         文档
// @Produce      json
// @Param        document_id path string true "文档 ID"
// @Success      200 {object} model.APIEnvelope{data=model.DeleteDocumentResponse}
// @Failure      500 {object} model.APIEnvelope
// @Router       /documents/{document_id} [delete]
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
// @Summary      列出文档
// @Description  获取所有已上传的文档列表。
// @Tags         文档
// @Produce      json
// @Success      200 {object} model.APIEnvelope{data=model.ListDocumentsResponse}
// @Failure      500 {object} model.APIEnvelope
// @Router       /documents [get]
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
