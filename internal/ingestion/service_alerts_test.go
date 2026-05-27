package ingestion

import (
	"testing"
	"time"

	"karasu/internal/store"
)

type alertNotifierSpy struct {
	alerts []store.AlertEvent
}

func (s *alertNotifierSpy) NotifyAlert(alert store.AlertEvent) error {
	s.alerts = append(s.alerts, alert)
	return nil
}

func TestUpsertAlertNotifiesOnlyOnTransitions(t *testing.T) {
	t.Parallel()

	service := NewIngestionService(nil, nil)
	notifier := &alertNotifierSpy{}
	service.SetAlertNotifier(notifier)

	service.upsertAlert("health:test", "health", store.AlertSeverityWarning, "latency elevated", "system-health", "", true)
	service.upsertAlert("health:test", "health", store.AlertSeverityWarning, "latency elevated", "system-health", "", true)
	service.upsertAlert("health:test", "health", store.AlertSeverityInfo, "resolved", "system-health", "", false)

	if len(notifier.alerts) != 2 {
		t.Fatalf("expected 2 notifications, got %d", len(notifier.alerts))
	}
	if !notifier.alerts[0].Active {
		t.Fatal("expected first notification to be active")
	}
	if notifier.alerts[1].Active {
		t.Fatal("expected second notification to be resolved")
	}
}

func TestAlertNotificationCooldownThrottlesActiveTransitions(t *testing.T) {
	t.Parallel()

	service := NewIngestionService(nil, nil)
	notifier := &alertNotifierSpy{}
	service.SetAlertNotifier(notifier)
	if err := service.SetAlertNotifyCooldown(time.Hour); err != nil {
		t.Fatalf("failed to set notify cooldown: %v", err)
	}

	service.upsertAlert("decision:urgent:sell", "decision", store.AlertSeverityCritical, "urgent 1", "decision-engine", "", true)
	service.upsertAlert("decision:urgent:sell", "decision", store.AlertSeverityCritical, "urgent 2", "decision-engine", "", true)

	if len(notifier.alerts) != 1 {
		t.Fatalf("expected 1 notification after cooldown throttling, got %d", len(notifier.alerts))
	}
}

func TestAlertNotificationCooldownDoesNotThrottleResolve(t *testing.T) {
	t.Parallel()

	service := NewIngestionService(nil, nil)
	notifier := &alertNotifierSpy{}
	service.SetAlertNotifier(notifier)
	if err := service.SetAlertNotifyCooldown(time.Hour); err != nil {
		t.Fatalf("failed to set notify cooldown: %v", err)
	}

	service.upsertAlert("opportunity:gold", "opportunity", store.AlertSeverityInfo, "or 1", "opportunity-engine", "", true)
	service.upsertAlert("opportunity:gold", "opportunity", store.AlertSeverityInfo, "resolved", "opportunity-engine", "", false)

	if len(notifier.alerts) != 2 {
		t.Fatalf("expected create+resolve notifications, got %d", len(notifier.alerts))
	}
	if notifier.alerts[1].Active {
		t.Fatal("expected resolve notification to be active=false")
	}
}
