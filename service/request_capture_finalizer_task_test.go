package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestCaptureFinalizerTaskOptionsFromEnv(t *testing.T) {
	t.Setenv("CAPTURE_FINALIZER_INTERVAL_SECONDS", "12")
	t.Setenv("CAPTURE_FINALIZER_LIMIT", "34")
	t.Setenv("CAPTURE_FINALIZER_REMOVE_ON_SUCCESS", "false")
	t.Setenv("CAPTURE_SPOOL_DIR", "/capture/spool")

	assert.Equal(t, 12, RequestCaptureFinalizerIntervalSeconds())
	assert.Equal(t, 34, RequestCaptureFinalizerLimit())

	options := RequestCaptureFinalizerWorkerOptionsFromEnv()
	assert.Equal(t, "/capture/spool", options.SpoolDir)
	assert.Equal(t, 34, options.Limit)
	assert.False(t, options.RemoveOnSuccess)
}

func TestRequestCaptureFinalizerTaskEnabledFollowsCaptureStorage(t *testing.T) {
	assert.False(t, RequestCaptureFinalizerTaskEnabled())

	t.Setenv("CAPTURE_ENABLED", "true")
	t.Setenv("CAPTURE_OBJECT_BACKEND", "s3")
	assert.True(t, RequestCaptureFinalizerTaskEnabled())

	t.Setenv("CAPTURE_FINALIZER_ENABLED", "false")
	assert.False(t, RequestCaptureFinalizerTaskEnabled())
}
