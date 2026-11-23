package notify

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLogNotifier_NoTokensIsNoop(t *testing.T) {
	n := NewLogNotifier(discardLogger())
	if err := n.Notify(context.Background(), nil, Notification{Title: "x"}); err != nil {
		t.Fatalf("empty tokens should be a no-op, got %v", err)
	}
}

func TestLogNotifier_SendsWithoutError(t *testing.T) {
	n := NewLogNotifier(discardLogger())
	err := n.Notify(context.Background(), []string{"tok1", "tok2"},
		Notification{Title: "Blood needed", Body: "A+ near you"})
	if err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
}
