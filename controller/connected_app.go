package controller

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	connectedAppSlugPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	connectedAppScopePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

type connectedAppRequest struct {
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	AllowedScopes     []string `json:"allowed_scopes"`
	DefaultScopes     []string `json:"default_scopes"`
	AuthorizationFlow string   `json:"authorization_flow"`
	Trusted           *bool    `json:"trusted"`
	Status            *int     `json:"status"`
}

type connectedAppResponse struct {
	ID                int      `json:"id"`
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	AllowedScopes     []string `json:"allowed_scopes"`
	DefaultScopes     []string `json:"default_scopes"`
	Trusted           bool     `json:"trusted"`
	Status            int      `json:"status"`
	AuthorizationFlow string   `json:"authorization_flow"`
	GrantCount        int64    `json:"grant_count"`
	DeviceCount       int64    `json:"device_count"`
	ActiveDeviceCount int64    `json:"active_device_count"`
	CreatedAt         int64    `json:"created_at"`
	UpdatedAt         int64    `json:"updated_at"`
}

func ListConnectedApps(c *gin.Context) {
	apps, err := model.ListConnectedApps()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	items := make([]connectedAppResponse, 0, len(apps))
	for _, app := range apps {
		item, err := buildConnectedAppResponse(app)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		items = append(items, item)
	}
	common.ApiSuccess(c, items)
}

func CreateConnectedApp(c *gin.Context) {
	var req connectedAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "Invalid request body")
		return
	}

	app, err := buildConnectedAppForCreate(req)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.CreateConnectedApp(&app); err != nil {
		common.ApiError(c, err)
		return
	}

	resp, err := buildConnectedAppResponse(app)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

func UpdateConnectedApp(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "Invalid connected app id")
		return
	}

	var req connectedAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "Invalid request body")
		return
	}

	app, err := model.GetConnectedAppByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "Connected app not found")
			return
		}
		common.ApiError(c, err)
		return
	}

	if err := applyConnectedAppUpdate(app, req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateConnectedApp(app); err != nil {
		common.ApiError(c, err)
		return
	}

	resp, err := buildConnectedAppResponse(*app)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

func buildConnectedAppForCreate(req connectedAppRequest) (model.ConnectedApp, error) {
	slug, err := normalizeConnectedAppSlug(req.Slug)
	if err != nil {
		return model.ConnectedApp{}, err
	}
	if existing, err := model.GetConnectedAppBySlug(slug); err == nil && existing.Id > 0 {
		return model.ConnectedApp{}, fmt.Errorf("connected app slug already exists")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ConnectedApp{}, err
	}

	name, description, allowedScopes, defaultScopes, err := normalizeConnectedAppFields(req)
	if err != nil {
		return model.ConnectedApp{}, err
	}
	authorizationFlow, err := normalizeConnectedAppAuthorizationFlow(req.AuthorizationFlow)
	if err != nil {
		return model.ConnectedApp{}, err
	}

	status := model.ConnectedAppStatusEnabled
	if req.Status != nil {
		status, err = normalizeConnectedAppStatus(*req.Status)
		if err != nil {
			return model.ConnectedApp{}, err
		}
	}

	trusted := false
	if req.Trusted != nil {
		trusted = *req.Trusted
	}

	return model.ConnectedApp{
		Slug:              slug,
		Name:              name,
		Description:       description,
		AllowedScopes:     strings.Join(allowedScopes, " "),
		DefaultScopes:     strings.Join(defaultScopes, " "),
		AuthorizationFlow: authorizationFlow,
		Trusted:           trusted,
		Status:            status,
	}, nil
}

func applyConnectedAppUpdate(app *model.ConnectedApp, req connectedAppRequest) error {
	name, description, allowedScopes, defaultScopes, err := normalizeConnectedAppFields(req)
	if err != nil {
		return err
	}

	app.Name = name
	app.Description = description
	app.AllowedScopes = strings.Join(allowedScopes, " ")
	app.DefaultScopes = strings.Join(defaultScopes, " ")
	if strings.TrimSpace(req.AuthorizationFlow) != "" {
		authorizationFlow, err := normalizeConnectedAppAuthorizationFlow(req.AuthorizationFlow)
		if err != nil {
			return err
		}
		app.AuthorizationFlow = authorizationFlow
	}
	if req.Trusted != nil {
		app.Trusted = *req.Trusted
	}
	if req.Status != nil {
		status, err := normalizeConnectedAppStatus(*req.Status)
		if err != nil {
			return err
		}
		app.Status = status
	}
	return nil
}

func normalizeConnectedAppSlug(raw string) (string, error) {
	slug := strings.ToLower(strings.TrimSpace(raw))
	if slug == "" {
		return "", fmt.Errorf("connected app slug is required")
	}
	if !connectedAppSlugPattern.MatchString(slug) {
		return "", fmt.Errorf("connected app slug must use lowercase letters, numbers, underscores or hyphens")
	}
	return slug, nil
}

func normalizeConnectedAppFields(req connectedAppRequest) (string, string, []string, []string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", "", nil, nil, fmt.Errorf("connected app name is required")
	}
	if len(name) > 128 {
		return "", "", nil, nil, fmt.Errorf("connected app name is too long")
	}

	description := strings.TrimSpace(req.Description)
	if len(description) > 512 {
		return "", "", nil, nil, fmt.Errorf("connected app description is too long")
	}

	allowedScopes, err := normalizeAndValidateConnectedAppScopes(req.AllowedScopes, "allowed_scopes")
	if err != nil {
		return "", "", nil, nil, err
	}
	if len(allowedScopes) == 0 {
		return "", "", nil, nil, fmt.Errorf("allowed_scopes must contain at least one scope")
	}

	defaultScopes, err := normalizeAndValidateConnectedAppScopes(req.DefaultScopes, "default_scopes")
	if err != nil {
		return "", "", nil, nil, err
	}
	allowedSet := make(map[string]struct{}, len(allowedScopes))
	for _, scope := range allowedScopes {
		allowedSet[scope] = struct{}{}
	}
	for _, scope := range defaultScopes {
		if _, ok := allowedSet[scope]; !ok {
			return "", "", nil, nil, fmt.Errorf("default scope %q is not allowed by allowed_scopes", scope)
		}
	}

	if len(strings.Join(allowedScopes, " ")) > 512 {
		return "", "", nil, nil, fmt.Errorf("allowed_scopes is too long")
	}
	if len(strings.Join(defaultScopes, " ")) > 512 {
		return "", "", nil, nil, fmt.Errorf("default_scopes is too long")
	}

	return name, description, allowedScopes, defaultScopes, nil
}

func normalizeAndValidateConnectedAppScopes(scopes []string, fieldName string) ([]string, error) {
	normalized := model.NormalizeConnectedAppScopes(scopes)
	for _, scope := range normalized {
		if !connectedAppScopePattern.MatchString(scope) {
			return nil, fmt.Errorf("%s contains invalid scope %q", fieldName, scope)
		}
	}
	return normalized, nil
}

func normalizeConnectedAppStatus(status int) (int, error) {
	if status != model.ConnectedAppStatusEnabled && status != model.ConnectedAppStatusDisabled {
		return 0, fmt.Errorf("connected app status must be enabled or disabled")
	}
	return status, nil
}

func normalizeConnectedAppAuthorizationFlow(flow string) (string, error) {
	normalized := strings.TrimSpace(flow)
	if normalized == "" {
		return model.ConnectedAppAuthorizationFlowDeviceCode, nil
	}
	if normalized != model.ConnectedAppAuthorizationFlowDeviceCode {
		return "", fmt.Errorf("connected app authorization_flow is not supported")
	}
	return normalized, nil
}

func buildConnectedAppResponse(app model.ConnectedApp) (connectedAppResponse, error) {
	var grantCount int64
	if err := model.DB.Model(&model.ConnectedAppGrant{}).Where("app_id = ?", app.Id).Count(&grantCount).Error; err != nil {
		return connectedAppResponse{}, err
	}
	var deviceCount int64
	if err := model.DB.Model(&model.ConnectedAppTokenBinding{}).Where("app_id = ?", app.Id).Count(&deviceCount).Error; err != nil {
		return connectedAppResponse{}, err
	}
	var activeDeviceCount int64
	if err := model.DB.Model(&model.ConnectedAppTokenBinding{}).
		Where("app_id = ? AND status = ?", app.Id, model.ConnectedAppTokenBindingStatusActive).
		Count(&activeDeviceCount).Error; err != nil {
		return connectedAppResponse{}, err
	}

	return connectedAppResponse{
		ID:                app.Id,
		Slug:              app.Slug,
		Name:              app.Name,
		Description:       app.Description,
		AllowedScopes:     app.ScopeList(),
		DefaultScopes:     app.DefaultScopeList(),
		Trusted:           app.Trusted,
		Status:            app.Status,
		AuthorizationFlow: connectedAppResponseAuthorizationFlow(app),
		GrantCount:        grantCount,
		DeviceCount:       deviceCount,
		ActiveDeviceCount: activeDeviceCount,
		CreatedAt:         app.CreatedAt,
		UpdatedAt:         app.UpdatedAt,
	}, nil
}

func connectedAppResponseAuthorizationFlow(app model.ConnectedApp) string {
	if strings.TrimSpace(app.AuthorizationFlow) == "" {
		return model.ConnectedAppAuthorizationFlowDeviceCode
	}
	return app.AuthorizationFlow
}
