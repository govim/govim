package protocol

import (
	"context"
	"fmt"
	"time"

	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/telemetry/log"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/lsp/telemetry/tag"
	"github.com/myitcv/govim/cmd/govim/internal/golang_org_x_tools/xcontext"
)

func init() {
	log.AddLogger(logger)
}

type contextKey int

const (
	clientKey = contextKey(iota)
)

func WithClient(ctx context.Context, client Client) context.Context {
	return context.WithValue(ctx, clientKey, client)
}

// logger implements log.Logger in terms of the LogMessage call to a client.
func logger(ctx context.Context, at time.Time, tags tag.List) bool {
	client, ok := ctx.Value(clientKey).(Client)
	if !ok {
		return false
	}
	entry := log.ToEntry(ctx, time.Time{}, tags)
	msg := &LogMessageParams{Type: Info, Message: fmt.Sprint(entry)}
	if entry.Error != nil {
		msg.Type = Error
	}
	go client.LogMessage(xcontext.Detach(ctx), msg)
	return true
}
