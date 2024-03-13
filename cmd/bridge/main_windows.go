//go:build windows
// +build windows

package main

import (
	"context"
	"log/slog"

	"github.com/wzshiming/bridge/config"
)

func runWithReload(ctx context.Context, log *slog.Logger, tasks []config.Chain, configs []string) {
	run(ctx, log, tasks)
}
