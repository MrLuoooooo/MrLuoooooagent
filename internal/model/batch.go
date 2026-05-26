package model

// BatchRequest for POST /api/v1/batch.
type BatchRequest struct {
	Tasks []BatchTask `json:"tasks" binding:"required"`
	Agent bool        `json:"agent"`
}

// BatchTask is a single task in a batch.
type BatchTask struct {
	ID     string `json:"id"`
	Prompt string `json:"prompt"`
}

// BatchProgress is the SSE event for batch streaming progress.
type BatchProgress struct {
	Type   string `json:"type"`
	TaskID string `json:"task_id,omitempty"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

const (
	BatchTaskStart = "task_start"
	BatchTaskToken = "task_token"
	BatchTaskDone  = "task_done"
	BatchTaskError = "task_error"
	BatchSummary   = "summary"
	BatchDone      = "done"
)
