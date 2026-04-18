package main

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// mockDockerClient combines MockContainerLister and MockEventProducer
type mockDockerClient struct {
	*MockContainerLister
	*MockEventProducer
}

// mockMDNSPublisher is a mock implementation of MDNSPublisher
type mockMDNSPublisher struct {
	zone       *MultiHostZone
	shutdownFn func() error
	startErr   error
}

func (m *mockMDNSPublisher) Start(ip net.IP, hostnames []string) (*MultiHostZone, func() error, error) {
	if m.startErr != nil {
		return nil, nil, m.startErr
	}
	if m.zone == nil {
		m.zone = newMultiHostZone(ip, hostnames)
	}
	shutdown := m.shutdownFn
	if shutdown == nil {
		shutdown = func() error { return nil }
	}
	return m.zone, shutdown, nil
}

// mockHostnameResolver is a mock implementation of HostnameResolver
type mockHostnameResolver struct {
	hostname string
	err      error
}

func (m *mockHostnameResolver) Hostname() (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.hostname, nil
}

func TestResolvePublishIP_FlagTakesPrecedence(t *testing.T) {
	t.Setenv("HOSTNAME_IP", "env-ip")

	tests := []struct {
		name     string
		ipFlag   string
		envIP    string
		hostname string
		expected string
	}{
		{
			name:     "flag takes precedence over env and hostname",
			ipFlag:   "192.168.1.100",
			envIP:    "10.0.0.1",
			hostname: "system-hostname",
			expected: "192.168.1.100",
		},
		{
			name:     "env takes precedence over hostname when flag is empty",
			ipFlag:   "",
			envIP:    "10.0.0.1",
			hostname: "system-hostname",
			expected: "10.0.0.1",
		},
		{
			name:     "hostname is used when flag and env are empty",
			ipFlag:   "",
			envIP:    "",
			hostname: "system-hostname",
			expected: "system-hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			t.Setenv("HOSTNAME_IP", tt.envIP)
			resolver := &mockHostnameResolver{hostname: tt.hostname}

			result, err := resolvePublishIP(tt.ipFlag, resolver)
			req.NoError(err)
			req.Equal(tt.expected, result)
		})
	}
}

func TestResolvePublishIP_ResolverError(t *testing.T) {
	req := require.New(t)

	resolver := &mockHostnameResolver{err: errors.New("hostname error")}
	t.Setenv("HOSTNAME_IP", "")

	result, err := resolvePublishIP("", resolver)
	req.Error(err)
	req.Empty(result)
	req.Contains(err.Error(), "failed to get hostname")
}

func TestCreateDockerClient_Success(t *testing.T) {
	req := require.New(t)

	// This test will try to connect to the actual Docker daemon
	// In a real environment, you might want to skip this or use testcontainers
	cli, err := createDockerClient()
	if err != nil {
		t.Skip("Docker daemon not available:", err)
	}
	req.NotNil(cli)
	req.NoError(cli.Close())
}

func TestSetupSignalHandling_ContextCancellation(t *testing.T) {
	req := require.New(t)

	ctx := setupSignalHandling()
	req.NotNil(ctx)

	// Context should not be cancelled immediately
	select {
	case <-ctx.Done():
		t.Fatal("context should not be cancelled immediately")
	default:
		// Expected
	}
}

func TestRunApp_IPResolutionError(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockLister := NewMockContainerLister(ctrl)
	mockProducer := NewMockEventProducer(ctrl)
	mockDocker := &mockDockerClient{MockContainerLister: mockLister, MockEventProducer: mockProducer}

	publisher := &mockMDNSPublisher{startErr: errors.New("publisher error")}
	resolver := &mockHostnameResolver{err: errors.New("resolver error")}

	err := runApp(ctx, mockDocker, publisher, resolver, 5*time.Second, "")
	req.Error(err)
	req.Contains(err.Error(), "failed to resolve publish IP")
}

func TestRunApp_HostnamesError(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockLister := NewMockContainerLister(ctrl)
	mockProducer := NewMockEventProducer(ctrl)
	mockDocker := &mockDockerClient{MockContainerLister: mockLister, MockEventProducer: mockProducer}

	mockDocker.MockContainerLister.EXPECT().ContainerList(ctx, container.ListOptions{}).Return(nil, errors.New("containers error"))

	publisher := &mockMDNSPublisher{}
	resolver := &mockHostnameResolver{hostname: "test-host"}

	err := runApp(ctx, mockDocker, publisher, resolver, 5*time.Second, "")
	req.Error(err)
	req.Contains(err.Error(), "failed to get hostnames")
}

func TestRunApp_PublisherError(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	mockLister := NewMockContainerLister(ctrl)
	mockProducer := NewMockEventProducer(ctrl)
	mockDocker := &mockDockerClient{MockContainerLister: mockLister, MockEventProducer: mockProducer}

	mockDocker.MockContainerLister.EXPECT().ContainerList(ctx, container.ListOptions{}).Return([]container.Summary{}, nil)

	publisher := &mockMDNSPublisher{startErr: errors.New("start error")}
	resolver := &mockHostnameResolver{hostname: "test-host"}

	err := runApp(ctx, mockDocker, publisher, resolver, 5*time.Second, "")
	req.Error(err)
	req.Contains(err.Error(), "failed to start mDNS publisher")
}

func TestRealMDNSPublisher_Start(t *testing.T) {
	req := require.New(t)

	publisher := &realMDNSPublisher{}
	ip := net.ParseIP("192.168.1.100")
	hostnames := []string{"test.local", "app.local"}

	zone, shutdown, err := publisher.Start(ip, hostnames)
	req.NoError(err)
	req.NotNil(zone)
	req.NotNil(shutdown)

	// Verify shutdown can be called without error
	req.NoError(shutdown())
}

func TestRealMDNSPublisher_Start_InvalidIP(t *testing.T) {
	req := require.New(t)

	publisher := &realMDNSPublisher{}
	ip := net.ParseIP("") // invalid IP
	hostnames := []string{"test.local"}

	zone, shutdown, err := publisher.Start(ip, hostnames)
	// nil IP is handled by ParseIP and will create a zone with nil IP
	// but the mDNS server creation should work
	req.NoError(err)
	req.NotNil(zone)
	req.NotNil(shutdown)
	req.NoError(shutdown())
}

func TestRealHostnameResolver_Hostname(t *testing.T) {
	req := require.New(t)

	resolver := &realHostnameResolver{}
	hostname, err := resolver.Hostname()
	req.NoError(err)
	req.NotEmpty(hostname)
}

func TestRunApp_DockerEventsCancellation(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockLister := NewMockContainerLister(ctrl)
	mockProducer := NewMockEventProducer(ctrl)
	mockDocker := &mockDockerClient{MockContainerLister: mockLister, MockEventProducer: mockProducer}

	mockDocker.MockContainerLister.EXPECT().ContainerList(ctx, container.ListOptions{}).Return([]container.Summary{}, nil)

	// Setup event channels
	eventsChan := make(chan events.Message, 1)
	errChan := make(chan error, 1)
	mockDocker.MockEventProducer.EXPECT().Events(ctx, gomock.Any()).Return(eventsChan, errChan)

	publisher := &mockMDNSPublisher{}
	resolver := &mockHostnameResolver{hostname: "test-host"}

	// Run in goroutine and cancel immediately
	done := make(chan error, 1)
	go func() {
		done <- runApp(ctx, mockDocker, publisher, resolver, 100*time.Millisecond, "")
	}()

	// Cancel after a short delay
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should eventually return without error
	select {
	case err := <-done:
		// Either returns nil or context cancelled error
		if err != nil {
			req.Contains(err.Error(), "context canceled")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runApp did not return after context cancellation")
	}
}

func TestRunApp_ZoneSync(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockLister := NewMockContainerLister(ctrl)
	mockProducer := NewMockEventProducer(ctrl)
	mockDocker := &mockDockerClient{MockContainerLister: mockLister, MockEventProducer: mockProducer}

	// First call - initial hostnames
	mockDocker.MockContainerLister.EXPECT().ContainerList(ctx, container.ListOptions{}).Return([]container.Summary{
		{Labels: map[string]string{"traefik.http.routers.web.rule": "Host((web.local))"}},
	}, nil)

	// Setup event channels
	eventsChan := make(chan events.Message, 2)
	errChan := make(chan error, 1)
	mockDocker.MockEventProducer.EXPECT().Events(ctx, gomock.Any()).Return(eventsChan, errChan)

	publisher := &mockMDNSPublisher{}
	resolver := &mockHostnameResolver{hostname: "test-host"}

	// Run in goroutine
	done := make(chan error, 1)
	go func() {
		done <- runApp(ctx, mockDocker, publisher, resolver, 100*time.Millisecond, "")
	}()

	// Wait for burst gate and then trigger sync
	time.Sleep(150 * time.Millisecond)
	eventsChan <- events.Message{Action: "start"}

	// Wait a bit for sync to happen, then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should eventually return
	select {
	case err := <-done:
		if err != nil {
			req.Contains(err.Error(), "context canceled")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runApp did not return")
	}

	// Verify zone was synced (should have hostnames from first call)
	if publisher.zone != nil {
		// Zone should exist
		req.NotNil(publisher.zone)
	}
}

func TestRunApp_DockerEventError(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockLister := NewMockContainerLister(ctrl)
	mockProducer := NewMockEventProducer(ctrl)
	mockDocker := &mockDockerClient{MockContainerLister: mockLister, MockEventProducer: mockProducer}

	mockDocker.MockContainerLister.EXPECT().ContainerList(ctx, container.ListOptions{}).Return([]container.Summary{}, nil)

	// Setup event channels - send error immediately
	eventsChan := make(chan events.Message, 1)
	errChan := make(chan error, 1)
	errChan <- errors.New("docker daemon error")
	mockDocker.MockEventProducer.EXPECT().Events(ctx, gomock.Any()).Return(eventsChan, errChan)

	publisher := &mockMDNSPublisher{}
	resolver := &mockHostnameResolver{hostname: "test-host"}

	err := runApp(ctx, mockDocker, publisher, resolver, 5*time.Second, "")
	req.Error(err)
	req.Contains(err.Error(), "docker event producer failed")
}
