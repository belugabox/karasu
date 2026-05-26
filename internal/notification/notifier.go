package notification

import "karasu/internal/store"

// AlertNotifier publishes alert transitions to an external channel.
type AlertNotifier interface {
	NotifyAlert(alert store.AlertEvent) error
}
