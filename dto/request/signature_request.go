package request

// SyncSignatureRequest carries optional identifiers when synchronizing a completed DocuSeal signature session.
type SyncSignatureRequest struct {
	SubmitterID int64  `json:"submitter_id,omitempty"`
	Slug        string `json:"slug,omitempty"`
}

// DocusealWebhookFieldValue represents a single field entry inside a DocuSeal webhook payload.
type DocusealWebhookFieldValue struct {
	Field string      `json:"field"`
	Value interface{} `json:"value"`
}

// DocusealWebhookSubmission represents the parent submission metadata inside a DocuSeal webhook event.
type DocusealWebhookSubmission struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

// DocusealWebhookData represents the submitter data inside a DocuSeal webhook payload.
type DocusealWebhookData struct {
	ID           int64                       `json:"id"`
	SubmissionID int64                       `json:"submission_id"`
	UUID         string                      `json:"uuid"`
	Email        string                      `json:"email"`
	Slug         string                      `json:"slug"`
	Name         string                      `json:"name"`
	Status       string                      `json:"status"`
	ExternalID   string                      `json:"external_id"`
	CompletedAt  *string                     `json:"completed_at"`
	Values       []DocusealWebhookFieldValue `json:"values"`
	Submission   *DocusealWebhookSubmission  `json:"submission,omitempty"`
}

// DocusealWebhookPayload represents the incoming JSON webhook from a self-hosted DocuSeal instance.
type DocusealWebhookPayload struct {
	EventType string              `json:"event_type"`
	Timestamp string              `json:"timestamp"`
	Data      DocusealWebhookData `json:"data"`
}
