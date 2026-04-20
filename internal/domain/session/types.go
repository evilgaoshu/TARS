package session

type Status string

const (
	StatusOpen            Status = "open"
	StatusAnalyzing       Status = "analyzing"
	StatusPendingApproval Status = "pending_approval"
	StatusExecuting       Status = "executing"
	StatusVerifying       Status = "verifying"
	StatusResolved        Status = "resolved"
	StatusFailed          Status = "failed"
)

type Aggregate struct {
	ID     string
	Status Status
}
