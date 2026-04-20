package execution

type Status string

const (
	StatusPending   Status = "pending"
	StatusApproved  Status = "approved"
	StatusExecuting Status = "executing"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusTimeout   Status = "timeout"
	StatusRejected  Status = "rejected"
)

type Request struct {
	ID         string
	SessionID  string
	TargetHost string
	Command    string
	Status     Status
}
