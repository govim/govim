package protocol

import (
	"context"
	"fmt"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/telemetry"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/xcontext"
)

type contextKey int

const (
	clientKey = contextKey(iota)
)

func WithClient(ctx context.Context, client Client) context.Context {
	return context.WithValue(ctx, clientKey, client)
}

func LogEvent(ctx context.Context, event telemetry.Event) {
	client, ok := ctx.Value(clientKey).(Client)
	if !ok {
		return
	}
	msg := &LogMessageParams{Type: Info, Message: fmt.Sprint(event)}
	if event.Error != nil {
		msg.Type = Error
	}
	go client.LogMessage(xcontext.Detach(ctx), msg)
}
