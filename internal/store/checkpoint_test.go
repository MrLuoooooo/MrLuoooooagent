package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckpointStore_GetSet(t *testing.T) {
	dir := t.TempDir()
	s, err := NewCheckpointStore(dir)
	if err != nil {
		t.Fatalf("NewCheckpointStore: %v", err)
	}

	ctx := context.Background()

	// Set checkpoint
	data := []byte(`{"state":"test","step":3}`)
	if err := s.Set(ctx, "conv_001", data); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Get back
	got, exists, err := s.Get(ctx, "conv_001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !exists {
		t.Fatal("Get returned exists=false, expected true")
	}
	if string(got) != string(data) {
		t.Errorf("data mismatch: got %q, want %q", string(got), string(data))
	}
}

func TestCheckpointStore_GetNotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := NewCheckpointStore(dir)
	if err != nil {
		t.Fatalf("NewCheckpointStore: %v", err)
	}

	_, exists, err := s.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if exists {
		t.Error("Get returned exists=true for nonexistent key")
	}
}

func TestCheckpointStore_Delete(t *testing.T) {
	dir := t.TempDir()
	s, err := NewCheckpointStore(dir)
	if err != nil {
		t.Fatalf("NewCheckpointStore: %v", err)
	}

	ctx := context.Background()
	if err := s.Set(ctx, "conv_002", []byte(`{"state":"data"}`)); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Verify exists
	_, exists, err := s.Get(ctx, "conv_002")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !exists {
		t.Fatal("checkpoint should exist after Set")
	}

	// Delete
	if err := s.Delete(ctx, "conv_002"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify gone
	_, exists, err = s.Get(ctx, "conv_002")
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if exists {
		t.Error("checkpoint still exists after Delete")
	}
}

func TestCheckpointStore_DeleteNotExist(t *testing.T) {
	dir := t.TempDir()
	s, err := NewCheckpointStore(dir)
	if err != nil {
		t.Fatalf("NewCheckpointStore: %v", err)
	}

	if err := s.Delete(context.Background(), "no_such_file"); err != nil {
		t.Fatalf("Delete nonexistent: %v", err)
	}
}

func TestCheckpointStore_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	s, err := NewCheckpointStore(dir)
	if err != nil {
		t.Fatalf("NewCheckpointStore: %v", err)
	}

	ctx := context.Background()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			_ = s.Set(ctx, "conv_concurrent", []byte(`{"step":`+string(rune('0'+id))+`}`))
			_, _, _ = s.Get(ctx, "conv_concurrent")
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestNewCheckpointStore_CreatesDir(t *testing.T) {
	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "subdir", "nested")
	s, err := NewCheckpointStore(dataDir)
	if err != nil {
		t.Fatalf("NewCheckpointStore: %v", err)
	}

	expectedDir := filepath.Join(dataDir, "checkpoints")
	if info, err := os.Stat(expectedDir); err != nil {
		t.Fatalf("checkpoints dir was not created: %v", err)
	} else if !info.IsDir() {
		t.Fatal("checkpoints path is not a directory")
	}
	_ = s
}

func TestCheckpointStore_FileContent(t *testing.T) {
	dir := t.TempDir()
	s, err := NewCheckpointStore(dir)
	if err != nil {
		t.Fatalf("NewCheckpointStore: %v", err)
	}

	ctx := context.Background()
	original := []byte(`{"foo":"bar"}`)
	if err := s.Set(ctx, "conv_filecheck", original); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(s.dir, "conv_filecheck.eino_cp")
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read checkpoint file: %v", err)
	}
	if string(data) != string(original) {
		t.Fatalf("file content mismatch: got %q, want %q", string(data), string(original))
	}

	// Delete and verify removal
	if err := s.Delete(ctx, "conv_filecheck"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatal("checkpoint file still exists after delete")
	}
}
