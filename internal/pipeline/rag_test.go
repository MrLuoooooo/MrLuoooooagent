package pipeline

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// fakeRetriever implements retriever.Retriever for testing.
type fakeRetriever struct {
	docs []*schema.Document
	err  error
}

func (f *fakeRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	return f.docs, f.err
}

func TestNewRAGChain_Compiles(t *testing.T) {
	fr := &fakeRetriever{docs: []*schema.Document{
		{Content: "Go is an open source language."},
	}}
	tmpl := NewDefaultRAGTemplate()
	cm := &fakeCM{}

	chain, err := NewRAGChain(fr, tmpl, cm, nil, 5, 15)
	if err != nil {
		t.Fatalf("NewRAGChain() error = %v", err)
	}
	if chain == nil {
		t.Fatal("chain should not be nil")
	}
}

func TestNewRAGChain_Invoke(t *testing.T) {
	fr := &fakeRetriever{docs: []*schema.Document{
		{Content: "Go supports concurrency with goroutines."},
		{Content: "Go was created at Google in 2009."},
	}}
	tmpl := NewDefaultRAGTemplate()
	cm := &fakeCM{}

	chain, err := NewRAGChain(fr, tmpl, cm, nil, 5, 15)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	result, err := chain.Invoke(context.Background(), "What is Go?")
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	// fakeCM returns the template directly, so we can verify context was injected.
	if result.Content == "" {
		t.Error("content should not be empty")
	}
}

func TestNewDefaultRAGTemplate(t *testing.T) {
	tmpl := NewDefaultRAGTemplate()
	if tmpl == nil {
		t.Fatal("template should not be nil")
	}
}

// fakeCM is a ChatModel that returns the input messages as output for testing.
type fakeCM struct{}

func (f *fakeCM) Generate(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if len(msgs) == 0 {
		return &schema.Message{Role: schema.Assistant, Content: "no input"}, nil
	}
	return &schema.Message{Role: schema.Assistant, Content: msgs[len(msgs)-1].Content}, nil
}

func (f *fakeCM) Stream(ctx context.Context, msgs []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	msg, _ := f.Generate(ctx, msgs)
	return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
}

func (f *fakeCM) BindTools(tools []*schema.ToolInfo) error { return nil }

var _ prompt.ChatTemplate = NewDefaultRAGTemplate()
