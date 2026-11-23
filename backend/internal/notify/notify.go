// Package notify abstracts outbound push notifications (FR-09). The backend
// dispatches a Notification to a donor's device tokens through a Notifier. The
// default LogNotifier just logs — so the whole flow runs locally and in tests
// without Firebase credentials — and a real FCM client implements the same
// interface for production (dependency-injected at startup).
package notify

import (
	"context"
	"log/slog"
)

// Notification is a single push message. Data carries optional key/value pairs
// the client app can act on (e.g. the request id to deep-link to).
type Notification struct {
	Title string
	Body  string
	Data  map[string]string
}

// Notifier sends a Notification to the given device tokens. Implementations must
// be safe for concurrent use. A nil/empty tokens slice is a no-op.
type Notifier interface {
	Notify(ctx context.Context, tokens []string, n Notification) error
}

// LogNotifier records notifications via slog instead of sending them. It is the
// default in development and tests; swap in an FCM-backed Notifier in production.
type LogNotifier struct {
	logger *slog.Logger
}

// NewLogNotifier returns a LogNotifier writing to logger.
func NewLogNotifier(logger *slog.Logger) *LogNotifier {
	return &LogNotifier{logger: logger}
}

// Notify logs the notification and its recipient count.
func (l *LogNotifier) Notify(_ context.Context, tokens []string, n Notification) error {
	if len(tokens) == 0 {
		return nil
	}
	l.logger.Info("push notification (log-only)",
		"recipients", len(tokens), "title", n.Title, "body", n.Body)
	return nil
}
