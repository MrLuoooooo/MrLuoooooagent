package store

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestConversationMemory_SaveAndLoad(t *testing.T) {
	m := NewConversationMemory(100)
	ctx := context.Background()
	convID := "test_conv_1"

	// Save a user message.
	err := m.Save(ctx, convID, []*schema.Message{
		{Role: schema.User, Content: "hello"},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Save an assistant reply.
	err = m.Save(ctx, convID, []*schema.Message{
		{Role: schema.Assistant, Content: "hi there"},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load and verify.
	msgs, err := m.Load(ctx, convID)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	if msgs[0].Content != "hello" {
		t.Errorf("msg[0].Content = %q, want %q", msgs[0].Content, "hello")
	}
	if msgs[1].Content != "hi there" {
		t.Errorf("msg[1].Content = %q, want %q", msgs[1].Content, "hi there")
	}
}

func TestConversationMemory_HistoryCap(t *testing.T) {
	// Max 3 messages.
	m := NewConversationMemory(3)
	ctx := context.Background()
	convID := "cap_test"

	for i := 0; i < 5; i++ {
		msg := &schema.Message{
			Role:    schema.User,
			Content: "msg",
		}
		m.Save(ctx, convID, []*schema.Message{msg})
	}

	msgs, _ := m.Load(ctx, convID)
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages after cap, got %d", len(msgs))
	}
}

func TestConversationMemory_UnknownConversation(t *testing.T) {
	m := NewConversationMemory(100)
	msgs, err := m.Load(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if msgs != nil {
		t.Fatalf("expected nil for unknown conversation, got %d messages", len(msgs))
	}
}

func TestConversationMemory_ConcurrentSafety(t *testing.T) {
	m := NewConversationMemory(100)
	ctx := context.Background()
	done := make(chan bool, 2)

	go func() {
		for i := 0; i < 50; i++ {
			m.Save(ctx, "concurrent", []*schema.Message{
				{Role: schema.User, Content: "a"},
			})
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 50; i++ {
			m.Load(ctx, "concurrent")
		}
		done <- true
	}()

	<-done
	<-done
}

func TestConversationMemory_DefaultCap(t *testing.T) {
	m := NewConversationMemory(0)
	if m.maxHistory != 100 {
		t.Errorf("default maxHistory = %d, want 100", m.maxHistory)
	}
}

func TestNewConversationID(t *testing.T) {
	id1 := NewConversationID()
	id2 := NewConversationID()
	if id1 == id2 {
		t.Errorf("two sequential IDs are identical: %q", id1)
	}
}
