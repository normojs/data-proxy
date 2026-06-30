package common

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamStatus_SetEndReason_FirstWins(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	s.SetEndReason(StreamEndReasonDone, nil)
	s.SetEndReason(StreamEndReasonTimeout, nil)
	s.SetEndReason(StreamEndReasonClientGone, fmt.Errorf("context canceled"))

	assert.Equal(t, StreamEndReasonDone, s.EndReason)
	assert.Nil(t, s.EndError)
}

func TestStreamStatus_SetEndReason_WithError(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	expectedErr := fmt.Errorf("read: connection reset")
	s.SetEndReason(StreamEndReasonScannerErr, expectedErr)

	assert.Equal(t, StreamEndReasonScannerErr, s.EndReason)
	assert.Equal(t, expectedErr, s.EndError)
}

func TestStreamStatus_SetEndReason_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	s.SetEndReason(StreamEndReasonDone, nil)
}

func TestStreamStatus_SetEndReason_Concurrent(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	reasons := []StreamEndReason{
		StreamEndReasonDone,
		StreamEndReasonTimeout,
		StreamEndReasonClientGone,
		StreamEndReasonScannerErr,
		StreamEndReasonHandlerStop,
		StreamEndReasonEOF,
		StreamEndReasonPanic,
		StreamEndReasonPingFail,
	}

	var wg sync.WaitGroup
	for _, r := range reasons {
		wg.Add(1)
		go func(reason StreamEndReason) {
			defer wg.Done()
			s.SetEndReason(reason, nil)
		}(r)
	}
	wg.Wait()

	assert.NotEqual(t, StreamEndReasonNone, s.EndReason)
}

func TestStreamStatus_RecordError_Basic(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	s.RecordError("bad json")
	s.RecordError("another bad json")
	s.RecordError("client gone")

	assert.True(t, s.HasErrors())
	assert.Equal(t, 3, s.TotalErrorCount())
	assert.Len(t, s.Errors, 3)
}

func TestStreamStatus_RecordError_CapAtMax(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	for i := 0; i < 30; i++ {
		s.RecordError(fmt.Sprintf("error_%d", i))
	}

	assert.Equal(t, maxStreamErrorEntries, len(s.Errors))
	assert.Equal(t, 30, s.TotalErrorCount())
}

func TestStreamStatus_RecordError_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	s.RecordError("should not panic")
}

func TestStreamStatus_RecordError_Concurrent(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s.RecordError(fmt.Sprintf("error_%d", idx))
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 100, s.TotalErrorCount())
	assert.LessOrEqual(t, len(s.Errors), maxStreamErrorEntries)
}

func TestStreamStatus_ErrorMessages(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	s.RecordError("bad json")
	s.RecordError("write failed")

	messages, count := s.ErrorMessages()
	assert.Equal(t, 2, count)
	assert.Equal(t, []string{"bad json", "write failed"}, messages)
}

func TestStreamStatus_HasErrors_Empty(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()
	assert.False(t, s.HasErrors())
	assert.Equal(t, 0, s.TotalErrorCount())
}

func TestStreamStatus_HasErrors_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	assert.False(t, s.HasErrors())
	assert.Equal(t, 0, s.TotalErrorCount())
}

func TestStreamStatus_IsNormalEnd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		reason StreamEndReason
		normal bool
	}{
		{StreamEndReasonDone, true},
		{StreamEndReasonEOF, true},
		{StreamEndReasonHandlerStop, true},
		{StreamEndReasonTimeout, false},
		{StreamEndReasonClientGone, false},
		{StreamEndReasonScannerErr, false},
		{StreamEndReasonPanic, false},
		{StreamEndReasonPingFail, false},
		{StreamEndReasonNone, false},
	}
	for _, tt := range tests {
		s := NewStreamStatus()
		s.SetEndReason(tt.reason, nil)
		assert.Equal(t, tt.normal, s.IsNormalEnd(), "reason=%s", tt.reason)
	}
}

func TestStreamStatus_IsNormalEnd_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	assert.True(t, s.IsNormalEnd())
}

func TestStreamStatus_ClassifyFailure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		reason             StreamEndReason
		recordError        bool
		endErr             error
		hasFirstResponse   bool
		wantCategory       StreamFailureCategory
		wantSource         StreamFailureSource
		wantStage          StreamFailureStage
		wantChannelFailure bool
	}{
		{
			name:         "done without soft errors",
			reason:       StreamEndReasonDone,
			wantCategory: StreamFailureCategoryNone,
			wantSource:   StreamFailureSourceNone,
			wantStage:    StreamFailureStageNone,
		},
		{
			name:               "done with soft errors after first response",
			reason:             StreamEndReasonDone,
			recordError:        true,
			hasFirstResponse:   true,
			wantCategory:       StreamFailureCategoryStreamHandlerError,
			wantSource:         StreamFailureSourceProxy,
			wantStage:          StreamFailureStageAfterFirstResponse,
			wantChannelFailure: false,
		},
		{
			name:         "client disconnected before first response",
			reason:       StreamEndReasonClientGone,
			endErr:       fmt.Errorf("context canceled"),
			wantCategory: StreamFailureCategoryClientDisconnected,
			wantSource:   StreamFailureSourceClient,
			wantStage:    StreamFailureStageBeforeFirstResponse,
		},
		{
			name:               "upstream timeout after first response",
			reason:             StreamEndReasonTimeout,
			hasFirstResponse:   true,
			wantCategory:       StreamFailureCategoryUpstreamTimeout,
			wantSource:         StreamFailureSourceUpstream,
			wantStage:          StreamFailureStageAfterFirstResponse,
			wantChannelFailure: true,
		},
		{
			name:               "scanner error is upstream stream error",
			reason:             StreamEndReasonScannerErr,
			endErr:             fmt.Errorf("unexpected EOF"),
			wantCategory:       StreamFailureCategoryUpstreamStreamError,
			wantSource:         StreamFailureSourceUpstream,
			wantStage:          StreamFailureStageBeforeFirstResponse,
			wantChannelFailure: true,
		},
		{
			name:               "mapped error is upstream mapped error",
			reason:             StreamEndReasonMappedError,
			endErr:             fmt.Errorf("mapped upstream stream error"),
			wantCategory:       StreamFailureCategoryUpstreamMappedError,
			wantSource:         StreamFailureSourceUpstream,
			wantStage:          StreamFailureStageBeforeFirstResponse,
			wantChannelFailure: true,
		},
		{
			name:         "ping fail is downstream write failure",
			reason:       StreamEndReasonPingFail,
			endErr:       fmt.Errorf("write ping data failed"),
			wantCategory: StreamFailureCategoryDownstreamWriteFailed,
			wantSource:   StreamFailureSourceClient,
			wantStage:    StreamFailureStageBeforeFirstResponse,
		},
		{
			name:         "panic is proxy failure",
			reason:       StreamEndReasonPanic,
			endErr:       fmt.Errorf("handler panic"),
			wantCategory: StreamFailureCategoryInternalPanic,
			wantSource:   StreamFailureSourceProxy,
			wantStage:    StreamFailureStageBeforeFirstResponse,
		},
		{
			name:         "no end reason is unknown",
			reason:       StreamEndReasonNone,
			wantCategory: StreamFailureCategoryUnknown,
			wantSource:   StreamFailureSourceUnknown,
			wantStage:    StreamFailureStageBeforeFirstResponse,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := NewStreamStatus()
			if tt.recordError {
				s.RecordError("soft error")
			}
			s.SetEndReason(tt.reason, tt.endErr)

			got := s.ClassifyFailure(tt.hasFirstResponse)

			assert.Equal(t, tt.wantCategory, got.Category)
			assert.Equal(t, tt.wantSource, got.Source)
			assert.Equal(t, tt.wantStage, got.Stage)
			assert.Equal(t, tt.wantChannelFailure, got.ChannelFailureCandidate)
		})
	}
}

func TestStreamStatus_MappedErrorChannelFailureOverride(t *testing.T) {
	t.Parallel()

	s := NewStreamStatus()
	s.SetMappedError(429, "upstream_key_sleeping", "sleeping", "sleeping-key", false)
	s.SetEndReason(StreamEndReasonMappedError, fmt.Errorf("sleeping"))

	got := s.ClassifyFailure(false)

	assert.Equal(t, StreamFailureCategoryUpstreamMappedError, got.Category)
	assert.Equal(t, StreamFailureSourceUpstream, got.Source)
	assert.False(t, got.ChannelFailureCandidate)
	assert.Equal(t, 429, s.MappedErrorStatusCode)
	assert.Equal(t, "upstream_key_sleeping", s.MappedErrorCode)
	assert.Equal(t, "sleeping", s.MappedErrorMessage)
	assert.Equal(t, "sleeping-key", s.MappedErrorRuleName)
}

func TestStreamStatus_Summary(t *testing.T) {
	t.Parallel()

	s := NewStreamStatus()
	s.SetEndReason(StreamEndReasonDone, nil)
	summary := s.Summary()
	assert.Contains(t, summary, "reason=done")
	assert.NotContains(t, summary, "soft_errors")

	s2 := NewStreamStatus()
	s2.SetEndReason(StreamEndReasonTimeout, nil)
	s2.RecordError("bad json")
	s2.RecordError("write failed")
	summary2 := s2.Summary()
	assert.Contains(t, summary2, "reason=timeout")
	assert.Contains(t, summary2, "soft_errors=2")
}

func TestStreamStatus_Summary_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	assert.Equal(t, "StreamStatus<nil>", s.Summary())
}
