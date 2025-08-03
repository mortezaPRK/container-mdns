package main

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
)

type (
	nothing struct{}

	fqdn string

	ContainerLister interface {
		ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	}

	EventProducer interface {
		Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
	}
)
