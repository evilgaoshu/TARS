package contracts

import (
	"context"
	"strings"
	"time"
)

type EventMetadata struct {
	CorrelationID string
	TenantID      string
	TraceID       string
}

type EventPublishRequest struct {
	Topic         string
	AggregateID   string
	Payload       []byte
	Headers       map[string]string
	Metadata      EventMetadata
	Status        string
	BlockedReason string
	AvailableAt   time.Time
	CreatedAt     time.Time
}

type EventEnvelope struct {
	EventID       string
	Topic         string
	AggregateID   string
	Payload       []byte
	Headers       map[string]string
	Metadata      EventMetadata
	Attempt       int
	Status        string
	BlockedReason string
	LastError     string
	AvailableAt   time.Time
	CreatedAt     time.Time
}

type DeliveryDecision string

const (
	DeliveryDecisionAck        DeliveryDecision = "ack"
	DeliveryDecisionRetry      DeliveryDecision = "retry"
	DeliveryDecisionDeadLetter DeliveryDecision = "dead_letter"
)

type DeliveryResult struct {
	Decision  DeliveryDecision
	LastError string
	Delay     time.Duration
	Reason    string
}

type DeliveryPolicy struct {
	MaxAttempts   int
	RetrySchedule []time.Duration
}

func (p DeliveryPolicy) Normalize() DeliveryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 1
	}
	return p
}

func (p DeliveryPolicy) RetryDelay(attempt int) time.Duration {
	p = p.Normalize()
	if attempt <= 0 || len(p.RetrySchedule) == 0 {
		return 0
	}
	index := attempt - 1
	if index < 0 {
		index = 0
	}
	if index >= len(p.RetrySchedule) {
		return p.RetrySchedule[len(p.RetrySchedule)-1]
	}
	return p.RetrySchedule[index]
}

func (p DeliveryPolicy) Decide(attempt int, err error) DeliveryResult {
	p = p.Normalize()
	if err == nil {
		return DeliveryResult{Decision: DeliveryDecisionAck}
	}
	lastError := strings.TrimSpace(err.Error())
	if attempt < p.MaxAttempts {
		return DeliveryResult{
			Decision:  DeliveryDecisionRetry,
			LastError: lastError,
			Delay:     p.RetryDelay(attempt),
		}
	}
	return DeliveryResult{
		Decision:  DeliveryDecisionDeadLetter,
		LastError: lastError,
	}
}

type EventPublisher interface {
	PublishEvent(ctx context.Context, event EventPublishRequest) (EventEnvelope, error)
}

type EventConsumer interface {
	RecoverPendingEvents(ctx context.Context) (int, error)
	ClaimEvents(ctx context.Context, limit int) ([]EventEnvelope, error)
	ResolveEvent(ctx context.Context, eventID string, result DeliveryResult) error
}

type EventBus interface {
	EventPublisher
	EventConsumer
}

var defaultDeliveryPolicies = map[string]DeliveryPolicy{
	"session.analyze_requested": {
		MaxAttempts:   1,
		RetrySchedule: nil,
	},
	"session.closed": {
		MaxAttempts:   1,
		RetrySchedule: nil,
	},
	"telegram.send": {
		MaxAttempts:   3,
		RetrySchedule: []time.Duration{time.Second, 5 * time.Second},
	},
}

func DefaultDeliveryPolicy(topic string) DeliveryPolicy {
	if policy, ok := defaultDeliveryPolicies[topic]; ok {
		return policy.Normalize()
	}
	return DeliveryPolicy{MaxAttempts: 1}
}
