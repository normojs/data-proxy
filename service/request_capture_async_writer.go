package service

import (
	"sync"
	"time"
)

const (
	requestCaptureAsyncWriterBufferChunks = 256
	requestCaptureAsyncWriterFlushBytes   = 64 * 1024
	requestCaptureAsyncWriterFlushEvery   = 200 * time.Millisecond
)

type requestCaptureAsyncArtifactWriter struct {
	session     *RequestCaptureSession
	name        string
	contentType string
	chunks      chan []byte
	done        chan struct{}
	closeOnce   sync.Once

	mu        sync.Mutex
	err       error
	truncated bool
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
	copied := append([]byte(nil), chunk...)
	select {
	case w.chunks <- copied:
	default:
		w.setTruncated()
	}
}

func (w *requestCaptureAsyncArtifactWriter) Close() error {
	if w == nil {
		return nil
	}
	w.closeOnce.Do(func() {
		close(w.chunks)
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

func (w *requestCaptureAsyncArtifactWriter) setTruncated() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.truncated = true
}

func (w *requestCaptureAsyncArtifactWriter) isTruncated() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.truncated
}
