package store

import (
	"testing"
)

func TestNewConversationID(t *testing.T) {
	id1 := NewConversationID()
	id2 := NewConversationID()
	if id1 == id2 {
		t.Errorf("two sequential IDs are identical: %q", id1)
	}
	if len(id1) < 10 {
		t.Errorf("unexpected short ID: %q", id1)
	}
}

func TestNewConversationID_Prefix(t *testing.T) {
	id := NewConversationID()
	if len(id) < 5 || id[:5] != "conv_" {
		t.Errorf("expected conv_ prefix, got %q", id)
	}
}

func TestNewConversationID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := NewConversationID()
		if seen[id] {
			t.Fatalf("duplicate ID: %q", id)
		}
		seen[id] = true
	}
}
