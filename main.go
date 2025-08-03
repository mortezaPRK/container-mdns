package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/hashicorp/mdns"
)

func main() {
	var (
		interval = flag.Duration("interval", 5*time.Second, "Burst gate interval to avoid excessive updates during rapid Docker events")
		ip       = flag.String("ip", "", "IP address to publish (overrides HOSTNAME_IP env)")
	)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	publishIP, err := getIPFromEnvOrDefault(*ip)
	if err != nil {
		log.Fatalf("Failed to get IP from environment: %v", err)
	}

	initialHostnames, err := getHostnames(ctx, cli)
	if err != nil {
		log.Fatalf("Failed to get hostnames: %v", err)
	}

	zone, shutdown, err := startPublisher(publishIP, initialHostnames)
	if err != nil {
		log.Fatalf("Failed to start mDNS publisher: %v", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			log.Printf("Failed to shutdown mDNS publisher: %v", err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		log.Println("Cleaning up mDNS publishers...")
		cancel()
	}()

	if err := dockerEventProducer(
		ctx,
		cli,
		*interval,
		func() error {
			hostnames, err := getHostnames(ctx, cli)
			if err != nil {
				return fmt.Errorf("error getting hostnames: %w", err)
			}

			zone.Sync(hostnames)

			return nil
		},
	); err != nil {
		log.Fatalf("Docker event producer failed: %v", err)
	}
}

func startPublisher(ip string, hostnames []string) (*MultiHostZone, func() error, error) {
	zone := newMultiHostZone(net.ParseIP(ip), hostnames)

	server, err := mdns.NewServer(&mdns.Config{Zone: zone})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start mDNS server: %w", err)
	}

	return zone, server.Shutdown, nil
}
