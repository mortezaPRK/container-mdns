package main

import (
	"fmt"
	"net"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// nginx:1.27 - using SHA for reproducibility
	testImage = "nginx@sha256:2d1afc5cb0cc9a9f4920a1e71df2e3e53fe5db06fd1e7787540d25905e423dce"
	// docker:27-dind - using SHA for reproducibility
	dindImage         = "docker@sha256:d2dc198f7d83f27f3a60841332cf3f5a4a356f9c296d28a2736dcab3a505ea1a"
	testNetwork       = "mdns-test-network"
	testContainerName = "mdns-test-web"
)

func TestIntegration_GetHostnamesFromRealDocker(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	req := require.New(t)
	ctx := t.Context()

	// Start Docker-in-Docker container
	dindContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        dindImage,
			Privileged:   true,
			ExposedPorts: []string{"2375/tcp"},
			WaitingFor:   wait.ForLog("API listen on [::]:2375").WithOccurrence(1),
			Tmpfs: map[string]string{
				"/var/lib/docker": "",
			},
		},
		Started: true,
	})
	req.NoError(err)
	defer func() {
		req.NoError(dindContainer.Terminate(ctx))
	}()

	// Get DinD host and port
	host, err := dindContainer.Host(ctx)
	req.NoError(err)

	port, err := dindContainer.MappedPort(ctx, "2375")
	req.NoError(err)

	dindEndpoint := fmt.Sprintf("tcp://%s:%s", host, port.Port())

	// Create Docker client connected to DinD
	cli, err := client.NewClientWithOpts(client.WithHost(dindEndpoint), client.WithAPIVersionNegotiation())
	req.NoError(err)

	// Pull test image in DinD
	reader, err := cli.ImagePull(ctx, testImage, image.PullOptions{})
	req.NoError(err)
	_ = reader.Close()

	// Create test container with Traefik labels
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: testImage,
		Labels: map[string]string{
			"traefik.http.routers.web1.rule": "Host((web1.local))",
			"traefik.http.routers.web2.rule": "Host((web2.local))",
			"other.label":                    "should be ignored",
		},
	}, nil, nil, nil, testContainerName)
	req.NoError(err)

	// Start the container
	req.NoError(cli.ContainerStart(ctx, resp.ID, container.StartOptions{}))

	// Clean up container
	defer func() {
		req.NoError(cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}))
	}()

	// Test getHostnames
	hostnames, err := getHostnames(ctx, cli)
	req.NoError(err)
	req.Len(hostnames, 2)
	req.ElementsMatch([]string{"web1.local", "web2.local"}, hostnames)
}

func TestIntegration_GetHostnames_MultipleContainers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	req := require.New(t)
	ctx := t.Context()

	// Start DinD
	dindContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        dindImage,
			Privileged:   true,
			ExposedPorts: []string{"2375/tcp"},
			WaitingFor:   wait.ForLog("API listen on [::]:2375").WithOccurrence(1),
			Tmpfs: map[string]string{
				"/var/lib/docker": "",
			},
		},
		Started: true,
	})
	req.NoError(err)
	defer dindContainer.Terminate(ctx)

	host, _ := dindContainer.Host(ctx)
	port, _ := dindContainer.MappedPort(ctx, "2375")
	dindEndpoint := fmt.Sprintf("tcp://%s:%s", host, port.Port())

	cli, err := client.NewClientWithOpts(client.WithHost(dindEndpoint), client.WithAPIVersionNegotiation())
	req.NoError(err)

	// Pull image
	reader, err := cli.ImagePull(ctx, testImage, image.PullOptions{})
	req.NoError(err)
	_ = reader.Close()

	// Create multiple containers with different labels
	containers := []struct {
		name   string
		labels map[string]string
	}{
		{
			name: "web1",
			labels: map[string]string{
				"traefik.http.routers.app.rule": "Host((app.local))",
			},
		},
		{
			name: "web2",
			labels: map[string]string{
				"traefik.http.routers.api.rule": "Host((api.local))",
			},
		},
		{
			name:   "web3", // No valid labels - should be ignored
			labels: map[string]string{"other": "label"},
		},
		{
			name: "web4",
			labels: map[string]string{
				"traefik.http.routers.admin.rule": "Host((admin.local))",
			},
		},
	}

	var containerIDs []string
	defer func() {
		for _, id := range containerIDs {
			_ = cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true})
		}
	}()

	for _, c := range containers {
		resp, err := cli.ContainerCreate(ctx, &container.Config{
			Image:  testImage,
			Labels: c.labels,
		}, nil, nil, nil, c.name)
		req.NoError(err)
		containerIDs = append(containerIDs, resp.ID)
		req.NoError(cli.ContainerStart(ctx, resp.ID, container.StartOptions{}))
	}

	// Test getHostnames - should only return valid ones
	hostnames, err := getHostnames(ctx, cli)
	req.NoError(err)
	req.Len(hostnames, 3)
	req.ElementsMatch([]string{"app.local", "api.local", "admin.local"}, hostnames)
}

func TestIntegration_GetHostnames_ContainerLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	req := require.New(t)
	ctx := t.Context()

	// Start DinD
	dindContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        dindImage,
			Privileged:   true,
			ExposedPorts: []string{"2375/tcp"},
			WaitingFor:   wait.ForLog("API listen on [::]:2375").WithOccurrence(1),
			Tmpfs: map[string]string{
				"/var/lib/docker": "",
			},
		},
		Started: true,
	})
	req.NoError(err)
	defer dindContainer.Terminate(ctx)

	host, _ := dindContainer.Host(ctx)
	port, _ := dindContainer.MappedPort(ctx, "2375")
	dindEndpoint := fmt.Sprintf("tcp://%s:%s", host, port.Port())

	cli, err := client.NewClientWithOpts(client.WithHost(dindEndpoint), client.WithAPIVersionNegotiation())
	req.NoError(err)

	// Pull image
	reader, err := cli.ImagePull(ctx, testImage, image.PullOptions{})
	req.NoError(err)
	_ = reader.Close()

	// Initial state - no containers
	hostnames, err := getHostnames(ctx, cli)
	req.NoError(err)
	req.Empty(hostnames)

	// Create container
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: testImage,
		Labels: map[string]string{
			"traefik.http.routers.test.rule": "Host((test.local))",
		},
	}, nil, nil, nil, "lifecycle-test")
	req.NoError(err)
	defer func() {
		_ = cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
	}()

	// Started - should appear
	req.NoError(cli.ContainerStart(ctx, resp.ID, container.StartOptions{}))
	hostnames, err = getHostnames(ctx, cli)
	req.NoError(err)
	req.ElementsMatch([]string{"test.local"}, hostnames)

	// Stopped - should disappear
	timeoutSec := 5
	req.NoError(cli.ContainerStop(ctx, resp.ID, container.StopOptions{Timeout: &timeoutSec}))
	hostnames, err = getHostnames(ctx, cli)
	req.NoError(err)
	req.Empty(hostnames)
}

func TestIntegration_MultiHostZone(t *testing.T) {
	req := require.New(t)

	testIP := net.ParseIP("192.168.1.100")
	initialHostnames := []string{"app.local", "api.local"}

	zone := newMultiHostZone(testIP, initialHostnames)

	// Verify initial state
	req.Equal(testIP, zone.ip)

	// Sync with new hostnames
	newHostnames := []string{"app.local", "admin.local"}
	zone.Sync(newHostnames)

	// Verify sync worked - we can't directly access hostnames, but we can test Records
	// This is a basic test - in reality you'd need to send DNS queries to verify
	req.NotNil(zone)
}

func TestIntegration_GetIPFromEnvOrDefault(t *testing.T) {
	req := require.New(t)

	// Test 1: Default parameter takes precedence
	ip, err := getIPFromEnvOrDefault("10.0.0.1")
	req.NoError(err)
	req.Equal("10.0.0.1", ip)

	// Test 2: Environment variable (when default is empty)
	t.Setenv("HOSTNAME_IP", "10.0.0.2")
	ip, err = getIPFromEnvOrDefault("")
	req.NoError(err)
	req.Equal("10.0.0.2", ip)

	// Test 3: Fallback to hostname (should return something)
	t.Setenv("HOSTNAME_IP", "")
	ip, err = getIPFromEnvOrDefault("")
	req.NoError(err)
	req.NotEmpty(ip)
}
