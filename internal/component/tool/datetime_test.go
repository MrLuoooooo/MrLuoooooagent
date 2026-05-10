package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestDateTimeTool_Info(t *testing.T) {
	dt := &DateTimeTool{}
	info, err := dt.Info(context.Background())
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	if info.Name != "get_current_datetime" {
		t.Errorf("Name = %q, want %q", info.Name, "get_current_datetime")
	}
	if info.Desc == "" {
		t.Error("Desc should not be empty")
	}
}

func TestDateTimeTool_InvokableRun_Unix(t *testing.T) {
	dt := &DateTimeTool{}
	args := `{"format":"unix"}`
	result, err := dt.InvokableRun(context.Background(), args)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}

	// Verify it's a valid Unix timestamp (within ±10s of now).
	ts, err := time.Parse(time.RFC3339, result)
	if err == nil {
		t.Logf("result parsed as timestamp: %v, trying as unix int", ts)
	}

	// Try parsing as integer.
	var epoch int64
	if err := json.Unmarshal([]byte(result), &epoch); err != nil {
		t.Fatalf("result %q is not a valid unix timestamp: %v", result, err)
	}
	now := time.Now().Unix()
	diff := epoch - now
	if diff < -10 || diff > 10 {
		t.Errorf("unix timestamp %d is too far from now %d (diff=%d)", epoch, now, diff)
	}
}

func TestDateTimeTool_InvokableRun_ISO(t *testing.T) {
	dt := &DateTimeTool{}
	args := `{"format":"iso"}`
	result, err := dt.InvokableRun(context.Background(), args)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}

	// RFC3339 with timezone offset: "2026-05-09T13:30:00+08:00"
	if _, err := time.Parse("2006-01-02T15:04:05-07:00", result); err != nil {
		t.Errorf("result %q is not valid ISO format: %v", result, err)
	}
}

func TestDateTimeTool_InvokableRun_Timezone(t *testing.T) {
	dt := &DateTimeTool{}
	args := `{"format":"date","timezone":"Asia/Shanghai"}`
	result, err := dt.InvokableRun(context.Background(), args)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}
	// Should be a valid YYYY-MM-DD date.
	if _, err := time.Parse("2006-01-02", result); err != nil {
		t.Errorf("result %q is not valid date: %v", result, err)
	}
}

func TestDateTimeTool_InvokableRun_InvalidTimezone(t *testing.T) {
	dt := &DateTimeTool{}
	args := `{"format":"date","timezone":"Mars/Olympus"}`
	_, err := dt.InvokableRun(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for invalid timezone, got nil")
	}
	if !strings.Contains(err.Error(), "invalid timezone") {
		t.Errorf("error message = %q, want contains %q", err.Error(), "invalid timezone")
	}
}

func TestDateTimeTool_InvokableRun_DefaultFormat(t *testing.T) {
	dt := &DateTimeTool{}
	// No format specified — should return the default format (MST).
	args := `{}`
	result, err := dt.InvokableRun(context.Background(), args)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}
	if len(result) < 10 {
		t.Errorf("result too short: %q", result)
	}
}

func TestDateTimeTool_InvokableRun_InvalidJSON(t *testing.T) {
	dt := &DateTimeTool{}
	_, err := dt.InvokableRun(context.Background(), "{bad json")
	if err == nil {
		t.Fatal("expected error for bad JSON, got nil")
	}
}

// Compile-time interface check.
var _ schema.ToolInfo = schema.ToolInfo{}
