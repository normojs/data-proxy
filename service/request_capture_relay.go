package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type RelayRequestCapture struct {
	mu                    sync.Mutex
	Decision              RequestCaptureDecision
	Policy                RequestCapturePolicy
	Session               *RequestCaptureSession
	RecordId              int64
	IsStream              bool
	UpstreamBodyBytes     int64
	DownstreamBodyBytes   int64
	upstreamWriter        *requestCaptureAsyncArtifactWriter
	upstreamArtifact      string
	upstreamContentType   string
	downstreamWriter      *requestCaptureAsyncArtifactWriter
	downstreamArtifact    string
	downstreamContentType string
}

const relayRequestCaptureContextKey = "request_capture"

type relayRequestCaptureResponseWriter struct {
	gin.ResponseWriter
	capture *RelayRequestCapture
}

type relayRequestCaptureReadCloser struct {
	io.ReadCloser
	capture     *RelayRequestCapture
	contentType string
}

func StartRelayRequestCapture(c *gin.Context, info *relaycommon.RelayInfo) *RelayRequestCapture {
	if c == nil || info == nil {
		return nil
	}
	policy := LoadRequestCapturePolicyFromEnv()
	decision := policy.Decide(RequestCaptureDecisionInput{
		RequestId:      info.RequestId,
		UserId:         info.UserId,
		TokenId:        info.TokenId,
		ChannelId:      common.GetContextKeyInt(c, constant.ContextKeyChannelId),
		ConnectedAppId: int64(c.GetInt("connected_app_id")),
		ModelName:      info.OriginModelName,
		RequestPath:    c.Request.URL.Path,
		ProtocolChain:  relayRequestCaptureProtocolChain(info),
		IsStream:       info.IsStream,
	})
	if !decision.Enabled {
		return nil
	}
	capture := &RelayRequestCapture{
		Decision: decision,
		Policy:   policy,
		IsStream: info.IsStream,
	}
	if decision.Level != model.RequestCaptureLevelMetadata {
		session, err := NewRequestCaptureSession(RequestCaptureSessionOptions{
			RequestId:        info.RequestId,
			CaptureLevel:     decision.Level,
			SpoolDir:         policy.SpoolDir,
			ModelName:        info.OriginModelName,
			RequestPath:      c.Request.URL.Path,
			ProtocolChain:    relayRequestCaptureProtocolChain(info),
			IsStream:         info.IsStream,
			MaxArtifactBytes: policy.MaxArtifactBytes,
		})
		if err != nil {
			relayRequestCaptureLogError("create capture session", info.RequestId, err)
		} else {
			capture.Session = session
			if err := relayRequestCaptureClientRequest(c, session); err != nil {
				relayRequestCaptureLogError("capture client request", info.RequestId, err)
			}
		}
	}
	capture.RecordId = relayRequestCaptureCreateRecord(c, info, capture)
	c.Set(relayRequestCaptureContextKey, capture)
	return capture
}

func GetRelayRequestCapture(c *gin.Context) *RelayRequestCapture {
	if c == nil {
		return nil
	}
	value, ok := c.Get(relayRequestCaptureContextKey)
	if !ok || value == nil {
		return nil
	}
	capture, _ := value.(*RelayRequestCapture)
	return capture
}

func WrapUpstreamResponseForCapture(c *gin.Context, resp *http.Response) *http.Response {
	if resp == nil || resp.Body == nil {
		return resp
	}
	capture := GetRelayRequestCapture(c)
	if capture == nil || capture.Session == nil {
		return resp
	}
	resp.Body = &relayRequestCaptureReadCloser{
		ReadCloser:  resp.Body,
		capture:     capture,
		contentType: resp.Header.Get("Content-Type"),
	}
	return resp
}

func NewRelayRequestCaptureResponseWriter(writer gin.ResponseWriter, capture *RelayRequestCapture) gin.ResponseWriter {
	if writer == nil || capture == nil || capture.Session == nil {
		return writer
	}
	return &relayRequestCaptureResponseWriter{
		ResponseWriter: writer,
		capture:        capture,
	}
}

func (w *relayRequestCaptureResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if n > 0 {
		w.capture.AppendDownstreamResponse(w.Header().Get("Content-Type"), data[:n])
	}
	return n, err
}

func (w *relayRequestCaptureResponseWriter) WriteString(data string) (int, error) {
	n, err := w.ResponseWriter.WriteString(data)
	if n > 0 {
		w.capture.AppendDownstreamResponse(w.Header().Get("Content-Type"), []byte(data[:n]))
	}
	return n, err
}

func (r *relayRequestCaptureReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.capture.AppendUpstreamResponse(r.contentType, p[:n])
	}
	return n, err
}

func (capture *RelayRequestCapture) AppendUpstreamResponse(contentType string, data []byte) {
	if capture == nil || capture.Session == nil || len(data) == 0 {
		return
	}
	capture.mu.Lock()
	if capture.upstreamArtifact == "" {
		capture.upstreamContentType = relayRequestCaptureContentType(contentType)
		capture.upstreamArtifact = relayRequestCaptureUpstreamArtifactName(capture.IsStream, capture.upstreamContentType)
	}
	capture.UpstreamBodyBytes += int64(len(data))
	if capture.IsStream {
		if capture.upstreamWriter == nil {
			capture.upstreamWriter = newRequestCaptureAsyncArtifactWriter(capture.Session, capture.upstreamArtifact, capture.upstreamContentType)
		}
		writer := capture.upstreamWriter
		capture.mu.Unlock()
		writer.Append(data)
		return
	}
	artifact := capture.upstreamArtifact
	capturedContentType := capture.upstreamContentType
	capture.mu.Unlock()
	if err := capture.Session.AppendArtifact(artifact, capturedContentType, data); err != nil {
		relayRequestCaptureLogError("append upstream response", "", err)
		return
	}
}

func (capture *RelayRequestCapture) AppendDownstreamResponse(contentType string, data []byte) {
	if capture == nil || capture.Session == nil || len(data) == 0 {
		return
	}
	capture.mu.Lock()
	if capture.downstreamArtifact == "" {
		capture.downstreamContentType = relayRequestCaptureContentType(contentType)
		capture.downstreamArtifact = relayRequestCaptureDownstreamArtifactName(capture.IsStream, capture.downstreamContentType)
	}
	capture.DownstreamBodyBytes += int64(len(data))
	if capture.IsStream {
		if capture.downstreamWriter == nil {
			capture.downstreamWriter = newRequestCaptureAsyncArtifactWriter(capture.Session, capture.downstreamArtifact, capture.downstreamContentType)
		}
		writer := capture.downstreamWriter
		capture.mu.Unlock()
		writer.Append(data)
		return
	}
	artifact := capture.downstreamArtifact
	capturedContentType := capture.downstreamContentType
	capture.mu.Unlock()
	if err := capture.Session.AppendArtifact(artifact, capturedContentType, data); err != nil {
		relayRequestCaptureLogError("append downstream response", "", err)
		return
	}
}

func FinishRelayRequestCapture(capture *RelayRequestCapture, err error) {
	if capture == nil {
		return
	}
	if writerErr := capture.closeWriters(); err == nil && writerErr != nil {
		err = writerErr
	}
	now := common.GetTimestamp()
	status := model.RequestCaptureStatusUploaded
	lastError := ""
	if err != nil {
		status = model.RequestCaptureStatusFailed
		lastError = err.Error()
		if capture.Session != nil {
			if failErr := capture.Session.Fail(err); failErr != nil {
				relayRequestCaptureLogError("fail capture session", "", failErr)
			}
		}
	} else if capture.Session != nil {
		status = model.RequestCaptureStatusFinalizing
		if finishErr := capture.Session.Finish(); finishErr != nil {
			status = model.RequestCaptureStatusFailed
			lastError = finishErr.Error()
			relayRequestCaptureLogError("finish capture session", "", finishErr)
		}
	}
	if model.DB == nil || capture.RecordId == 0 {
		return
	}
	updates := map[string]interface{}{
		"capture_status":        status,
		"finished_at":           now,
		"last_error":            lastError,
		"upstream_body_bytes":   capture.UpstreamBodyBytes,
		"downstream_body_bytes": capture.DownstreamBodyBytes,
	}
	if capture.Session != nil {
		updates["spool_dir"] = capture.Session.Dir()
	}
	if status == model.RequestCaptureStatusFailed {
		updates["has_error"] = true
	}
	if err := model.DB.Model(&model.RequestCaptureRecord{}).Where("id = ?", capture.RecordId).Updates(updates).Error; err != nil {
		relayRequestCaptureLogError("update capture record", "", err)
	}
}

func (capture *RelayRequestCapture) closeWriters() error {
	if capture == nil {
		return nil
	}
	capture.mu.Lock()
	upstreamWriter := capture.upstreamWriter
	downstreamWriter := capture.downstreamWriter
	capture.upstreamWriter = nil
	capture.downstreamWriter = nil
	capture.mu.Unlock()

	var firstErr error
	if upstreamWriter != nil {
		if err := upstreamWriter.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if downstreamWriter != nil {
		if err := downstreamWriter.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func relayRequestCaptureContentType(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}

func relayRequestCaptureDownstreamArtifactName(isStream bool, contentType string) string {
	contentType = strings.ToLower(contentType)
	if isStream || strings.Contains(contentType, "text/event-stream") {
		return "downstream_response.sse"
	}
	if strings.Contains(contentType, "json") {
		return "downstream_response.json"
	}
	return "downstream_response.bin"
}

func relayRequestCaptureUpstreamArtifactName(isStream bool, contentType string) string {
	contentType = strings.ToLower(contentType)
	if isStream || strings.Contains(contentType, "text/event-stream") {
		return "upstream_response.sse"
	}
	if strings.Contains(contentType, "json") {
		return "upstream_response.json"
	}
	return "upstream_response.bin"
}

func relayRequestCaptureCreateRecord(c *gin.Context, info *relaycommon.RelayInfo, capture *RelayRequestCapture) int64 {
	if model.DB == nil || capture == nil {
		return 0
	}
	now := common.GetTimestamp()
	metadata, _ := common.Marshal(map[string]interface{}{
		"decision_reason": capture.Decision.Reason,
		"retry_index":     info.RetryIndex,
		"relay_format":    string(info.RelayFormat),
	})
	conversion, _ := common.Marshal(map[string]interface{}{
		"chain": info.RequestConversionChain,
		"meta":  info.RequestConversionMeta,
		"notes": info.RequestConversionNotes,
	})
	record := model.RequestCaptureRecord{
		RequestId:         info.RequestId,
		UserId:            info.UserId,
		TokenId:           info.TokenId,
		ChannelId:         common.GetContextKeyInt(c, constant.ContextKeyChannelId),
		ConnectedAppId:    int64(c.GetInt("connected_app_id")),
		Group:             info.UsingGroup,
		ModelName:         info.OriginModelName,
		Method:            c.Request.Method,
		RequestPath:       c.Request.URL.Path,
		ProtocolChain:     relayRequestCaptureProtocolChain(info),
		CaptureLevel:      capture.Decision.Level,
		CaptureStatus:     model.RequestCaptureStatusSpooling,
		IsStream:          info.IsStream,
		RequestBytes:      c.Request.ContentLength,
		PromptTokens:      info.GetEstimatePromptTokens(),
		CompletionTokens:  info.GetEstimateCompletionTokens(),
		SpoolDir:          relayRequestCaptureSessionDir(capture),
		MetadataJson:      string(metadata),
		ConversionJson:    string(conversion),
		StartedAt:         now,
		CreatedAt:         now,
		UpdatedAt:         now,
		UpstreamRequestId: c.GetString(common.UpstreamRequestIdKey),
	}
	if record.RequestBytes < 0 {
		record.RequestBytes = 0
	}
	if err := model.DB.Create(&record).Error; err != nil {
		relayRequestCaptureLogError("create capture record", info.RequestId, err)
		return 0
	}
	return record.Id
}

func relayRequestCaptureClientRequest(c *gin.Context, session *RequestCaptureSession) error {
	if c == nil || session == nil {
		return nil
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return err
	}
	body, err := storage.Bytes()
	if err != nil {
		return err
	}
	contentType := strings.TrimSpace(c.Request.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return session.WriteArtifact("client_request.json", contentType, body)
}

func relayRequestCaptureProtocolChain(info *relaycommon.RelayInfo) string {
	if info == nil {
		return ""
	}
	parts := make([]string, 0, len(info.RequestConversionChain)+1)
	if info.RelayFormat != "" {
		parts = append(parts, string(info.RelayFormat))
	}
	for _, item := range info.RequestConversionChain {
		value := string(item)
		if value != "" {
			parts = append(parts, value)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "->")
}

func relayRequestCaptureSessionDir(capture *RelayRequestCapture) string {
	if capture == nil || capture.Session == nil {
		return ""
	}
	return capture.Session.Dir()
}

func relayRequestCaptureLogError(action string, requestId string, err error) {
	if err == nil {
		return
	}
	if errors.Is(err, http.ErrAbortHandler) {
		return
	}
	requestId = strings.TrimSpace(requestId)
	if requestId != "" {
		common.SysLog(fmt.Sprintf("request capture %s failed for request %s: %s", action, requestId, err.Error()))
		return
	}
	common.SysLog(fmt.Sprintf("request capture %s failed: %s", action, err.Error()))
}

func RelayRequestCaptureError(err *types.NewAPIError) error {
	if err == nil {
		return nil
	}
	return err
}
