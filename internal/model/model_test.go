package model

import (
	"encoding/json"
	"testing"
	"time"
)

// --- ChatRequest / ChatResponse / StreamEvent ---

func TestChatRequest_JSON(t *testing.T) {
	req := ChatRequest{
		Question:       "hello",
		Stream:         true,
		Agent:          false,
		ConversationID: "conv-1",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal ChatRequest: %v", err)
	}
	var got ChatRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal ChatRequest: %v", err)
	}
	if got.Question != "hello" || got.Stream != true || got.ConversationID != "conv-1" {
		t.Errorf("ChatRequest round-trip mismatch: %+v", got)
	}
}

func TestChatRequest_DefaultValues(t *testing.T) {
	raw := `{"question":"test"}`
	var req ChatRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if req.Stream != false || req.Agent != false || req.ConversationID != "" {
		t.Errorf("expected zero values, got Stream=%v Agent=%v ConvID=%q",
			req.Stream, req.Agent, req.ConversationID)
	}
}

func TestChatResponseData_JSON(t *testing.T) {
	resp := ChatResponse{Code: 0, Message: "ok", Data: ChatResponseData{Content: "hi", Role: "assistant"}}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got ChatResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Code != 0 || got.Message != "ok" || got.Data.Content != "hi" || got.Data.Role != "assistant" {
		t.Errorf("ChatResponse round-trip: %+v", got)
	}
}

func TestStreamEvent_JSON(t *testing.T) {
	events := []StreamEvent{
		{Type: EventToken, Content: "hello"},
		{Type: EventToolCall, Tool: "get_weather", ToolName: "get_weather", ToolArgs: `{"loc":"beijing}"`},
		{Type: EventToolResult, Tool: "get_weather", Content: "25°C"},
		{Type: EventDone},
		{Type: EventError, Content: "something went wrong"},
		{Type: EventConversationID, Content: "conv-123"},
	}
	for _, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("Marshal %s: %v", e.Type, err)
		}
		var got StreamEvent
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal %s: %v", e.Type, err)
		}
		if got.Type != e.Type {
			t.Errorf("type mismatch: got %q want %q", got.Type, e.Type)
		}
	}
}

func TestStreamEvent_EventConstants(t *testing.T) {
	if EventToken != "token" {
		t.Errorf("EventToken = %q, want token", EventToken)
	}
	if EventToolCall != "tool_call" {
		t.Errorf("EventToolCall = %q, want tool_call", EventToolCall)
	}
	if EventToolResult != "tool_result" {
		t.Errorf("EventToolResult = %q, want tool_result", EventToolResult)
	}
	if EventDone != "done" {
		t.Errorf("EventDone = %q, want done", EventDone)
	}
	if EventError != "error" {
		t.Errorf("EventError = %q, want error", EventError)
	}
	if EventConversationID != "conversation_id" {
		t.Errorf("EventConversationID = %q, want conversation_id", EventConversationID)
	}
}

// --- BatchRequest / BatchProgress ---

func TestBatchRequest_JSON(t *testing.T) {
	br := BatchRequest{
		Tasks: []BatchTask{{ID: "1", Prompt: "do something"}, {ID: "2", Prompt: "do more"}},
		Agent: true,
	}
	data, err := json.Marshal(br)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got BatchRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got.Tasks) != 2 || got.Tasks[0].ID != "1" || got.Agent != true {
		t.Errorf("BatchRequest round-trip: %+v", got)
	}
}

func TestBatchTask_ZeroID(t *testing.T) {
	raw := `{"prompt":"hello"}`
	var bt BatchTask
	if err := json.Unmarshal([]byte(raw), &bt); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if bt.ID != "" || bt.Prompt != "hello" {
		t.Errorf("expected ID empty Prompt=hello, got ID=%q Prompt=%q", bt.ID, bt.Prompt)
	}
}

func TestBatchProgress_JSON(t *testing.T) {
	progs := []BatchProgress{
		{Type: BatchTaskStart, TaskID: "1"},
		{Type: BatchTaskToken, TaskID: "1", Result: "partial"},
		{Type: BatchTaskDone, TaskID: "1", Result: "done"},
		{Type: BatchTaskError, TaskID: "1", Error: "fail"},
		{Type: BatchSummary, Result: "all done"},
		{Type: BatchDone},
	}
	for _, p := range progs {
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("Marshal %s: %v", p.Type, err)
		}
		var got BatchProgress
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal %s: %v", p.Type, err)
		}
		if got.Type != p.Type {
			t.Errorf("type: got %q want %q", got.Type, p.Type)
		}
	}
}

func TestBatchProgress_EventConstants(t *testing.T) {
	if BatchTaskStart != "task_start" {
		t.Errorf("BatchTaskStart = %q", BatchTaskStart)
	}
	if BatchTaskToken != "task_token" {
		t.Errorf("BatchTaskToken = %q", BatchTaskToken)
	}
	if BatchTaskDone != "task_done" {
		t.Errorf("BatchTaskDone = %q", BatchTaskDone)
	}
	if BatchTaskError != "task_error" {
		t.Errorf("BatchTaskError = %q", BatchTaskError)
	}
	if BatchSummary != "summary" {
		t.Errorf("BatchSummary = %q", BatchSummary)
	}
	if BatchDone != "done" {
		t.Errorf("BatchDone = %q", BatchDone)
	}
}

// --- Conversation structs ---

func TestCreateConversationRequest_JSON(t *testing.T) {
	req := CreateConversationRequest{Title: "my chat"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got CreateConversationRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Title != "my chat" {
		t.Errorf("title = %q", got.Title)
	}
}

func TestCreateConversationResponse_JSON(t *testing.T) {
	resp := CreateConversationResponse{
		ConversationID: "conv-1",
		Title:          "test",
		CreatedAt:      "2026-01-01T00:00:00Z",
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got CreateConversationResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.ConversationID != "conv-1" || got.Title != "test" {
		t.Errorf("round-trip: %+v", got)
	}
}

func TestConversationItem_JSON(t *testing.T) {
	item := ConversationItem{
		ConversationID: "conv-2",
		Title:          "hello",
		MessageCount:   3,
		CreatedAt:      "2026-01-01",
		UpdatedAt:      "2026-01-02",
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got ConversationItem
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.MessageCount != 3 || got.ConversationID != "conv-2" {
		t.Errorf("round-trip: %+v", got)
	}
}

func TestListConversationsResponse_JSON(t *testing.T) {
	resp := ListConversationsResponse{
		Total: 1,
		Conversations: []ConversationItem{
			{ConversationID: "c1", Title: "t1", MessageCount: 2},
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got ListConversationsResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Total != 1 || len(got.Conversations) != 1 || got.Conversations[0].ConversationID != "c1" {
		t.Errorf("round-trip: %+v", got)
	}
}

func TestListConversationsResponse_EmptyList(t *testing.T) {
	resp := ListConversationsResponse{Total: 0, Conversations: []ConversationItem{}}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got ListConversationsResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got.Conversations) != 0 {
		t.Errorf("expected empty slice, got %d items", len(got.Conversations))
	}
}

// --- MessageItem / ToolCall / FunctionCall ---

func TestMessageItem_WithToolCalls(t *testing.T) {
	msg := MessageItem{
		Role:    "assistant",
		Content: "",
		ToolCalls: []ToolCall{
			{
				ID:   "call-1", Type: "function",
				Function: FunctionCall{Name: "get_weather", Arguments: `{"loc":"bj}"`},
			},
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got MessageItem
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("ToolCall round-trip: %+v", got)
	}
	if got.Content != "" {
		t.Errorf("expected empty content, got %q", got.Content)
	}
}

func TestMessageItem_WithoutToolCalls(t *testing.T) {
	msg := MessageItem{Role: "user", Content: "hello"}
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got MessageItem
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Role != "user" || got.Content != "hello" || got.ToolCalls != nil {
		t.Errorf("round-trip: %+v", got)
	}
}

func TestGetMessagesResponse_JSON(t *testing.T) {
	resp := GetMessagesResponse{
		ConversationID: "conv-1",
		Total:          2,
		Messages: []MessageItem{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "hello"},
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got GetMessagesResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Total != 2 || len(got.Messages) != 2 {
		t.Errorf("round-trip: %+v", got)
	}
}

func TestDeleteConversationResponse_JSON(t *testing.T) {
	resp := DeleteConversationResponse{ConversationID: "conv-1"}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got DeleteConversationResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.ConversationID != "conv-1" {
		t.Errorf("round-trip: %+v", got)
	}
}

// --- Document structs ---

func TestUploadDocumentResponse_JSON(t *testing.T) {
	resp := UploadDocumentResponse{
		DocumentIDs: []string{"doc-1", "doc-2"},
		ChunkCount:  10,
		Status:      "success",
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got UploadDocumentResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(got.DocumentIDs) != 2 || got.ChunkCount != 10 || got.Status != "success" {
		t.Errorf("round-trip: %+v", got)
	}
}

func TestDeleteDocumentResponse_JSON(t *testing.T) {
	resp := DeleteDocumentResponse{DocumentID: "doc-1", Status: "deleted"}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got DeleteDocumentResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.DocumentID != "doc-1" || got.Status != "deleted" {
		t.Errorf("round-trip: %+v", got)
	}
}

func TestDocumentItem_JSON(t *testing.T) {
	item := DocumentItem{
		DocumentID: "doc-1",
		Content:    "some content",
		ChunkCount: 5,
		CreatedAt:  "2026-01-01",
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got DocumentItem
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.DocumentID != "doc-1" || got.ChunkCount != 5 {
		t.Errorf("round-trip: %+v", got)
	}
}

func TestListDocumentsResponse_JSON(t *testing.T) {
	resp := ListDocumentsResponse{
		Total: 2,
		Documents: []DocumentItem{
			{DocumentID: "d1", Content: "c1"},
			{DocumentID: "d2", Content: "c2"},
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got ListDocumentsResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Total != 2 || len(got.Documents) != 2 {
		t.Errorf("round-trip: %+v", got)
	}
}

// --- Approval ---

func TestApprovalStatus_Values(t *testing.T) {
	tests := []struct {
		got  ApprovalStatus
		want string
	}{
		{ApprovalPending, "pending"},
		{ApprovalAccepted, "accepted"},
		{ApprovalRejected, "rejected"},
		{ApprovalExpired, "expired"},
	}
	for _, tt := range tests {
		if string(tt.got) != tt.want {
			t.Errorf("ApprovalStatus = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestApprovalItem_JSON(t *testing.T) {
	now := time.Now()
	item := ApprovalItem{
		ID:         "a-1",
		CreatedAt:  now,
		Source:     "chat",
		TaskName:   "conv-1",
		ActionType: "execute_command",
		RiskLevel:  "中",
		Reason:     "needs approval",
		Prompt:     "run build",
		FullOutput: "build output...",
		Status:     ApprovalPending,
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got ApprovalItem
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.ID != "a-1" || got.Source != "chat" || got.Status != ApprovalPending {
		t.Errorf("round-trip: %+v", got)
	}
	if got.ApprovedAt != nil {
		t.Errorf("expected nil ApprovedAt, got %v", got.ApprovedAt)
	}
}

func TestApprovalItem_WithApprovedAt(t *testing.T) {
	now := time.Now()
	item := ApprovalItem{
		ID:         "a-2",
		CreatedAt:  now,
		Status:     ApprovalAccepted,
		ApprovedAt: &now,
	}
	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got ApprovalItem
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.ApprovedAt == nil {
		t.Fatal("expected non-nil ApprovedAt")
	}
	// Compare within 1-second precision (JSON drops monotonic clock and nanoseconds)
	diff := got.ApprovedAt.Sub(now.Truncate(time.Second))
	if diff > time.Second || diff < -time.Second {
		t.Errorf("ApprovedAt too far off: got %v, want ~%v", got.ApprovedAt, now)
	}
}

// --- Envelope ---

func TestOK(t *testing.T) {
	env := OK("result")
	if env.Code != 0 || env.Message != "success" || env.Data != "result" {
		t.Errorf("OK() = %+v", env)
	}
}

func TestOK_NilData(t *testing.T) {
	env := OK(nil)
	if env.Code != 0 || env.Message != "success" || env.Data != nil {
		t.Errorf("OK(nil) = %+v", env)
	}
}

func TestErr(t *testing.T) {
	env := Err(1001, "not found")
	if env.Code != 1001 || env.Message != "not found" || env.Data != nil {
		t.Errorf("Err() = %+v", env)
	}
}

func TestAPIEnvelope_JSONSerialization(t *testing.T) {
	env := OK(map[string]int{"count": 42})
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded APIEnvelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Code != 0 || decoded.Message != "success" {
		t.Errorf("decoded: %+v", decoded)
	}
	// data should be a map
	m, ok := decoded.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Data type = %T, want map", decoded.Data)
	}
	if m["count"].(float64) != 42 {
		t.Errorf("count = %v", m["count"])
	}
}

func TestAPIEnvelope_ErrJSONSerialization(t *testing.T) {
	env := Err(500, "internal error")
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded APIEnvelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Code != 500 || decoded.Message != "internal error" {
		t.Errorf("decoded: %+v", decoded)
	}
}
