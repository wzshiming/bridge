//go:build windows
// +build windows

package main

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/wzshiming/bridge/config"
)

func runWithReload(ctx context.Context, log logr.Logger, tasks []config.Chain, configs []string) {
	run(ctx, log, tasks)
}
