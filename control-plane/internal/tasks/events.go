package tasks

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/AirSodaz/gantry/internal/taskevents"
	"github.com/jackc/pgx/v5"
)

func appendEvent(ctx context.Context, tx pgx.Tx, runID, eventType string) error {
	return taskevents.Append(ctx, tx, runID, eventType, map[string]any{})
}
func appendEventPayload(ctx context.Context, tx pgx.Tx, runID, eventType, payload string) error {
	var value any = map[string]any{}
	if strings.TrimSpace(payload) != "" {
		if err := json.Unmarshal([]byte(payload), &value); err != nil {
			return err
		}
	}
	return taskevents.Append(ctx, tx, runID, eventType, value)
}
