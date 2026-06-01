package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"go.uber.org/zap"
)

type stubDocStore struct {
	docs map[string]service.DocMeta
}

func (s *stubDocStore) Save(_ context.Context, doc service.DocMeta) error {
	s.docs[doc.ID] = doc
	return nil
}

func (s *stubDocStore) Delete(_ context.Context, id string) error {
	delete(s.docs, id)
	return nil
}

func (s *stubDocStore) List(_ context.Context) ([]service.DocMeta, error) {
	var result []service.DocMeta
	for _, d := range s.docs {
		result = append(result, d)
	}
	return result, nil
}

type stubVectorDeleter struct{}

func (s *stubVectorDeleter) DeleteByDocumentID(_ context.Context, _ string) error {
	return nil
}

type stubIngestionChain struct{}

func (s *stubIngestionChain) Invoke(_ context.Context, data []byte, _ ...compose.Option) ([]string, error) {
	return []string{"doc-1", "chunk-1", "chunk-2"}, nil
}
func (s *stubIngestionChain) Stream(_ context.Context, _ []byte, _ ...compose.Option) (*schema.StreamReader[[]string], error) {
	return nil, nil
}
func (s *stubIngestionChain) Collect(_ context.Context, _ *schema.StreamReader[[]byte], _ ...compose.Option) ([]string, error) {
	return nil, nil
}
func (s *stubIngestionChain) Transform(_ context.Context, _ *schema.StreamReader[[]byte], _ ...compose.Option) (*schema.StreamReader[[]string], error) {
	return nil, nil
}

func newDocHandler() *DocumentHandler {
	svc := service.NewDocumentService(&stubIngestionChain{}, &stubDocStore{docs: make(map[string]service.DocMeta)}, &stubVectorDeleter{}, zap.NewNop())
	cfg := &config.Config{
		Document: config.DocumentConfig{
			MaxFileSize:  10485760,
			AllowedTypes: []string{".pdf", ".txt", ".md", ".docx", ".xlsx", ".pptx"},
		},
	}
	return NewDocumentHandler(svc, cfg, zap.NewNop())
}

func TestUploadDocument_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newDocHandler()
	r := gin.New()
	r.POST("/documents", h.UploadDocument)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "test.txt")
	fw.Write([]byte("hello world"))
	w.Close()

	req := httptest.NewRequest("POST", "/documents", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body=%s", resp.Code, resp.Body.String())
	}
	var env model.APIEnvelope
	if err := json.Unmarshal(resp.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Code != 0 {
		t.Fatalf("code = %d, msg = %s", env.Code, env.Message)
	}
}

func TestUploadDocument_MissingFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newDocHandler()
	r := gin.New()
	r.POST("/documents", h.UploadDocument)

	req := httptest.NewRequest("POST", "/documents", nil)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestUploadDocument_EmptyFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newDocHandler()
	r := gin.New()
	r.POST("/documents", h.UploadDocument)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "empty.txt")
	fw.Write([]byte{})
	w.Close()

	req := httptest.NewRequest("POST", "/documents", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	// Empty file check happens after reading, so it should return 400
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400. body=%s", resp.Code, resp.Body.String())
	}
}

func TestUploadDocument_WrongFormField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newDocHandler()
	r := gin.New()
	r.POST("/documents", h.UploadDocument)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("wrong_field", "test.txt")
	fw.Write([]byte("hello"))
	w.Close()

	req := httptest.NewRequest("POST", "/documents", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}
}

func TestDeleteDocument(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newDocHandler()
	r := gin.New()
	r.DELETE("/documents/:document_id", h.DeleteDocument)

	req := httptest.NewRequest("DELETE", "/documents/doc-1", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	var env model.APIEnvelope
	json.Unmarshal(resp.Body.Bytes(), &env)
	if env.Code != 0 {
		t.Errorf("code = %d", env.Code)
	}
}

func TestListDocuments_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newDocHandler()
	r := gin.New()
	r.GET("/documents", h.ListDocuments)

	req := httptest.NewRequest("GET", "/documents", nil)
	resp := httptest.NewRecorder()
	r.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	var env model.APIEnvelope
	json.Unmarshal(resp.Body.Bytes(), &env)
	if env.Code != 0 {
		t.Errorf("code = %d", env.Code)
	}
}
