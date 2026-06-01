package tool

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRAGTool_Info(t *testing.T) {
	rt := NewRAGTool(func(ctx context.Context, query string) (string, error) {
		return "", nil
	})
	info, err := rt.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "retrieve_knowledge" {
		t.Errorf("name = %q", info.Name)
	}
}

func TestRAGTool_Success(t *testing.T) {
	rt := NewRAGTool(func(ctx context.Context, query string) (string, error) {
		return "answer for: " + query, nil
	})
	result, err := rt.InvokableRun(context.Background(), `{"query": "what is Go?"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(result, "what is Go?") {
		t.Errorf("result = %q", result)
	}
}

func TestRAGTool_EmptyQuery(t *testing.T) {
	rt := NewRAGTool(func(ctx context.Context, query string) (string, error) {
		return "", nil
	})
	_, err := rt.InvokableRun(context.Background(), `{"query": ""}`)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestRAGTool_MissingQuery(t *testing.T) {
	rt := NewRAGTool(func(ctx context.Context, query string) (string, error) {
		return "", nil
	})
	_, err := rt.InvokableRun(context.Background(), `{}`)
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestRAGTool_InvalidJSON(t *testing.T) {
	rt := NewRAGTool(func(ctx context.Context, query string) (string, error) {
		return "", nil
	})
	_, err := rt.InvokableRun(context.Background(), `not json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRAGTool_RagFnError(t *testing.T) {
	rt := NewRAGTool(func(ctx context.Context, query string) (string, error) {
		return "", errors.New("rag failed")
	})
	_, err := rt.InvokableRun(context.Background(), `{"query": "test"}`)
	if err == nil {
		t.Fatal("expected error from rag fn")
	}
	if !strings.Contains(err.Error(), "rag failed") {
		t.Errorf("unexpected error: %v", err)
	}
}
