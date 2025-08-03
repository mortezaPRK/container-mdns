package main

import (
	"context"
	"regexp"

	"github.com/docker/docker/api/types/container"
)

var (
	routerLabelRegex = regexp.MustCompile(`^traefik\.http\.routers\.(.*)\.rule$`)
	hostInValueRegex = regexp.MustCompile(`Host\(\(([^)]+)\)\)`)
)

// getHostnameFromLabel extracts the hostname from a container's labels.
func getHostnameFromLabel(c container.Summary) (string, bool) {
	for k, v := range c.Labels {
		if routerLabelRegex.MatchString(k) {
			if m := hostInValueRegex.FindStringSubmatch(v); len(m) > 1 {
				return m[1], true
			}
			return "", false
		}
	}
	return "", false
}

// getHostnames uses a ContainerLister to get all hostnames from running containers.
func getHostnames(ctx context.Context, lister ContainerLister) ([]string, error) {
	containers, err := lister.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(containers))
	for _, c := range containers {
		if h, ok := getHostnameFromLabel(c); ok {
			result = append(result, h)
		}
	}

	return result, nil
}
