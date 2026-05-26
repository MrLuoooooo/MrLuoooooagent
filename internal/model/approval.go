package model

import "time"

// ApprovalStatus for pending action confirmation.
type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalAccepted ApprovalStatus = "accepted"
	ApprovalRejected ApprovalStatus = "rejected"
	ApprovalExpired  ApprovalStatus = "expired"
)

// ApprovalItem represents a single pending approval request from the agent.
type ApprovalItem struct {
	ID          string         `json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	Source      string         `json:"source"`      // "cron" / "chat"
	TaskName    string         `json:"task_name"`    // cron job name or conversation ID
	ActionType  string         `json:"action_type"`  // brief description of the action
	RiskLevel   string         `json:"risk_level"`   // 低/中/高
	Reason      string         `json:"reason"`       // why approval is needed
	Prompt      string         `json:"prompt"`       // the original prompt that triggered this
	FullOutput  string         `json:"full_output"`  // complete agent output for review
	Status      ApprovalStatus `json:"status"`
	ApprovedAt  *time.Time     `json:"approved_at,omitempty"`
}
