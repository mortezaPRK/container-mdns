package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

func dockerEventProducer(pctx context.Context, cli EventProducer, interval time.Duration, handler func() error) error {
	// Use context to signal the producer to stop.
	ctx, cancel := context.WithCancel(pctx)
	defer cancel()

	// the final error is used to join all errors.
	var finalErr error
	var errorMu sync.Mutex
	addError := func(err error) {
		errorMu.Lock()
		finalErr = errors.Join(finalErr, err)
		errorMu.Unlock()
	}

	// Used for waiting for all goroutines to finish.
	var wg sync.WaitGroup

	eventsCh, errs := cli.Events(pctx, events.ListOptions{
		Filters: filters.NewArgs(
			filters.Arg("event", "start"),
			filters.Arg("event", "stop"),
			filters.Arg("event", "die"),
			filters.Arg("event", "destroy"),
		),
	})

	// Avoid calling the handler too often in case of a burst of events.
	gatedChan := newBurstGate(interval, eventsCh)

	wg.Add(1)
	go func() {
		defer cancel()
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-gatedChan:
				if !ok {
					return
				}

				if err := handler(); err != nil {
					addError(fmt.Errorf("docker event handler failed: %w", err))
					return
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer cancel()
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-errs:
				if !ok {
					return
				}

				addError(fmt.Errorf("docker event producer failed: %w", err))
				return
			}
		}
	}()

	wg.Wait()

	return finalErr
}
