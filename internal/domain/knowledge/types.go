package knowledge

type Document struct {
	ID         string
	SourceType string
	SourceRef  string
	Title      string
}

type Record struct {
	ID         string
	SessionID  string
	DocumentID string
	Status     string
}
