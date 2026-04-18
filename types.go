package main

import (
	"context"
	"net"

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

	// DockerClient combines ContainerLister and EventProducer for full Docker operations
	DockerClient interface {
		ContainerLister
		EventProducer
	}

	// MDNSPublisher defines the interface for creating and managing an mDNS server
	MDNSPublisher interface {
		// Start creates and starts an mDNS server for the given IP and hostnames
		// Returns the zone, a shutdown function, and any error that occurred
		Start(ip net.IP, hostnames []string) (*MultiHostZone, func() error, error)
	}

	// HostnameResolver defines the interface for resolving the system hostname
	HostnameResolver interface {
		// Hostname returns the system hostname
		Hostname() (string, error)
	}
)
