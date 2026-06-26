package service

import (
	"sync"
	"time"
)

const (
	requestCaptureAsyncWriterBufferChunks    = 256
	requestCaptureAsyncWriterFlushBytes      = 64 * 1024
	requestCaptureAsyncWriterFlushEvery      = 200 * time.Millisecond
	requestCaptureAsyncWriterMaxPendingBytes = 8 * 1024 * 1024

	requestCaptureAsyncWriterTruncationBackpressure      = "backpressure"
	requestCaptureAsyncWriterTruncationPendingBytesLimit = "pending_bytes_limit"
)

type requestCaptureAsyncArtifactWriter struct {
	session     *RequestCaptureSession
	name        string
	contentType string
	chunks      chan []byte
	done        chan struct{}
	closeOnce   sync.Once

	sendMu         sync.Mutex
	closed         bool
	pendingBytes   int64
	mu             sync.Mutex
	err            error
	truncated      bool
	truncateReason string
}

func newRequestCaptureAsyncArtifactWriter(session *RequestCaptureSession, name string, contentType string) *requestCaptureAsyncArtifactWriter {
	writer := &requestCaptureAsyncArtifactWriter{
		session:     session,
		name:        name,
		contentType: contentType,
		chunks:      make(chan []byte, requestCaptureAsyncWriterBufferChunks),
		done:        make(chan struct{}),
	}
	go writer.run()
	return writer
}

func (w *requestCaptureAsyncArtifactWriter) Append(chunk []byte) {
	if w == nil || len(chunk) == 0 || w.isTruncated() {
		return
	}
	w.sendMu.Lock()
	defer w.sendMu.Unlock()
	if w.closed || w.isTruncated() {
		return
	}
	if int64(len(chunk)) > requestCaptureAsyncWriterMaxPendingBytes || w.pendingBytes+int64(len(chunk)) > requestCaptureAsyncWriterMaxPendingBytes {
		w.setTruncated(requestCaptureAsyncWriterTruncationPendingBytesLimit)
		return
	}
	copied := append([]byte(nil), chunk...)
	select {
	case w.chunks <- copied:
		w.pendingBytes += int64(len(copied))
	default:
		w.setTruncated(requestCaptureAsyncWriterTruncationBackpressure)
	}
}

func (w *requestCaptureAsyncArtifactWriter) Close() error {
	if w == nil {
		return nil
	}
	w.closeOnce.Do(func() {
		w.sendMu.Lock()
		w.closed = true
		close(w.chunks)
		w.sendMu.Unlock()
		<-w.done
	})
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}

func (w *requestCaptureAsyncArtifactWriter) run() {
	defer close(w.done)
	ticker := time.NewTicker(requestCaptureAsyncWriterFlushEvery)
	defer ticker.Stop()

	buffer := make([]byte, 0, requestCaptureAsyncWriterFlushBytes)
	flush := func() {
		if len(buffer) == 0 {
			return
		}
		if err := w.session.AppendArtifact(w.name, w.contentType, buffer); err != nil {
			w.setError(err)
		}
		buffer = buffer[:0]
	}

	for {
		select {
		case chunk, ok := <-w.chunks:
			if !ok {
				flush()
				if w.isTruncated() {
					if err := w.session.MarkArtifactTruncated(w.name, w.contentType); err != nil {
						w.setError(err)
					}
				}
				return
			}
			if len(chunk) == 0 {
				continue
			}
			w.addPendingBytes(-int64(len(chunk)))
			buffer = append(buffer, chunk...)
			if len(buffer) >= requestCaptureAsyncWriterFlushBytes {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (w *requestCaptureAsyncArtifactWriter) setError(err error) {
	if err == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err == nil {
		w.err = err
	}
}

func (w *requestCaptureAsyncArtifactWriter) setTruncated(reason string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.truncated = true
	if w.truncateReason == "" {
		w.truncateReason = reason
	}
}

func (w *requestCaptureAsyncArtifactWriter) isTruncated() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.truncated
}

func (w *requestCaptureAsyncArtifactWriter) TruncationReason() string {
	if w == nil {
		return ""
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.truncateReason
}

func (w *requestCaptureAsyncArtifactWriter) addPendingBytes(delta int64) {
	w.sendMu.Lock()
	defer w.sendMu.Unlock()
	w.pendingBytes += delta
	if w.pendingBytes < 0 {
		w.pendingBytes = 0
	}
}
