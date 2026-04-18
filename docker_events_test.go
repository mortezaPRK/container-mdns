package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDockerEventProducer_HandlerCalled(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEP := NewMockEventProducer(ctrl)

	eventCh := make(chan events.Message)
	errCh := make(chan error)
	mockEP.EXPECT().Events(gomock.Any(), gomock.Any()).Return(eventCh, errCh)

	var called int32
	handler := func() error {
		atomic.AddInt32(&called, 1)
		return nil
	}

	go func() {
		eventCh <- events.Message{}
		close(eventCh)
	}()

	err := dockerEventProducer(t.Context(), mockEP, 10*time.Millisecond, handler)
	req.NoError(err)
	req.Equal(int32(1), atomic.LoadInt32(&called), "handler should be called once")
}

func TestDockerEventProducer_HandlerError(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEP := NewMockEventProducer(ctrl)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	eventCh := make(chan events.Message)
	errCh := make(chan error)
	mockEP.EXPECT().Events(gomock.Any(), gomock.Any()).Return(eventCh, errCh)

	handler := func() error {
		return errors.New("fail handler")
	}

	go func() {
		eventCh <- events.Message{}
		close(eventCh)
	}()

	err := dockerEventProducer(ctx, mockEP, 10*time.Millisecond, handler)
	req.ErrorContains(err, "docker event handler failed")
}

func TestDockerEventProducer_ProducerError(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEP := NewMockEventProducer(ctrl)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	eventCh := make(chan events.Message)
	errCh := make(chan error, 1)
	mockEP.EXPECT().Events(gomock.Any(), gomock.Any()).Return(eventCh, errCh)

	handler := func() error { return nil }

	go func() {
		errCh <- errors.New("fail producer")
		close(errCh)
	}()

	err := dockerEventProducer(ctx, mockEP, 10*time.Millisecond, handler)
	req.ErrorContains(err, "docker event producer failed")
}

func TestDockerEventProducer_ContextCancellation(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEP := NewMockEventProducer(ctrl)
	ctx, cancel := context.WithCancel(t.Context())

	eventCh := make(chan events.Message)
	errCh := make(chan error)
	mockEP.EXPECT().Events(gomock.Any(), gomock.Any()).Return(eventCh, errCh)

	handler := func() error { return nil }

	// Cancel context immediately
	cancel()

	err := dockerEventProducer(ctx, mockEP, 10*time.Millisecond, handler)
	req.NoError(err)
}

func TestDockerEventProducer_EventChannelClosed(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEP := NewMockEventProducer(ctrl)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	eventCh := make(chan events.Message)
	errCh := make(chan error)
	mockEP.EXPECT().Events(gomock.Any(), gomock.Any()).Return(eventCh, errCh)

	handler := func() error { return nil }

	// Close event channel immediately
	close(eventCh)

	err := dockerEventProducer(ctx, mockEP, 10*time.Millisecond, handler)
	req.NoError(err)
}

func TestDockerEventProducer_ErrorChannelClosed(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEP := NewMockEventProducer(ctrl)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	eventCh := make(chan events.Message)
	errCh := make(chan error)
	mockEP.EXPECT().Events(gomock.Any(), gomock.Any()).Return(eventCh, errCh)

	handler := func() error { return nil }

	// Close error channel immediately
	close(errCh)

	err := dockerEventProducer(ctx, mockEP, 10*time.Millisecond, handler)
	req.NoError(err)
}

func TestDockerEventProducer_MultipleErrors(t *testing.T) {
	req := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEP := NewMockEventProducer(ctrl)
	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()

	eventCh := make(chan events.Message)
	errCh := make(chan error, 1)
	mockEP.EXPECT().Events(gomock.Any(), gomock.Any()).Return(eventCh, errCh)

	handler := func() error {
		return errors.New("handler error")
	}

	go func() {
		eventCh <- events.Message{}
		errCh <- errors.New("producer error")
		close(eventCh)
		close(errCh)
	}()

	err := dockerEventProducer(ctx, mockEP, 10*time.Millisecond, handler)
	req.Error(err)
	// The producer error comes first and cancels context, so handler may not run
	req.ErrorContains(err, "docker event producer failed")
}
