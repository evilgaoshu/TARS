package ticket

type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusVerifying  Status = "verifying"
	StatusResolved   Status = "resolved"
	StatusClosed     Status = "closed"
)

type Aggregate struct {
	ID     string
	Status Status
}
