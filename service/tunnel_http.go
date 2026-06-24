package service

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/bridge"
	"github.com/QuantumNous/new-api/pkg/bridgepolicy"
	"github.com/QuantumNous/new-api/pkg/mcpgateway"
)

const (
	BridgeCapabilityHTTPTunnel  = "http_tunnel"
	BridgeToolHTTPTunnelRequest = "http_tunnel.request"

	DefaultTunnelHTTPMaxRequestBytes  = 8 * 1024 * 1024
	DefaultTunnelHTTPMaxResponseBytes = 2 * 1024 * 1024
)

var (
	ErrTunnelHTTPAuthRequired       = errors.New("tunnel http auth token is required")
	ErrTunnelHTTPAuthForbidden      = errors.New("tunnel http auth token is invalid")
	ErrTunnelHTTPRouteForbidden     = errors.New("tunnel http route is forbidden")
	ErrTunnelHTTPRouteConfigInvalid = errors.New("tunnel http route config is invalid")
	ErrTunnelHTTPRequestTooLarge    = errors.New("tunnel http request body is too large")
	ErrTunnelHTTPResponseTooLarge   = errors.New("tunnel http response body is too large")
)

type TunnelHTTPForwardRequest struct {
	Context       context.Context
	ConnectionKey string
	Slug          string
	Method        string
	Host          string
	ProxyPath     string
	RawQuery      string
	Headers       http.Header
	Body          []byte
	RequestId     string
	ClientIP      string
}

type TunnelHTTPForwardResponse struct {
	StatusCode      int
	Headers         http.Header
	Body            []byte
	DurationMS      int
	RequestId       string
	BridgeSessionId string
	TargetClient    string
	TargetURL       string
}

func ForwardTunnelHTTPRequest(req TunnelHTTPForwardRequest) (TunnelHTTPForwardResponse, error) {
	req.RequestId = tunnelHTTPRequestId(req.RequestId)
	app, connection, err := getAuthorizedTunnelHTTPApp(req.Slug, req.ConnectionKey)
	if err != nil {
		return TunnelHTTPForwardResponse{}, err
	}
	_ = model.TouchTunnelConnectionUsage(connection.Id, req.RequestId)
	targetURL, err := tunnelHTTPTargetURL(*app, req.ProxyPath, req.RawQuery)
	if err != nil {
		return TunnelHTTPForwardResponse{}, err
	}
	bytesIn := int64(len(req.Body))
	startedAt := time.Now()
	routeConfig, err := tunnelHTTPRouteConfigFromApp(*app)
	if err != nil {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, tunnelHTTPRouteAuditMetadata(routeConfig, "route_config_invalid", err, req))
		return TunnelHTTPForwardResponse{}, err
	}
	if err := validateTunnelHTTPRouteAccess(req, routeConfig); err != nil {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, tunnelHTTPRouteAuditMetadata(routeConfig, tunnelHTTPRouteErrorReason(err), err, req))
		return TunnelHTTPForwardResponse{}, err
	}
	if routeConfig.MaxRequestBytes > 0 && bytesIn > routeConfig.MaxRequestBytes {
		durationMS := int(time.Since(startedAt).Milliseconds())
		err := fmt.Errorf("%w: %d > %d", ErrTunnelHTTPRequestTooLarge, bytesIn, routeConfig.MaxRequestBytes)
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, tunnelHTTPRouteAuditMetadata(routeConfig, "request_too_large", err, req))
		return TunnelHTTPForwardResponse{}, err
	}
	if err := checkTunnelRequestRateLimit(*app, *connection, bytesIn); err != nil {
		durationMS := int(time.Since(startedAt).Milliseconds())
		metadata := tunnelHTTPRateLimitMetadata(err)
		metadata["phase"] = "request"
		_ = createTunnelHTTPAuditLogWithAction(model.TunnelAuditActionRateLimit, *app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, metadata)
		return TunnelHTTPForwardResponse{}, err
	}
	policy, err := model.GetBridgeClientPolicyByClientId(app.BridgeClientId)
	if err != nil {
		return TunnelHTTPForwardResponse{}, err
	}
	if err := bridgepolicy.ValidateTool(policy, BridgeToolHTTPTunnelRequest); err != nil {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, map[string]any{
			"reason": bridgepolicy.ErrorCode(err),
			"error":  err.Error(),
		})
		return TunnelHTTPForwardResponse{}, err
	}
	if err := bridgepolicy.ValidateHTTPTarget(policy, targetURL); err != nil {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, map[string]any{
			"reason": bridgepolicy.ErrorCode(err),
			"error":  err.Error(),
		})
		return TunnelHTTPForwardResponse{}, err
	}
	session, ok := bridge.DefaultHub.SelectSession(app.UserId, app.BridgeClientId, BridgeCapabilityHTTPTunnel)
	if !ok {
		durationMS := int(time.Since(startedAt).Milliseconds())
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, map[string]any{
			"reason": "bridge_client_not_online",
		})
		return TunnelHTTPForwardResponse{}, bridge.ErrClientNotFound
	}
	arguments := tunnelHTTPBridgeArguments(req, *app, targetURL, policy, routeConfig)
	audit := createTunnelHTTPBridgeAudit(req.RequestId, session, arguments, app.UserId, connection.AgentTokenId)
	response, err := bridge.DefaultHub.ForwardToolCall(tunnelHTTPContext(req.Context), session.SessionId, bridge.ToolCallRequest{
		Id:        req.RequestId,
		ToolName:  BridgeToolHTTPTunnelRequest,
		Arguments: bridgepolicy.ApplyArgumentLimits(policy, BridgeToolHTTPTunnelRequest, arguments),
	})
	durationMS := int(time.Since(startedAt).Milliseconds())
	if err != nil {
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, map[string]any{
			"bridge_session_id": session.SessionId,
			"target_client":     session.ClientId,
			"error":             err.Error(),
		})
		return TunnelHTTPForwardResponse{}, err
	}
	result, err := tunnelHTTPResponseFromBridge(response.Result)
	if err != nil {
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, 0, durationMS, map[string]any{
			"bridge_session_id": session.SessionId,
			"target_client":     session.ClientId,
			"error":             err.Error(),
		})
		return TunnelHTTPForwardResponse{}, err
	}
	maxResponseBytes := tunnelHTTPMaxResponseBytes(policy, routeConfig)
	if maxResponseBytes > 0 && int64(len(result.Body)) > maxResponseBytes {
		err := fmt.Errorf("%w: %d > %d", ErrTunnelHTTPResponseTooLarge, len(result.Body), maxResponseBytes)
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, int64(len(result.Body)), durationMS, map[string]any{
			"bridge_session_id":  session.SessionId,
			"target_client":      session.ClientId,
			"reason":             "response_too_large",
			"error":              err.Error(),
			"max_response_bytes": maxResponseBytes,
		})
		return TunnelHTTPForwardResponse{}, err
	}
	if err := checkTunnelResponseRateLimit(*app, *connection, int64(len(result.Body))); err != nil {
		updateTunnelHTTPBridgeAuditError(audit, err, durationMS)
		metadata := tunnelHTTPRateLimitMetadata(err)
		metadata["phase"] = "response"
		metadata["bridge_session_id"] = session.SessionId
		metadata["target_client"] = session.ClientId
		metadata["status_code"] = result.StatusCode
		_ = createTunnelHTTPAuditLogWithAction(model.TunnelAuditActionRateLimit, *app, *connection, req, targetURL, mcpgateway.DecisionDeny, bytesIn, int64(len(result.Body)), durationMS, metadata)
		return TunnelHTTPForwardResponse{}, err
	}
	result.DurationMS = durationMS
	result.RequestId = req.RequestId
	result.BridgeSessionId = response.Session.SessionId
	result.TargetClient = response.Session.ClientId
	result.TargetURL = targetURL
	updateTunnelHTTPBridgeAuditSuccess(audit, durationMS, len(result.Body))
	_ = createTunnelHTTPAuditLog(*app, *connection, req, targetURL, mcpgateway.DecisionAllow, bytesIn, int64(len(result.Body)), durationMS, map[string]any{
		"bridge_session_id": result.BridgeSessionId,
		"target_client":     result.TargetClient,
		"status_code":       result.StatusCode,
	})
	return result, nil
}

func getAuthorizedTunnelHTTPApp(slug string, connectionKey string) (*model.TunnelApp, *model.TunnelConnection, error) {
	if strings.TrimSpace(connectionKey) == "" {
		return nil, nil, errors.New("tunnel http connection key is required")
	}
	connection, err := model.GetTunnelConnectionByKeyHash(tunnelConnectionKeyHash(connectionKey))
	if err != nil {
		return nil, nil, err
	}
	app, err := model.GetTunnelAppByPublicSlug(slug)
	if err != nil {
		return nil, nil, err
	}
	if connection.AppId != app.Id || connection.UserId != app.UserId {
		return nil, nil, errors.New("tunnel http app not found")
	}
	if app.AppType != model.TunnelAppTypeHTTP {
		return nil, nil, errors.New("tunnel app is not an HTTP tunnel")
	}
	if app.Status != model.TunnelAppStatusApproved {
		return nil, nil, errors.New("tunnel http app is not approved")
	}
	if strings.TrimSpace(app.BridgeClientId) == "" {
		return nil, nil, errors.New("tunnel http app has no bridge client")
	}
	if connection.Status != model.TunnelConnectionStatusActive {
		return nil, nil, errors.New("tunnel http connection is not active")
	}
	if connection.ExpiresAt > 0 && connection.ExpiresAt <= common.GetTimestamp() {
		return nil, nil, errors.New("tunnel http connection is expired")
	}
	return app, connection, nil
}

type tunnelHTTPRouteConfig struct {
	AuthMode         string
	AuthToken        string
	Host             string
	PathPrefix       string
	MaxRequestBytes  int64
	MaxResponseBytes int64
}

func tunnelHTTPRouteConfigFromApp(app model.TunnelApp) (tunnelHTTPRouteConfig, error) {
	return tunnelHTTPRouteConfigFromJSON(app.RouteJson)
}

func tunnelHTTPRouteConfigFromJSON(raw string) (tunnelHTTPRouteConfig, error) {
	config := tunnelHTTPRouteConfig{
		AuthMode:   model.TunnelRouteAuthPrivate,
		PathPrefix: "/",
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return config, nil
	}
	route := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &route); err != nil {
		return config, fmt.Errorf("%w: %v", ErrTunnelHTTPRouteConfigInvalid, err)
	}
	authMode := strings.ToLower(stringFromAny(route["auth_mode"]))
	if authMode == "" {
		authMode = model.TunnelRouteAuthPrivate
	}
	switch authMode {
	case model.TunnelRouteAuthPrivate, model.TunnelRouteAuthPublic:
		config.AuthMode = authMode
	case model.TunnelRouteAuthToken:
		config.AuthMode = authMode
		config.AuthToken = stringFromAny(route["auth_token"])
		if config.AuthToken == "" {
			config.AuthToken = stringFromAny(route["token"])
		}
		if config.AuthToken == "" {
			config.AuthToken = stringFromAny(route["bearer_token"])
		}
		if config.AuthToken == "" {
			return config, fmt.Errorf("%w: token auth mode requires auth_token", ErrTunnelHTTPRouteConfigInvalid)
		}
	default:
		return config, fmt.Errorf("%w: unsupported auth_mode %q", ErrTunnelHTTPRouteConfigInvalid, authMode)
	}
	config.Host = tunnelHTTPNormalizeHost(stringFromAny(route["host"]))
	config.PathPrefix = tunnelHTTPNormalizePathPrefix(stringFromAny(route["path_prefix"]))
	config.MaxRequestBytes = int64FromAny(route["max_request_bytes"])
	config.MaxResponseBytes = int64FromAny(route["max_response_bytes"])
	if config.MaxRequestBytes < 0 {
		return config, fmt.Errorf("%w: max_request_bytes cannot be negative", ErrTunnelHTTPRouteConfigInvalid)
	}
	if config.MaxResponseBytes < 0 {
		return config, fmt.Errorf("%w: max_response_bytes cannot be negative", ErrTunnelHTTPRouteConfigInvalid)
	}
	return config, nil
}

func validateTunnelHTTPRouteConfigJSON(appType string, raw string) error {
	if appType != model.TunnelAppTypeHTTP {
		return nil
	}
	_, err := tunnelHTTPRouteConfigFromJSON(raw)
	return err
}

func validateTunnelHTTPRouteAccess(req TunnelHTTPForwardRequest, route tunnelHTTPRouteConfig) error {
	if route.Host != "" && !tunnelHTTPHostMatches(tunnelHTTPRequestHost(req), route.Host) {
		return fmt.Errorf("%w: host mismatch", ErrTunnelHTTPRouteForbidden)
	}
	if !tunnelHTTPPathMatches(req.ProxyPath, route.PathPrefix) {
		return fmt.Errorf("%w: path prefix mismatch", ErrTunnelHTTPRouteForbidden)
	}
	if route.AuthMode == model.TunnelRouteAuthToken {
		token := tunnelHTTPRequestAuthToken(req.Headers)
		if token == "" {
			return ErrTunnelHTTPAuthRequired
		}
		if subtle.ConstantTimeCompare([]byte(token), []byte(route.AuthToken)) != 1 {
			return ErrTunnelHTTPAuthForbidden
		}
	}
	return nil
}

func tunnelHTTPRouteAuditMetadata(route tunnelHTTPRouteConfig, reason string, err error, req TunnelHTTPForwardRequest) map[string]any {
	metadata := map[string]any{
		"reason":            reason,
		"auth_mode":         route.AuthMode,
		"route_host":        route.Host,
		"route_path_prefix": route.PathPrefix,
		"request_host":      tunnelHTTPRequestHost(req),
	}
	if route.MaxRequestBytes > 0 {
		metadata["max_request_bytes"] = route.MaxRequestBytes
	}
	if route.MaxResponseBytes > 0 {
		metadata["max_response_bytes"] = route.MaxResponseBytes
	}
	if err != nil {
		metadata["error"] = err.Error()
	}
	return metadata
}

func tunnelHTTPRouteErrorReason(err error) string {
	switch {
	case errors.Is(err, ErrTunnelHTTPAuthRequired):
		return "auth_token_required"
	case errors.Is(err, ErrTunnelHTTPAuthForbidden):
		return "auth_token_invalid"
	case errors.Is(err, ErrTunnelHTTPRouteConfigInvalid):
		return "route_config_invalid"
	case errors.Is(err, ErrTunnelHTTPRouteForbidden):
		return "route_forbidden"
	default:
		return "route_denied"
	}
}

func tunnelHTTPRequestAuthToken(headers http.Header) string {
	if headers == nil {
		return ""
	}
	for _, value := range headers.Values("Authorization") {
		value = strings.TrimSpace(value)
		if strings.EqualFold(value, "bearer") {
			continue
		}
		if strings.HasPrefix(strings.ToLower(value), "bearer ") {
			return strings.TrimSpace(value[len("bearer "):])
		}
	}
	return strings.TrimSpace(headers.Get("X-Tunnel-Token"))
}

func tunnelHTTPRequestHost(req TunnelHTTPForwardRequest) string {
	if req.Host != "" {
		return tunnelHTTPNormalizeHost(req.Host)
	}
	return tunnelHTTPNormalizeHost(req.Headers.Get("Host"))
}

func tunnelHTTPHostMatches(requestHost string, routeHost string) bool {
	requestHost = tunnelHTTPNormalizeHost(requestHost)
	routeHost = tunnelHTTPNormalizeHost(routeHost)
	if routeHost == "" || requestHost == routeHost {
		return true
	}
	if strings.Contains(routeHost, ":") {
		return false
	}
	return tunnelHTTPHostWithoutPort(requestHost) == routeHost
}

func tunnelHTTPNormalizeHost(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err == nil && parsed.Host != "" {
			value = parsed.Host
		}
	}
	value = strings.TrimSuffix(value, ".")
	return value
}

func tunnelHTTPHostWithoutPort(value string) string {
	value = tunnelHTTPNormalizeHost(value)
	if value == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		return strings.Trim(strings.TrimSpace(strings.ToLower(host)), "[]")
	}
	if strings.Count(value, ":") == 1 {
		host, _, ok := strings.Cut(value, ":")
		if ok {
			return strings.TrimSpace(host)
		}
	}
	return strings.Trim(value, "[]")
}

func tunnelHTTPNormalizePathPrefix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return path.Clean(value)
}

func tunnelHTTPPathMatches(proxyPath string, pathPrefix string) bool {
	pathPrefix = tunnelHTTPNormalizePathPrefix(pathPrefix)
	if pathPrefix == "/" {
		return true
	}
	proxyPath = tunnelHTTPNormalizePathPrefix(proxyPath)
	return proxyPath == pathPrefix || strings.HasPrefix(proxyPath, strings.TrimRight(pathPrefix, "/")+"/")
}

func tunnelHTTPTargetURL(app model.TunnelApp, proxyPath string, rawQuery string) (string, error) {
	base := strings.TrimSpace(app.TargetPath)
	if strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://") {
		parsed, err := url.Parse(base)
		if err != nil {
			return "", err
		}
		joinTunnelHTTPURLPath(parsed, proxyPath)
		if rawQuery != "" {
			parsed.RawQuery = rawQuery
		}
		return parsed.String(), nil
	}
	host := strings.TrimSpace(app.TargetHost)
	if host == "" {
		host = "127.0.0.1"
	}
	if app.TargetPort <= 0 || app.TargetPort > 65535 {
		return "", errors.New("invalid tunnel http target port")
	}
	if base == "" {
		base = "/"
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	parsed := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", host, app.TargetPort),
		Path:   base,
	}
	joinTunnelHTTPURLPath(parsed, proxyPath)
	if rawQuery != "" {
		parsed.RawQuery = rawQuery
	}
	return parsed.String(), nil
}

func joinTunnelHTTPURLPath(parsed *url.URL, proxyPath string) {
	if parsed == nil {
		return
	}
	basePath := parsed.EscapedPath()
	if basePath == "" {
		basePath = "/"
	}
	proxyPath = strings.TrimSpace(proxyPath)
	if proxyPath == "" || proxyPath == "/" {
		parsed.Path = path.Clean("/" + strings.TrimPrefix(basePath, "/"))
		if strings.HasSuffix(basePath, "/") && !strings.HasSuffix(parsed.Path, "/") {
			parsed.Path += "/"
		}
		return
	}
	joined := path.Join("/", strings.TrimPrefix(basePath, "/"), strings.TrimPrefix(proxyPath, "/"))
	if strings.HasSuffix(proxyPath, "/") && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	parsed.Path = joined
}

func tunnelHTTPBridgeArguments(req TunnelHTTPForwardRequest, app model.TunnelApp, targetURL string, policy bridgepolicy.Policy, route tunnelHTTPRouteConfig) map[string]any {
	headers := tunnelHTTPHeaderMap(req.Headers, true)
	maxResponseBytes := tunnelHTTPMaxResponseBytes(policy, route)
	return map[string]any{
		"target":             targetURL,
		"method":             strings.ToUpper(strings.TrimSpace(req.Method)),
		"headers":            headers,
		"body_base64":        base64.StdEncoding.EncodeToString(req.Body),
		"max_response_bytes": maxResponseBytes,
		"client_ip":          req.ClientIP,
		"server": map[string]any{
			"id":        app.Id,
			"name":      app.Name,
			"namespace": app.PublicSlug,
		},
	}
}

func tunnelHTTPMaxResponseBytes(policy bridgepolicy.Policy, route tunnelHTTPRouteConfig) int64 {
	maxResponseBytes := int64(policy.MaxResultBytes)
	if maxResponseBytes <= 0 {
		maxResponseBytes = DefaultTunnelHTTPMaxResponseBytes
	}
	if route.MaxResponseBytes > 0 && route.MaxResponseBytes < maxResponseBytes {
		maxResponseBytes = route.MaxResponseBytes
	}
	return maxResponseBytes
}

func tunnelHTTPResponseFromBridge(result dto.BridgeToolCallResult) (TunnelHTTPForwardResponse, error) {
	payload := mapFromBridgeMetadata(result.Metadata, "http_response")
	if len(payload) == 0 {
		payload = mapFromBridgeMetadata(result.Metadata, "result")
	}
	if len(payload) == 0 {
		for _, block := range result.Content {
			if strings.TrimSpace(block.Text) == "" {
				continue
			}
			if err := common.Unmarshal(common.StringToByteSlice(block.Text), &payload); err == nil && len(payload) > 0 {
				break
			}
		}
	}
	if len(payload) == 0 {
		return TunnelHTTPForwardResponse{}, errors.New("http tunnel bridge response is empty")
	}
	statusCode := intFromAny(payload["status_code"])
	if statusCode <= 0 {
		statusCode = intFromAny(payload["status"])
	}
	if statusCode <= 0 {
		statusCode = http.StatusOK
	}
	body, err := base64.StdEncoding.DecodeString(strings.TrimSpace(stringFromAny(payload["body_base64"])))
	if err != nil {
		return TunnelHTTPForwardResponse{}, err
	}
	return TunnelHTTPForwardResponse{
		StatusCode: statusCode,
		Headers:    tunnelHTTPHeaderFromAny(payload["headers"]),
		Body:       body,
	}, nil
}

func createTunnelHTTPAuditLog(app model.TunnelApp, connection model.TunnelConnection, req TunnelHTTPForwardRequest, targetURL string, decision string, bytesIn int64, bytesOut int64, durationMS int, metadata map[string]any) error {
	return createTunnelHTTPAuditLogWithAction(model.TunnelAuditActionProxyRequest, app, connection, req, targetURL, decision, bytesIn, bytesOut, durationMS, metadata)
}

func createTunnelHTTPAuditLogWithAction(action string, app model.TunnelApp, connection model.TunnelConnection, req TunnelHTTPForwardRequest, targetURL string, decision string, bytesIn int64, bytesOut int64, durationMS int, metadata map[string]any) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["connection_id"] = connection.Id
	metadata["connection_key_prefix"] = connection.KeyPrefix
	metadata["target_url"] = targetURL
	log := &model.TunnelAuditLog{
		AppId:               app.Id,
		ConnectionId:        connection.Id,
		ConnectionKeyPrefix: connection.KeyPrefix,
		UserId:              app.UserId,
		ActorUserId:         app.UserId,
		Action:              action,
		Decision:            decision,
		Reason:              stringFromAny(metadata["reason"]),
		RequestId:           req.RequestId,
		Method:              strings.ToUpper(strings.TrimSpace(req.Method)),
		Path:                limitTunnelString(req.ProxyPath, 512),
		BytesIn:             bytesIn,
		BytesOut:            bytesOut,
		DurationMS:          durationMS,
		SessionId:           stringFromAny(metadata["bridge_session_id"]),
		MetadataJson:        tunnelAuditMetadataJSON(metadata),
	}
	if err := model.CreateTunnelAuditLog(log); err != nil {
		return err
	}
	return recordTunnelAuditBillingEvent(model.BillingEventSourceTunnelHTTP, app, connection, *log)
}

func tunnelHTTPRateLimitMetadata(err error) map[string]any {
	var limitErr TunnelRateLimitError
	if errors.As(err, &limitErr) {
		metadata := limitErr.Metadata()
		metadata["error"] = err.Error()
		return metadata
	}
	return map[string]any{
		"reason": "rate_limited",
		"error":  err.Error(),
	}
}

func createTunnelHTTPBridgeAudit(requestId string, session bridge.SessionSnapshot, args map[string]any, userId int, tokenId int) *model.BridgeAuditLog {
	if model.DB == nil {
		return nil
	}
	body := ""
	if raw, err := common.Marshal(args); err == nil {
		body = string(raw)
	}
	audit := &model.BridgeAuditLog{
		RequestId:   tunnelHTTPRequestId(requestId),
		SessionId:   session.SessionId,
		ClientId:    session.ClientId,
		UserId:      userId,
		TokenId:     tokenId,
		ToolName:    BridgeToolHTTPTunnelRequest,
		RequestBody: body,
		Status:      model.BridgeAuditStatusPending,
	}
	if err := model.CreateBridgeAuditLog(audit); err != nil {
		return nil
	}
	return audit
}

func updateTunnelHTTPBridgeAuditSuccess(audit *model.BridgeAuditLog, durationMS int, resultSize int) {
	if audit == nil {
		return
	}
	_ = model.UpdateBridgeAuditLogStatus(audit.Id, model.BridgeAuditStatusSuccess, map[string]any{
		"duration_ms": durationMS,
		"result_size": resultSize,
	})
}

func updateTunnelHTTPBridgeAuditError(audit *model.BridgeAuditLog, err error, durationMS int) {
	if audit == nil {
		return
	}
	code := bridgepolicy.ErrorCode(err)
	if code == "" {
		code = "HTTP_TUNNEL_FAILED"
	}
	_ = model.UpdateBridgeAuditLogStatus(audit.Id, model.BridgeAuditStatusError, map[string]any{
		"error_code":    code,
		"error_message": limitTunnelString(err.Error(), 512),
		"duration_ms":   durationMS,
	})
}

func tunnelHTTPHeaderMap(headers http.Header, request bool) map[string][]string {
	result := map[string][]string{}
	for key, values := range headers {
		name := http.CanonicalHeaderKey(strings.TrimSpace(key))
		if name == "" || tunnelHTTPDropHeader(name, request) {
			continue
		}
		for _, value := range values {
			result[name] = append(result[name], value)
		}
	}
	return result
}

func tunnelHTTPHeaderFromAny(value any) http.Header {
	result := http.Header{}
	object := mapFromAny(value)
	for key, raw := range object {
		name := http.CanonicalHeaderKey(strings.TrimSpace(key))
		if name == "" || tunnelHTTPDropHeader(name, false) {
			continue
		}
		for _, item := range sliceStringFromAny(raw) {
			result.Add(name, item)
		}
	}
	return result
}

func tunnelHTTPDropHeader(name string, request bool) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "connection", "proxy-connection", "keep-alive", "proxy-authenticate", "proxy-authorization",
		"te", "trailer", "transfer-encoding", "upgrade", "content-length":
		return true
	case "host":
		return request
	case "content-encoding":
		return !request
	default:
		return false
	}
}

func tunnelHTTPRequestId(requestId string) string {
	requestId = strings.TrimSpace(requestId)
	if requestId != "" {
		return requestId
	}
	return "tunnel-http-" + common.GetRandomString(24)
}

func tunnelHTTPContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func mapFromBridgeMetadata(metadata map[string]any, key string) map[string]any {
	if metadata == nil {
		return nil
	}
	return mapFromAny(metadata[key])
}

func mapFromAny(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if object, ok := value.(map[string]any); ok {
		return object
	}
	body, err := common.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var object map[string]any
	if err := common.Unmarshal(body, &object); err != nil {
		return map[string]any{}
	}
	return object
}

func sliceStringFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := stringFromAny(item); text != "" {
				result = append(result, text)
			}
		}
		return result
	case string:
		return []string{typed}
	default:
		text := stringFromAny(value)
		if text == "" {
			return nil
		}
		return []string{text}
	}
}

func stringFromAny(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func int64FromAny(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	case float64:
		return int64(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		return 0
	}
}
