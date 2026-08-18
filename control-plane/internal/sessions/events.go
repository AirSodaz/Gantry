package sessions

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/AirSodaz/gantry/internal/sessionevents"
	"github.com/jackc/pgx/v5"
)

func appendEvent(ctx context.Context, tx pgx.Tx, runID, eventType string) error {
	return sessionevents.Append(ctx, tx, runID, eventType, map[string]any{})
}
func appendSessionEvent(ctx context.Context, tx pgx.Tx, sessionID, eventType string, payload any) error {
	return sessionevents.AppendSession(ctx, tx, sessionID, eventType, payload)
}
func appendEventPayload(ctx context.Context, tx pgx.Tx, runID, eventType, payload string) error {
	var value any = map[string]any{}
	if strings.TrimSpace(payload) != "" {
		if err := json.Unmarshal([]byte(payload), &value); err != nil {
			return err
		}
	}
	return sessionevents.Append(ctx, tx, runID, eventType, value)
}
