package service

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type mockRAG struct{}

func (m *mockRAG) Invoke(_ context.Context, input string, _ ...compose.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: "rag: " + input}, nil
}
func (m *mockRAG) Stream(_ context.Context, input string, _ ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	sr, sw := schema.Pipe[*schema.Message](5)
	go func() { sw.Send(&schema.Message{Content: "rag: " + input}, nil); sw.Close() }()
	return sr, nil
}
func (m *mockRAG) Collect(context.Context, *schema.StreamReader[string], ...compose.Option) (*schema.Message, error) {
	return nil, nil
}
func (m *mockRAG) Transform(context.Context, *schema.StreamReader[string], ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

type mockAgent struct{}

func (m *mockAgent) Invoke(_ context.Context, input *schema.Message, _ ...compose.Option) (*schema.Message, error) {
	return &schema.Message{Role: schema.Assistant, Content: "agent: " + input.Content}, nil
}
func (m *mockAgent) Stream(_ context.Context, input *schema.Message, _ ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	sr, sw := schema.Pipe[*schema.Message](5)
	go func() { sw.Send(&schema.Message{Content: "agent: " + input.Content}, nil); sw.Close() }()
	return sr, nil
}
func (m *mockAgent) Collect(context.Context, *schema.StreamReader[*schema.Message], ...compose.Option) (*schema.Message, error) {
	return nil, nil
}
func (m *mockAgent) Transform(context.Context, *schema.StreamReader[*schema.Message], ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

type mockDocChain struct{}

func (m *mockDocChain) Invoke(_ context.Context, input []byte, _ ...compose.Option) ([]string, error) {
	return []string{"mock_id"}, nil
}
func (m *mockDocChain) Stream(_ context.Context, _ []byte, _ ...compose.Option) (*schema.StreamReader[[]string], error) {
	return nil, nil
}
func (m *mockDocChain) Collect(_ context.Context, _ *schema.StreamReader[[]byte], _ ...compose.Option) ([]string, error) {
	return nil, nil
}
func (m *mockDocChain) Transform(_ context.Context, _ *schema.StreamReader[[]byte], _ ...compose.Option) (*schema.StreamReader[[]string], error) {
	return nil, nil
}

func TestChatService_Chat(t *testing.T) {
	s := NewChatService(&mockRAG{}, &mockAgent{}, zap.NewNop())
	msg, err := s.Chat(context.Background(), "hello")
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "rag: hello" {
		t.Errorf("got %q, want %q", msg.Content, "rag: hello")
	}
}

func TestChatService_Agent(t *testing.T) {
	s := NewChatService(&mockRAG{}, &mockAgent{}, zap.NewNop())
	msg, err := s.Agent(context.Background(), &schema.Message{Role: schema.User, Content: "time?"})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "agent: time?" {
		t.Errorf("got %q, want %q", msg.Content, "agent: time?")
	}
}

func TestChatService_ChatStream(t *testing.T) {
	s := NewChatService(&mockRAG{}, &mockAgent{}, zap.NewNop())
	stream, err := s.ChatStream(context.Background(), "stream test")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	chunk, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if chunk.Content != "rag: stream test" {
		t.Errorf("got %q", chunk.Content)
	}
}

func TestDocumentService_Ingest_Empty(t *testing.T) {
	s := NewDocumentService(nil, zap.NewNop())
	ids, err := s.Ingest(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if ids != nil {
		t.Errorf("expected nil for empty input, got %v", ids)
	}
}

func TestDocumentService_Ingest_OK(t *testing.T) {
	s := NewDocumentService(&mockDocChain{}, zap.NewNop())
	ids, err := s.Ingest(context.Background(), []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "mock_id" {
		t.Errorf("got %v", ids)
	}
}
