package common

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type StreamEndReason string

const (
	StreamEndReasonNone        StreamEndReason = ""
	StreamEndReasonDone        StreamEndReason = "done"
	StreamEndReasonTimeout     StreamEndReason = "timeout"
	StreamEndReasonClientGone  StreamEndReason = "client_gone"
	StreamEndReasonScannerErr  StreamEndReason = "scanner_error"
	StreamEndReasonHandlerStop StreamEndReason = "handler_stop"
	StreamEndReasonMappedError StreamEndReason = "mapped_error"
	StreamEndReasonEOF         StreamEndReason = "eof"
	StreamEndReasonPanic       StreamEndReason = "panic"
	StreamEndReasonPingFail    StreamEndReason = "ping_fail"
)

type StreamFailureCategory string

const (
	StreamFailureCategoryNone                  StreamFailureCategory = "none"
	StreamFailureCategoryClientDisconnected    StreamFailureCategory = "client_disconnected"
	StreamFailureCategoryUpstreamTimeout       StreamFailureCategory = "upstream_timeout"
	StreamFailureCategoryUpstreamStreamError   StreamFailureCategory = "upstream_stream_error"
	StreamFailureCategoryUpstreamMappedError   StreamFailureCategory = "upstream_mapped_error"
	StreamFailureCategoryStreamHandlerError    StreamFailureCategory = "stream_handler_error"
	StreamFailureCategoryDownstreamWriteFailed StreamFailureCategory = "downstream_write_failed"
	StreamFailureCategoryInternalPanic         StreamFailureCategory = "internal_panic"
	StreamFailureCategoryUnknown               StreamFailureCategory = "unknown"
)

type StreamFailureSource string

const (
	StreamFailureSourceNone     StreamFailureSource = "none"
	StreamFailureSourceClient   StreamFailureSource = "client"
	StreamFailureSourceUpstream StreamFailureSource = "upstream"
	StreamFailureSourceProxy    StreamFailureSource = "proxy"
	StreamFailureSourceUnknown  StreamFailureSource = "unknown"
)

type StreamFailureStage string

const (
	StreamFailureStageNone                StreamFailureStage = "none"
	StreamFailureStageBeforeFirstResponse StreamFailureStage = "before_first_response"
	StreamFailureStageAfterFirstResponse  StreamFailureStage = "after_first_response"
)

const maxStreamErrorEntries = 20

type StreamErrorEntry struct {
	Message   string
	Timestamp time.Time
}

type StreamFailureClassification struct {
	Category                StreamFailureCategory
	Source                  StreamFailureSource
	Stage                   StreamFailureStage
	ChannelFailureCandidate bool
}

type StreamStatus struct {
	EndReason StreamEndReason
	EndError  error
	endOnce   sync.Once

	mu         sync.Mutex
	Errors     []StreamErrorEntry
	ErrorCount int

	MappedErrorCode               string
	MappedErrorStatusCode         int
	MappedErrorMessage            string
	MappedErrorRuleName           string
	MappedChannelFailureCandidate *bool
}

func NewStreamStatus() *StreamStatus {
	return &StreamStatus{}
}

func (s *StreamStatus) SetEndReason(reason StreamEndReason, err error) {
	if s == nil {
		return
	}
	s.endOnce.Do(func() {
		s.EndReason = reason
		s.EndError = err
	})
}

func (s *StreamStatus) RecordError(msg string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ErrorCount++
	if len(s.Errors) < maxStreamErrorEntries {
		s.Errors = append(s.Errors, StreamErrorEntry{
			Message:   msg,
			Timestamp: time.Now(),
		})
	}
}

func (s *StreamStatus) SetMappedError(statusCode int, errorCode string, message string, ruleName string, channelFailureCandidate bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.MappedErrorStatusCode = statusCode
	s.MappedErrorCode = errorCode
	s.MappedErrorMessage = message
	s.MappedErrorRuleName = ruleName
	s.MappedChannelFailureCandidate = &channelFailureCandidate
	s.mu.Unlock()
}

func (s *StreamStatus) HasErrors() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ErrorCount > 0
}

func (s *StreamStatus) TotalErrorCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ErrorCount
}

func (s *StreamStatus) ErrorMessages() ([]string, int) {
	if s == nil {
		return nil, 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	messages := make([]string, 0, len(s.Errors))
	for _, e := range s.Errors {
		messages = append(messages, e.Message)
	}
	return messages, s.ErrorCount
}

func (s *StreamStatus) IsNormalEnd() bool {
	if s == nil {
		return true
	}
	return s.EndReason == StreamEndReasonDone ||
		s.EndReason == StreamEndReasonEOF ||
		s.EndReason == StreamEndReasonHandlerStop
}

func (s *StreamStatus) ClassifyFailure(hasFirstResponse bool) StreamFailureClassification {
	category := s.FailureCategory()
	source := streamFailureSource(category)
	stage := StreamFailureStageNone
	if category != StreamFailureCategoryNone {
		if hasFirstResponse {
			stage = StreamFailureStageAfterFirstResponse
		} else {
			stage = StreamFailureStageBeforeFirstResponse
		}
	}
	return StreamFailureClassification{
		Category:                category,
		Source:                  source,
		Stage:                   stage,
		ChannelFailureCandidate: s.channelFailureCandidate(source),
	}
}

func (s *StreamStatus) channelFailureCandidate(source StreamFailureSource) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	override := s.MappedChannelFailureCandidate
	s.mu.Unlock()
	if override != nil {
		return *override
	}
	return source == StreamFailureSourceUpstream
}

func (s *StreamStatus) FailureCategory() StreamFailureCategory {
	if s == nil {
		return StreamFailureCategoryNone
	}
	hasErrors := s.HasErrors()
	switch s.EndReason {
	case StreamEndReasonDone, StreamEndReasonEOF:
		if hasErrors {
			return StreamFailureCategoryStreamHandlerError
		}
		return StreamFailureCategoryNone
	case StreamEndReasonHandlerStop:
		if hasErrors || s.EndError != nil {
			return StreamFailureCategoryStreamHandlerError
		}
		return StreamFailureCategoryNone
	case StreamEndReasonClientGone:
		return StreamFailureCategoryClientDisconnected
	case StreamEndReasonTimeout:
		return StreamFailureCategoryUpstreamTimeout
	case StreamEndReasonMappedError:
		return StreamFailureCategoryUpstreamMappedError
	case StreamEndReasonScannerErr:
		return StreamFailureCategoryUpstreamStreamError
	case StreamEndReasonPingFail:
		return StreamFailureCategoryDownstreamWriteFailed
	case StreamEndReasonPanic:
		return StreamFailureCategoryInternalPanic
	case StreamEndReasonNone:
		if hasErrors || s.EndError != nil {
			return StreamFailureCategoryStreamHandlerError
		}
		return StreamFailureCategoryUnknown
	default:
		if hasErrors || s.EndError != nil {
			return StreamFailureCategoryUnknown
		}
		return StreamFailureCategoryNone
	}
}

func streamFailureSource(category StreamFailureCategory) StreamFailureSource {
	switch category {
	case StreamFailureCategoryNone:
		return StreamFailureSourceNone
	case StreamFailureCategoryClientDisconnected, StreamFailureCategoryDownstreamWriteFailed:
		return StreamFailureSourceClient
	case StreamFailureCategoryUpstreamTimeout, StreamFailureCategoryUpstreamStreamError, StreamFailureCategoryUpstreamMappedError:
		return StreamFailureSourceUpstream
	case StreamFailureCategoryStreamHandlerError, StreamFailureCategoryInternalPanic:
		return StreamFailureSourceProxy
	default:
		return StreamFailureSourceUnknown
	}
}

func (s *StreamStatus) Summary() string {
	if s == nil {
		return "StreamStatus<nil>"
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "reason=%s", s.EndReason)
	if s.EndError != nil {
		fmt.Fprintf(b, " end_error=%q", s.EndError.Error())
	}
	s.mu.Lock()
	if s.ErrorCount > 0 {
		fmt.Fprintf(b, " soft_errors=%d", s.ErrorCount)
	}
	s.mu.Unlock()
	return b.String()
}
