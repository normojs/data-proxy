/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type connectedAppUserGrantResponse struct {
	ID           int64    `json:"id"`
	AppID        int      `json:"app_id"`
	AppSlug      string   `json:"app_slug"`
	AppName      string   `json:"app_name"`
	AppTrusted   bool     `json:"app_trusted"`
	Scopes       []string `json:"scopes"`
	Status       string   `json:"status"`
	AuthorizedAt int64    `json:"authorized_at"`
	LastUsedAt   int64    `json:"last_used_at"`
	RevokedAt    int64    `json:"revoked_at"`
	CreatedAt    int64    `json:"created_at"`
	UpdatedAt    int64    `json:"updated_at"`
}

// ListSelfConnectedAppGrants lists apps the current user has authorized.
func ListSelfConnectedAppGrants(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		common.ApiErrorMsg(c, "login required")
		return
	}
	includeRevoked := c.Query("include_revoked") == "true" || c.Query("include_revoked") == "1"
	grants, err := model.ListConnectedAppGrantsByUserId(userId, !includeRevoked)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	appIDs := make([]int, 0, len(grants))
	seen := map[int]struct{}{}
	for _, grant := range grants {
		if grant.AppId <= 0 {
			continue
		}
		if _, ok := seen[grant.AppId]; ok {
			continue
		}
		seen[grant.AppId] = struct{}{}
		appIDs = append(appIDs, grant.AppId)
	}
	appsByID := map[int]model.ConnectedApp{}
	if len(appIDs) > 0 {
		var apps []model.ConnectedApp
		if err := model.DB.Where("id IN ?", appIDs).Find(&apps).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		for _, app := range apps {
			appsByID[app.Id] = app
		}
	}
	items := make([]connectedAppUserGrantResponse, 0, len(grants))
	for _, grant := range grants {
		app := appsByID[grant.AppId]
		items = append(items, connectedAppUserGrantResponse{
			ID:           grant.Id,
			AppID:        grant.AppId,
			AppSlug:      app.Slug,
			AppName:      app.Name,
			AppTrusted:   app.Trusted,
			Scopes:       grant.ScopeList(),
			Status:       grant.Status,
			AuthorizedAt: grant.AuthorizedAt,
			LastUsedAt:   grant.LastUsedAt,
			RevokedAt:    grant.RevokedAt,
			CreatedAt:    grant.CreatedAt,
			UpdatedAt:    grant.UpdatedAt,
		})
	}
	common.ApiSuccess(c, gin.H{"items": items})
}

// RevokeSelfConnectedAppGrant revokes an authorized app for the current user
// and disables bound platform tokens / access tokens for that app.
func RevokeSelfConnectedAppGrant(c *gin.Context) {
	userId := c.GetInt("id")
	if userId <= 0 {
		common.ApiErrorMsg(c, "login required")
		return
	}
	appID, err := strconv.Atoi(c.Param("app_id"))
	if err != nil || appID <= 0 {
		common.ApiErrorMsg(c, "invalid app_id")
		return
	}
	now := common.GetTimestamp()
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		grant, err := model.GetConnectedAppGrant(appID, userId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("authorization not found")
			}
			return err
		}
		if grant.Status != model.ConnectedAppGrantStatusAuthorized {
			return fmt.Errorf("authorization is not active")
		}
		bindings, err := model.ListConnectedAppTokenBindings(appID, userId)
		if err != nil {
			return err
		}
		for i := range bindings {
			binding := bindings[i]
			if binding.Status != model.ConnectedAppTokenBindingStatusActive {
				continue
			}
			if err := model.RevokeConnectedAppTokenBinding(tx, &binding, now); err != nil {
				return err
			}
			if binding.TokenId > 0 {
				if err := model.DisableTokenWithTx(tx, binding.TokenId, userId); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}
		}
		if err := model.RevokeConnectedAppAccessTokens(tx, appID, userId, "", now); err != nil {
			return err
		}
		return model.RevokeConnectedAppGrant(tx, appID, userId, now)
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"app_id":  appID,
		"status":  model.ConnectedAppGrantStatusRevoked,
		"message": "authorization revoked",
	})
}
