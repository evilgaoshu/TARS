package dto

import "time"

// NotificationTemplateContent is the subject+body of a notification template.
type NotificationTemplateContent struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// NotificationTemplate is the API representation of a notification template.
type NotificationTemplate struct {
	ID             string                      `json:"id"`
	Kind           string                      `json:"kind"`
	Locale         string                      `json:"locale"`
	Name           string                      `json:"name"`
	Status         string                      `json:"status,omitempty"`
	Enabled        bool                        `json:"enabled"`
	VariableSchema map[string]string            `json:"variable_schema,omitempty"`
	UsageRefs      []string                    `json:"usage_refs,omitempty"`
	Content        NotificationTemplateContent `json:"content"`
	UpdatedAt      time.Time                   `json:"updated_at,omitempty"`
}

// NotificationTemplateListResponse is the paginated list response for notification templates.
type NotificationTemplateListResponse struct {
	Items []NotificationTemplate `json:"items"`
	ListPage
}

// NotificationTemplateUpsertRequest is used for POST/PUT of a notification template.
type NotificationTemplateUpsertRequest struct {
	Template       NotificationTemplate `json:"template"`
	OperatorReason string               `json:"operator_reason"`
}

type MsgTemplateContent = NotificationTemplateContent
type MsgTemplate = NotificationTemplate
type MsgTemplateListResponse = NotificationTemplateListResponse
type MsgTemplateUpsertRequest = NotificationTemplateUpsertRequest
