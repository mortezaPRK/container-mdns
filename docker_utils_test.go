package main

import (
	"errors"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetHostnameFromLabel(t *testing.T) {
	req := require.New(t)
	c := container.Summary{
		Labels: map[string]string{
			"traefik.http.routers.foo.rule": "Host((foo.local))",
		},
	}
	h, ok := getHostnameFromLabel(c)
	req.True(ok)
	req.Equal("foo.local", h)

	c = container.Summary{Labels: map[string]string{"other": "value"}}
	h, ok = getHostnameFromLabel(c)
	req.False(ok)
	req.Equal("", h)
}

func TestGetHostnameFromLabel_InvalidHostFormat(t *testing.T) {
	req := require.New(t)
	c := container.Summary{
		Labels: map[string]string{
			"traefik.http.routers.foo.rule": "Host(foo.local)", // Missing double parentheses
		},
	}
	h, ok := getHostnameFromLabel(c)
	req.False(ok)
	req.Equal("", h)
}

func TestGetHostnameFromLabel_EmptyHost(t *testing.T) {
	req := require.New(t)
	c := container.Summary{
		Labels: map[string]string{
			"traefik.http.routers.foo.rule": "Host(())", // Empty hostname
		},
	}
	h, ok := getHostnameFromLabel(c)
	req.False(ok)
	req.Equal("", h)
}

func TestGetHostnameFromLabel_MultipleLabels(t *testing.T) {
	req := require.New(t)
	c := container.Summary{
		Labels: map[string]string{
			"other.label":                   "value",
			"traefik.http.routers.foo.rule": "Host((foo.local))",
			"another.label":                 "another-value",
		},
	}
	h, ok := getHostnameFromLabel(c)
	req.True(ok)
	req.Equal("foo.local", h)
}

func TestGetHostnameFromLabel_NoLabels(t *testing.T) {
	req := require.New(t)
	c := container.Summary{
		Labels: map[string]string{},
	}
	h, ok := getHostnameFromLabel(c)
	req.False(ok)
	req.Equal("", h)
}

func TestGetHostnames(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLister := NewMockContainerLister(ctrl)
	mockLister.EXPECT().ContainerList(t.Context(), container.ListOptions{}).Return([]container.Summary{
		{Labels: map[string]string{"traefik.http.routers.foo.rule": "Host((foo.local))"}},
		{Labels: map[string]string{"traefik.http.routers.bar.rule": "Host((bar.local))"}},
	}, nil)

	hosts, err := getHostnames(t.Context(), mockLister)
	req.NoError(err)
	req.ElementsMatch([]string{"foo.local", "bar.local"}, hosts)

	// Error case
	mockLister2 := NewMockContainerLister(ctrl)
	mockLister2.EXPECT().ContainerList(t.Context(), container.ListOptions{}).Return(nil, errors.New("fail"))
	_, err = getHostnames(t.Context(), mockLister2)
	req.Error(err)
}

func TestGetHostnames_EmptyContainers(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLister := NewMockContainerLister(ctrl)
	mockLister.EXPECT().ContainerList(t.Context(), container.ListOptions{}).Return([]container.Summary{}, nil)

	hosts, err := getHostnames(t.Context(), mockLister)
	req.NoError(err)
	req.Empty(hosts)
}

func TestGetHostnames_MixedValidInvalid(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockLister := NewMockContainerLister(ctrl)
	mockLister.EXPECT().ContainerList(t.Context(), container.ListOptions{}).Return([]container.Summary{
		{Labels: map[string]string{"traefik.http.routers.foo.rule": "Host((foo.local))"}},
		{Labels: map[string]string{"other.label": "value"}}, // No valid hostname
		{Labels: map[string]string{"traefik.http.routers.bar.rule": "Host((bar.local))"}},
		{Labels: map[string]string{"traefik.http.routers.baz.rule": "Host(baz.local)"}}, // Invalid format
	}, nil)

	hosts, err := getHostnames(t.Context(), mockLister)
	req.NoError(err)
	req.ElementsMatch([]string{"foo.local", "bar.local"}, hosts)
}
