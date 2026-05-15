package openaiembed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbedStrings_ReturnsVectors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("bad auth header: %s", r.Header.Get("Authorization"))
		}

		var req openAIEmbedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("model = %q, want test-model", req.Model)
		}
		if len(req.Input) != 2 {
			t.Errorf("input len = %d, want 2", len(req.Input))
		}

		resp := openAIEmbedResponse{
			Object: "list",
			Model:  "test-model",
			Data: []openAIEmbedData{
				{Object: "embedding", Index: 0, Embedding: []float64{0.1, 0.2, 0.3}},
				{Object: "embedding", Index: 1, Embedding: []float64{0.4, 0.5, 0.6}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	emb := NewOpenAIEmbedder("test-key", "test-model", srv.URL)
	vecs, err := emb.EmbedStrings(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("EmbedStrings: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("got %d vectors, want 2", len(vecs))
	}
	if len(vecs[0]) != 3 || vecs[0][0] != 0.1 {
		t.Errorf("vec[0] = %v, want [0.1 0.2 0.3]", vecs[0])
	}
	if len(vecs[1]) != 3 || vecs[1][0] != 0.4 {
		t.Errorf("vec[1] = %v, want [0.4 0.5 0.6]", vecs[1])
	}
}

func TestEmbedStrings_APICall(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Content-Type.
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("Authorization missing Bearer prefix")
		}

		capturedBody, _ = json.Marshal(json.RawMessage(`{"ok":true}`)) // ignored, we capture from body below
		// Actually capture the request body.
		var req openAIEmbedRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model != "embed-model" {
			t.Errorf("model = %q, want embed-model", req.Model)
		}

		resp := openAIEmbedResponse{
			Data: []openAIEmbedData{
				{Index: 0, Embedding: []float64{1.0, 2.0}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	emb := NewOpenAIEmbedder("api-key-123", "embed-model", srv.URL)
	_, err := emb.EmbedStrings(context.Background(), []string{"test"})
	if err != nil {
		t.Fatalf("EmbedStrings: %v", err)
	}
	_ = capturedBody // body was captured in handler
}

func TestEmbedStrings_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid api key"}`))
	}))
	defer srv.Close()

	emb := NewOpenAIEmbedder("bad-key", "model", srv.URL)
	_, err := emb.EmbedStrings(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
	if !strings.Contains(err.Error(), "OpenAI error") {
		t.Errorf("error should contain 'OpenAI error': %v", err)
	}
}

func TestEmbedStrings_EmptyInput(t *testing.T) {
	emb := NewOpenAIEmbedder("key", "model", "http://localhost:1")
	vecs, err := emb.EmbedStrings(context.Background(), nil)
	if err != nil {
		t.Fatalf("empty input should return empty, not error: %v", err)
	}
	if len(vecs) != 0 {
		t.Errorf("empty input should return 0 vectors, got %d", len(vecs))
	}
}
