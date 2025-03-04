//go:build !windows
// +build !windows

package main

import (
	"context"
	"log/slog"
	"sync"
	"syscall"
	"time"

	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/notify"
)

func runWithReload(ctx context.Context, log *slog.Logger, tasks []config.Chain, configs []string) {
	reloadCn := make(chan struct{}, 1)
	notify.On(syscall.SIGHUP, func() {
		select {
		case reloadCn <- struct{}{}:
		default:
		}
	})
	wg := sync.WaitGroup{}
	defer wg.Wait()
	var lastWorking = map[string]func(){}
	var cleanups []func()
	count := 1
	reloadCn <- struct{}{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-reloadCn:
		}
		log := log.With("reload_count", count)
		tasks, err := config.LoadConfig(configs...)
		if err != nil {
			for {
				log.Error("LoadConfig", "err", err)
				log.Info("Try reload again after 1 second")
				time.Sleep(time.Second)
				tasks, err = config.LoadConfig(configs...)
				if err == nil {
					break
				}
			}
		}
		working := map[string]func(){}
		for _, task := range tasks {
			uniq := task.Unique()

			cleanup := lastWorking[uniq]
			if cleanup != nil {
				working[uniq] = cleanup
				continue
			}

			ctx, cancel := context.WithCancel(ctx)
			working[uniq] = cancel
			wg.Add(1)
			go func(ctx context.Context, task config.Chain) {
				defer wg.Done()
				log.Info(chain.ShowChainWithConfig(task))
				for ctx.Err() == nil {
					b := chain.NewBridge(log, dump)
					err := b.BridgeWithConfig(ctx, task)
					if err != nil {
						log.Error("BridgeWithConfig", "err", err)
					}
					time.Sleep(time.Second)
				}
			}(ctx, task)
		}

		for uniq := range lastWorking {
			if _, ok := working[uniq]; !ok {
				cancel := lastWorking[uniq]
				if cancel != nil {
					cleanups = append(cleanups, cancel)
				}
			}
		}
		lastWorking = working

		// TODO: wait for all task is working
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}

		if len(cleanups) > 0 {
			for _, cleanup := range cleanups {
				cleanup()
			}
			cleanups = cleanups[:0]
		}
		count++
	}
}
