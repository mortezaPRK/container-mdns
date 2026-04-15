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

// realMDNSPublisher is the real implementation of MDNSPublisher
type realMDNSPublisher struct{}

func (p *realMDNSPublisher) Start(ip net.IP, hostnames []string) (*MultiHostZone, func() error, error) {
	zone := newMultiHostZone(ip, hostnames)

	server, err := mdns.NewServer(&mdns.Config{Zone: zone})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start mDNS server: %w", err)
	}

	return zone, server.Shutdown, nil
}

// realHostnameResolver is the real implementation of HostnameResolver
type realHostnameResolver struct{}

func (r *realHostnameResolver) Hostname() (string, error) {
	return osHostname()
}

// resolvePublishIP determines the IP address to publish based on precedence:
// 1. IP flag value (if non-empty)
// 2. HOSTNAME_IP environment variable
// 3. System hostname (via resolver)
func resolvePublishIP(ipFlag string, resolver HostnameResolver) (string, error) {
	if ipFlag != "" {
		return ipFlag, nil
	}

	if ip := os.Getenv("HOSTNAME_IP"); ip != "" {
		return ip, nil
	}

	hostname, err := resolver.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}

	return hostname, nil
}

// createDockerClient creates a new Docker client with API version negotiation
func createDockerClient() (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}
	return cli, nil
}

// setupSignalHandling configures signal handling for graceful shutdown
// Returns a context that will be cancelled when SIGINT/SIGTERM is received
func setupSignalHandling() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		log.Println("Cleaning up mDNS publishers...")
		cancel()
	}()

	return ctx
}

// runApp is the main application logic, extracted for testability
func runApp(ctx context.Context, cli DockerClient, publisher MDNSPublisher, hostnameResolver HostnameResolver, interval time.Duration, ipFlag string) error {
	publishIP, err := resolvePublishIP(ipFlag, hostnameResolver)
	if err != nil {
		return fmt.Errorf("failed to resolve publish IP: %w", err)
	}

	initialHostnames, err := getHostnames(ctx, cli)
	if err != nil {
		return fmt.Errorf("failed to get hostnames: %w", err)
	}

	zone, shutdown, err := publisher.Start(net.ParseIP(publishIP), initialHostnames)
	if err != nil {
		return fmt.Errorf("failed to start mDNS publisher: %w", err)
	}
	defer func() {
		if err := shutdown(); err != nil {
			log.Printf("Failed to shutdown mDNS publisher: %v", err)
		}
	}()

	if err := dockerEventProducer(
		ctx,
		cli,
		interval,
		func() error {
			hostnames, err := getHostnames(ctx, cli)
			if err != nil {
				return fmt.Errorf("error getting hostnames: %w", err)
			}

			zone.Sync(hostnames)

			return nil
		},
	); err != nil {
		return fmt.Errorf("docker event producer failed: %w", err)
	}

	return nil
}

func main() {
	var (
		interval = flag.Duration("interval", 5*time.Second, "Burst gate interval to avoid excessive updates during rapid Docker events")
		ip       = flag.String("ip", "", "IP address to publish (overrides HOSTNAME_IP env)")
	)
	flag.Parse()

	ctx := setupSignalHandling()

	cli, err := createDockerClient()
	if err != nil {
		log.Fatalf("Failed to create Docker client: %v", err)
	}

	publisher := &realMDNSPublisher{}
	hostnameResolver := &realHostnameResolver{}

	if err := runApp(ctx, cli, publisher, hostnameResolver, *interval, *ip); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
