package service

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ── Mocks ──

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
	return &schema.Message{Role: schema.Assistant, Content: "agent: " + input.Content, ToolCalls: nil}, nil
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

// mockConvSvc records the last saved messages and reports saves.
type mockConvSvc struct {
	mu       sync.Mutex
	saved    []SaveRecord
	saveFail error // if set, SaveMessages returns this error
}

type SaveRecord struct {
	ConvID string
	Msgs   []*schema.Message
}

func (m *mockConvSvc) SaveMessages(_ context.Context, convID string, msgs []*schema.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveFail != nil {
		return m.saveFail
	}
	m.saved = append(m.saved, SaveRecord{ConvID: convID, Msgs: msgs})
	return nil
}

func (m *mockConvSvc) LoadMessages(_ context.Context, _ string) ([]*schema.Message, error) {
	return nil, nil
}

// ensure mockConvSvc compiles as expected interface
var _ interface {
	SaveMessages(context.Context, string, []*schema.Message) error
} = (*mockConvSvc)(nil)

// ── Tests ──

func newTestService(convSvc *mockConvSvc) *ChatService {
	if convSvc == nil {
		convSvc = &mockConvSvc{}
	}
	return NewChatService(&mockRAG{}, &mockAgent{}, convSvc, nil, nil, nil, nil, zap.NewNop())
}

func TestChatService_Chat_Persists(t *testing.T) {
	conv := &mockConvSvc{}
	s := newTestService(conv)

	msg, err := s.Chat(context.Background(), "hello", "conv_1")
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "rag: hello" {
		t.Errorf("got %q, want %q", msg.Content, "rag: hello")
	}

	// Chat() now only saves assistant message (user is saved by handler).
	if len(conv.saved) != 1 {
		t.Fatalf("expected 1 save, got %d", len(conv.saved))
	}
	rec := conv.saved[0]
	if rec.ConvID != "conv_1" {
		t.Errorf("convID: got %q, want conv_1", rec.ConvID)
	}
	if len(rec.Msgs) != 1 {
		t.Fatalf("expected 1 message (assistant only), got %d", len(rec.Msgs))
	}
	if rec.Msgs[0].Role != schema.Assistant || rec.Msgs[0].Content != "rag: hello" {
		t.Errorf("assistant msg: got %+v", rec.Msgs[0])
	}
}

// TestChatService_Chat_CacheHitStillPersists 语义缓存命中时也必须落库，
// 否则会话历史断裂（只有用户消息、缺助手回答）。
func TestChatService_Chat_CacheHitStillPersists(t *testing.T) {
	conv := &mockConvSvc{}
	cache := newTestCache(true) // fakeEmbedder: "分析茅台" → {1,0}
	ctx := context.Background()
	cache.Put(ctx, "分析茅台", "茅台是白酒龙头...")

	svc := NewChatService(&mockRAG{}, &mockAgent{}, conv, nil, nil, nil, cache, zap.NewNop())
	msg, err := svc.Chat(ctx, "分析茅台", "conv_1")
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if msg.Content != "茅台是白酒龙头..." {
		t.Fatalf("want cached answer, got %q", msg.Content)
	}

	conv.mu.Lock()
	defer conv.mu.Unlock()
	if len(conv.saved) == 0 {
		t.Fatal("cache hit must persist assistant message")
	}
	last := conv.saved[len(conv.saved)-1]
	if last.ConvID != "conv_1" || len(last.Msgs) == 0 || last.Msgs[0].Content != "茅台是白酒龙头..." {
		t.Fatalf("unexpected saved record: %+v", last)
	}
}

func TestChatService_Agent_Persists(t *testing.T) {
	conv := &mockConvSvc{}
	s := newTestService(conv)

	msg, err := s.Agent(context.Background(), &schema.Message{Role: schema.User, Content: "time?"}, "conv_2")
	if err != nil {
		t.Fatal(err)
	}
	if msg.Content != "agent: time?" {
		t.Errorf("got %q, want %q", msg.Content, "agent: time?")
	}

	// Agent() now only saves assistant message (user is saved by handler).
	if len(conv.saved) != 1 {
		t.Fatalf("expected 1 save, got %d", len(conv.saved))
	}
	rec := conv.saved[0]
	if rec.ConvID != "conv_2" {
		t.Errorf("convID: got %q", rec.ConvID)
	}
	if len(rec.Msgs) != 1 {
		t.Fatalf("expected 1 message (assistant only), got %d", len(rec.Msgs))
	}
}

func TestChatService_ChatStream_DoesNotPersist(t *testing.T) {
	// ChatStream should NOT persist — that's the handler's responsibility
	conv := &mockConvSvc{}
	s := newTestService(conv)

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
	if len(conv.saved) != 0 {
		t.Error("ChatStream should NOT persist; handler calls SaveMessages separately")
	}
}

func TestChatService_SaveMessages_UsesBackgroundContext(t *testing.T) {
	// Test that SaveMessages writes even when the passed ctx is cancelled
	conv := &mockConvSvc{}
	s := newTestService(conv)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel

	err := s.SaveMessages(cancelledCtx, "conv_3", "ping", "pong", nil)
	if err != nil {
		t.Fatalf("SaveMessages should succeed with cancelled ctx (uses Background): %v", err)
	}
	if len(conv.saved) != 1 {
		t.Fatalf("expected 1 save, got %d", len(conv.saved))
	}
	rec := conv.saved[0]
	if rec.ConvID != "conv_3" {
		t.Errorf("convID: got %q", rec.ConvID)
	}
	if len(rec.Msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(rec.Msgs))
	}
}

func TestChatService_SaveMessages_WithToolCalls(t *testing.T) {
	conv := &mockConvSvc{}
	s := newTestService(conv)

	toolCalls := []schema.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: schema.FunctionCall{
				Name:      "get_time",
				Arguments: "{}",
			},
		},
	}

	err := s.SaveMessages(context.Background(), "conv_4", "what time", "14:30", toolCalls)
	if err != nil {
		t.Fatal(err)
	}
	if len(conv.saved) != 1 {
		t.Fatalf("expected 1 save, got %d", len(conv.saved))
	}
	rec := conv.saved[0]
	if len(rec.Msgs[1].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call on assistant msg, got %d", len(rec.Msgs[1].ToolCalls))
	}
	if rec.Msgs[1].ToolCalls[0].Function.Name != "get_time" {
		t.Errorf("tool name: got %q", rec.Msgs[1].ToolCalls[0].Function.Name)
	}
}

func TestChatService_SaveMessages_ReportsError(t *testing.T) {
	conv := &mockConvSvc{saveFail: assertAnError("es write error")}
	s := newTestService(conv)

	err := s.SaveMessages(context.Background(), "conv_err", "q", "a", nil)
	if err == nil {
		t.Fatal("expected error from SaveMessages but got nil")
	}
}

func TestChatService_Chat_RagChainError(t *testing.T) {
	// RAG 链失败 → 降级消息而非 error（检索组件不可用不转 500），且不落 assistant 消息。
	var noRag compose.Runnable[string, *schema.Message] = &errRAG{}
	s := NewChatService(noRag, &mockAgent{}, &mockConvSvc{}, nil, nil, nil, nil, zap.NewNop())
	msg, err := s.Chat(context.Background(), "fail", "conv_no_save")
	if err != nil {
		t.Fatalf("degraded chat should not return error, got: %v", err)
	}
	if msg == nil || !strings.Contains(msg.Content, "知识库检索服务暂时不可用") {
		t.Fatalf("expected degraded message, got: %+v", msg)
	}
}

func TestChatService_ChatStream_RagChainError(t *testing.T) {
	// RAG 流失败 → 单条降级消息的流，不报 error。
	var noRag compose.Runnable[string, *schema.Message] = &errRAG{}
	s := NewChatService(noRag, &mockAgent{}, &mockConvSvc{}, nil, nil, nil, nil, zap.NewNop())
	sr, err := s.ChatStream(context.Background(), "fail")
	if err != nil {
		t.Fatalf("degraded stream should not return error, got: %v", err)
	}
	if sr == nil {
		t.Fatal("expected non-nil stream reader")
	}
	defer sr.Close()
	for {
		msg, err := sr.Recv()
		if err != nil {
			break
		}
		if !strings.Contains(msg.Content, "知识库检索服务暂时不可用") {
			t.Fatalf("expected degraded content, got: %q", msg.Content)
		}
	}
}

// ── Helper mocks for error testing ──

type errRAG struct{}

func (e *errRAG) Invoke(_ context.Context, _ string, _ ...compose.Option) (*schema.Message, error) {
	return nil, assertAnError("rag chain error")
}
func (e *errRAG) Stream(_ context.Context, _ string, _ ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, assertAnError("rag chain error")
}
func (e *errRAG) Collect(context.Context, *schema.StreamReader[string], ...compose.Option) (*schema.Message, error) {
	return nil, nil
}
func (e *errRAG) Transform(context.Context, *schema.StreamReader[string], ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

type errAgent struct{}

func (e *errAgent) Invoke(_ context.Context, _ *schema.Message, _ ...compose.Option) (*schema.Message, error) {
	return nil, assertAnError("agent error")
}
func (e *errAgent) Stream(_ context.Context, _ *schema.Message, _ ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, assertAnError("agent error")
}
func (e *errAgent) Collect(context.Context, *schema.StreamReader[*schema.Message], ...compose.Option) (*schema.Message, error) {
	return nil, nil
}
func (e *errAgent) Transform(context.Context, *schema.StreamReader[*schema.Message], ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

// assertAnError returns a sentinel error for test assertions.
type assertAnError string

func (e assertAnError) Error() string { return string(e) }
