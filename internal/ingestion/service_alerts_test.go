package ingestion

import (
	"testing"

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
