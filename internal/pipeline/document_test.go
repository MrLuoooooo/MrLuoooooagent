package pipeline

import (
	"strings"
	"testing"
)

func TestChunkText_Empty(t *testing.T) {
	result := chunkText("", 500, 50)
	// Empty text: len(runes)=0 <= size=500 → returns [""]
	if len(result) != 1 {
		t.Errorf("expected 1 chunk (wrap-around), got %d chunks", len(result))
	}
}

func TestChunkText_ShorterThanSize(t *testing.T) {
	result := chunkText("hello", 500, 50)
	if len(result) != 1 {
		t.Errorf("expected 1 chunk, got %d", len(result))
	}
	if result[0] != "hello" {
		t.Errorf("chunk = %q", result[0])
	}
}

func TestChunkText_MultipleChunks(t *testing.T) {
	text := strings.Repeat("hello world ", 100) // ~1200 chars
	result := chunkText(text, 500, 50)

	if len(result) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(result))
	}
	for i, chunk := range result {
		if len(chunk) == 0 {
			t.Errorf("chunk %d is empty", i)
		}
		if len([]rune(chunk)) > 500 {
			t.Errorf("chunk %d too long: %d runes", i, len([]rune(chunk)))
		}
	}
}

func TestChunkText_OverlapContent(t *testing.T) {
	text := "ABCDEFGHIJ" // 10 chars
	result := chunkText(text, 4, 2)

	// With size=4, overlap=2, 10 chars:
	// chunk1: ABCD (0:4), start += 2 → 2
	// chunk2: CDEF (2:6), start += 2 → 4
	// chunk3: EFGH (4:8), start += 2 → 6
	// chunk4: GHIJ (6:10), start += 2 → 8
	// chunk5: IJ (8:10), start += 2 → 10 (end)
	if len(result) != 5 {
		t.Errorf("expected 5 chunks, got %d: %v", len(result), result)
		return
	}

	if result[0] != "ABCD" {
		t.Errorf("chunk[0] = %q", result[0])
	}
	if result[2] != "EFGH" {
		t.Errorf("chunk[2] = %q", result[2])
	}
}

func TestChunkText_ZeroSize(t *testing.T) {
	result := chunkText("hello world", 0, 0)
	if len(result) != 1 {
		t.Errorf("zero size should default to 500, got %d chunks", len(result))
	}
}

func TestChunkText_OverlapTooLarge(t *testing.T) {
	// overlap >= size should be capped
	result := chunkText("ABCDEFGHIJKLMNOP", 5, 10)
	if len(result) == 0 {
		t.Fatal("should not be empty")
	}
	for _, c := range result {
		if len(c) == 0 {
			t.Error("no empty chunks expected")
		}
	}
}

func TestChunkText_SingleRune(t *testing.T) {
	text := "A"
	result := chunkText(text, 500, 50)
	if len(result) != 1 {
		t.Errorf("single char should be 1 chunk, got %d", len(result))
	}
}
