package service

import (
	"context"
	"testing"
	"time"
)

func TestNoOpCache_PushGet(t *testing.T) {
	c := NewNoOpCache(10)
	ctx := context.Background()

	items := []ShortTermItem{
		{Role: "user", Content: "Q1", CreatedAt: time.Now()},
		{Role: "assistant", Content: "A1", CreatedAt: time.Now()},
		{Role: "user", Content: "Q2", CreatedAt: time.Now()},
	}
	for _, it := range items {
		if err := c.Push(ctx, "conv1", it); err != nil {
			t.Fatal(err)
		}
	}

	got, err := c.GetWindow(ctx, "conv1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	if got[0].Content != "Q1" {
		t.Fatalf("first item wrong: %s", got[0].Content)
	}
}

func TestNoOpCache_PushRespectsMaxLen(t *testing.T) {
	c := NewNoOpCache(3)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_ = c.Push(ctx, "conv1", ShortTermItem{Role: "user", Content: "msg"})
	}
	got, _ := c.GetWindow(ctx, "conv1", 10)
	if len(got) != 3 {
		t.Fatalf("expected maxLen=3 cap, got %d", len(got))
	}
}

func TestNoOpCache_GetWindowRespectsSize(t *testing.T) {
	c := NewNoOpCache(10)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_ = c.Push(ctx, "conv1", ShortTermItem{Role: "user", Content: "x"})
	}
	got, _ := c.GetWindow(ctx, "conv1", 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 (size limit), got %d", len(got))
	}
}

func TestNoOpCache_Trim(t *testing.T) {
	c := NewNoOpCache(10)
	ctx := context.Background()
	for i := 0; i < 8; i++ {
		_ = c.Push(ctx, "conv1", ShortTermItem{Content: "x"})
	}
	_ = c.Trim(ctx, "conv1", 4)
	got, _ := c.GetWindow(ctx, "conv1", 10)
	if len(got) != 4 {
		t.Fatalf("expected 4 after trim, got %d", len(got))
	}
}

func TestNoOpCache_ConvIsolation(t *testing.T) {
	c := NewNoOpCache(10)
	ctx := context.Background()
	_ = c.Push(ctx, "conv1", ShortTermItem{Content: "a"})
	_ = c.Push(ctx, "conv2", ShortTermItem{Content: "b"})

	got1, _ := c.GetWindow(ctx, "conv1", 10)
	got2, _ := c.GetWindow(ctx, "conv2", 10)
	if len(got1) != 1 || got1[0].Content != "a" {
		t.Fatalf("conv1 corrupted: %v", got1)
	}
	if len(got2) != 1 || got2[0].Content != "b" {
		t.Fatalf("conv2 corrupted: %v", got2)
	}
}

func TestNoOpCache_MissingConv(t *testing.T) {
	c := NewNoOpCache(10)
	got, _ := c.GetWindow(context.Background(), "nonexistent", 10)
	if len(got) != 0 {
		t.Fatalf("expected empty for missing conv, got %d", len(got))
	}
}

func TestNoOpCache_Close(t *testing.T) {
	c := NewNoOpCache(10)
	_ = c.Push(context.Background(), "c1", ShortTermItem{Content: "x"})
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	got, _ := c.GetWindow(context.Background(), "c1", 10)
	if len(got) != 0 {
		t.Fatalf("expected empty after close, got %d", len(got))
	}
}
