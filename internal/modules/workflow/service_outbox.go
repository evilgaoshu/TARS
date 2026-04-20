package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tars/internal/contracts"
)

func (s *Service) ListOutbox(_ context.Context, filter contracts.ListOutboxFilter) ([]contracts.OutboxEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]contracts.OutboxEvent, 0, len(s.outboxOrder))
	for i := len(s.outboxOrder) - 1; i >= 0; i-- {
		event := s.outbox[s.outboxOrder[i]]
		if filter.Status != "" {
			if event.Status != filter.Status {
				continue
			}
		} else if event.Status != "blocked" && event.Status != "failed" {
			continue
		}
		item := cloneOutboxEvent(*event)
		if !matchesOutboxQuery(item, filter.Query) {
			continue
		}
		items = append(items, item)
	}
	sortOutboxEvents(items, filter.SortBy, filter.SortOrder)
	return items, nil
}

func (s *Service) ReplayOutbox(_ context.Context, eventID string, operatorReason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.outbox[eventID]
	if !ok {
		return contracts.ErrNotFound
	}
	if event.Status == "blocked" && !s.topicEnabled(event.Topic) {
		return contracts.ErrBlockedByFeatureFlag
	}

	event.Status = "pending"
	event.BlockedReason = ""
	event.LastError = ""
	event.RetryCount++

	if record, ok := s.sessions[event.AggregateID]; ok {
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     "outbox_replayed",
			Message:   fmt.Sprintf("operator replayed outbox event: %s", operatorReason),
			CreatedAt: time.Now().UTC(),
		})
	}

	return nil
}

func (s *Service) DeleteOutbox(_ context.Context, eventID string, operatorReason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.outbox[eventID]
	if !ok {
		return contracts.ErrNotFound
	}
	if event.Status != "failed" && event.Status != "blocked" {
		return contracts.ErrInvalidState
	}

	delete(s.outbox, eventID)
	delete(s.outboxPayloads, eventID)
	for i, id := range s.outboxOrder {
		if id == eventID {
			s.outboxOrder = append(s.outboxOrder[:i], s.outboxOrder[i+1:]...)
			break
		}
	}

	if record, ok := s.sessions[event.AggregateID]; ok {
		record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
			Event:     "outbox_deleted",
			Message:   fmt.Sprintf("operator deleted outbox event: %s", operatorReason),
			CreatedAt: time.Now().UTC(),
		})
	}

	return nil
}

func (s *Service) RecoverProcessingOutbox(ctx context.Context) (int, error) {
	return s.RecoverPendingEvents(ctx)
}

func (s *Service) RecoverPendingEvents(_ context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	recovered := 0
	for _, eventID := range s.outboxOrder {
		event := s.outbox[eventID]
		if event.Status != "processing" {
			continue
		}
		event.Status = "pending"
		recovered++
	}
	return recovered, nil
}

func (s *Service) ClaimOutboxBatch(ctx context.Context, limit int) ([]contracts.DispatchableOutboxEvent, error) {
	envelopes, err := s.ClaimEvents(ctx, limit)
	if err != nil {
		return nil, err
	}
	items := make([]contracts.DispatchableOutboxEvent, 0, len(envelopes))
	for _, event := range envelopes {
		items = append(items, toDispatchableOutboxEvent(event))
	}
	return items, nil
}

func (s *Service) ClaimEvents(_ context.Context, limit int) ([]contracts.EventEnvelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 1
	}

	items := make([]contracts.EventEnvelope, 0, limit)
	now := time.Now().UTC()
	for i := len(s.outboxOrder) - 1; i >= 0 && len(items) < limit; i-- {
		event := s.outbox[s.outboxOrder[i]]
		if event.Status != "pending" {
			continue
		}
		if !event.AvailableAt.IsZero() && event.AvailableAt.After(now) {
			continue
		}

		event.Status = "processing"
		items = append(items, s.outboxEnvelope(event.ID, *event))
	}

	return items, nil
}

func (s *Service) EnqueueNotifications(_ context.Context, sessionID string, messages []contracts.ChannelMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for _, message := range messages {
		payload, err := contracts.EncodeChannelMessage(message)
		if err != nil {
			return err
		}
		if _, err := s.publishEventLocked(contracts.EventPublishRequest{
			Topic:       "telegram.send",
			AggregateID: sessionID,
			Payload:     payload,
			Status:      "pending",
			CreatedAt:   now,
			AvailableAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CompleteOutbox(ctx context.Context, eventID string) error {
	return s.ResolveEvent(ctx, eventID, contracts.DeliveryResult{Decision: contracts.DeliveryDecisionAck})
}

func (s *Service) ResolveEvent(_ context.Context, eventID string, result contracts.DeliveryResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.outbox[eventID]
	if !ok {
		return contracts.ErrNotFound
	}
	now := time.Now().UTC()
	result.LastError = strings.TrimSpace(result.LastError)
	switch result.Decision {
	case contracts.DeliveryDecisionAck:
		event.Status = "done"
		event.BlockedReason = ""
		event.LastError = ""
		event.AvailableAt = time.Time{}
		return nil
	case contracts.DeliveryDecisionRetry:
		event.RetryCount++
		event.Status = "pending"
		event.LastError = result.LastError
		event.AvailableAt = now.Add(result.Delay)
		if record, ok := s.sessions[event.AggregateID]; ok {
			message := fmt.Sprintf("event retry scheduled after delivery failure: %s", result.LastError)
			if result.Reason != "" {
				message = fmt.Sprintf("%s (%s)", message, result.Reason)
			}
			record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
				Event:     "outbox_retry_scheduled",
				Message:   message,
				CreatedAt: now,
			})
		}
		return nil
	case contracts.DeliveryDecisionDeadLetter:
		event.RetryCount++
		event.Status = "failed"
		event.LastError = result.LastError
		event.AvailableAt = time.Time{}
		if record, ok := s.sessions[event.AggregateID]; ok {
			message := result.LastError
			if result.Reason != "" {
				message = fmt.Sprintf("%s (%s)", message, result.Reason)
			}
			record.detail.Timeline = append(record.detail.Timeline, contracts.TimelineEvent{
				Event:     "outbox_failed",
				Message:   message,
				CreatedAt: now,
			})
		}
		return nil
	default:
		return fmt.Errorf("unsupported delivery decision: %s", result.Decision)
	}
}

func (s *Service) MarkOutboxFailed(ctx context.Context, eventID string, lastError string) error {
	s.mu.RLock()
	event, ok := s.outbox[eventID]
	if !ok {
		s.mu.RUnlock()
		return contracts.ErrNotFound
	}
	policy := contracts.DefaultDeliveryPolicy(event.Topic)
	attempt := event.RetryCount + 1
	s.mu.RUnlock()
	decision := policy.Decide(attempt, fmt.Errorf("%s", strings.TrimSpace(lastError)))
	return s.ResolveEvent(ctx, eventID, decision)
}

func (s *Service) PublishEvent(_ context.Context, event contracts.EventPublishRequest) (contracts.EventEnvelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.publishEventLocked(event)
}

func (s *Service) publishEventLocked(event contracts.EventPublishRequest) (contracts.EventEnvelope, error) {
	now := event.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	status := strings.TrimSpace(event.Status)
	if status == "" {
		status = "pending"
	}
	availableAt := event.AvailableAt
	if availableAt.IsZero() && status == "pending" {
		availableAt = now
	}
	outboxEvent := &contracts.OutboxEvent{
		ID:            s.nextID("evt", &s.outboxSeq),
		Topic:         strings.TrimSpace(event.Topic),
		Status:        status,
		AggregateID:   strings.TrimSpace(event.AggregateID),
		Headers:       cloneStringMap(event.Headers),
		Metadata:      event.Metadata,
		BlockedReason: strings.TrimSpace(event.BlockedReason),
		AvailableAt:   availableAt,
		CreatedAt:     now,
	}
	s.outbox[outboxEvent.ID] = outboxEvent
	s.outboxPayloads[outboxEvent.ID] = append([]byte(nil), event.Payload...)
	s.outboxOrder = append(s.outboxOrder, outboxEvent.ID)
	return s.outboxEnvelope(outboxEvent.ID, *outboxEvent), nil
}

func (s *Service) outboxEnvelope(eventID string, event contracts.OutboxEvent) contracts.EventEnvelope {
	return contracts.EventEnvelope{
		EventID:       eventID,
		Topic:         event.Topic,
		AggregateID:   event.AggregateID,
		Payload:       append([]byte(nil), s.outboxPayloads[eventID]...),
		Headers:       cloneStringMap(event.Headers),
		Metadata:      event.Metadata,
		Attempt:       event.RetryCount + 1,
		Status:        event.Status,
		BlockedReason: event.BlockedReason,
		LastError:     event.LastError,
		AvailableAt:   event.AvailableAt,
		CreatedAt:     event.CreatedAt,
	}
}

func toDispatchableOutboxEvent(event contracts.EventEnvelope) contracts.DispatchableOutboxEvent {
	return contracts.DispatchableOutboxEvent{
		EventID:     event.EventID,
		Topic:       event.Topic,
		AggregateID: event.AggregateID,
		Headers:     cloneStringMap(event.Headers),
		Metadata:    event.Metadata,
		Attempt:     event.Attempt,
		Status:      event.Status,
		LastError:   event.LastError,
		AvailableAt: event.AvailableAt,
		CreatedAt:   event.CreatedAt,
		Payload:     append([]byte(nil), event.Payload...),
	}
}

func (s *Service) topicEnabled(topic string) bool {
	switch topic {
	case "session.analyze_requested":
		return s.opts.DiagnosisEnabled
	case "session.closed":
		return s.opts.KnowledgeIngestEnabled
	default:
		return true
	}
}
