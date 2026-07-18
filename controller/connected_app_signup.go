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
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// applySignupConnectedAppFromSession sets user.SignupConnectedAppId from the
// OAuth/session "signup_app" value when not already set. No-op on empty/invalid refs.
func applySignupConnectedAppFromSession(session sessions.Session, user *model.User) {
	if user == nil || user.SignupConnectedAppId > 0 || session == nil {
		return
	}
	ref := session.Get("signup_app")
	if ref == nil {
		return
	}
	s, ok := ref.(string)
	if !ok {
		return
	}
	user.SignupConnectedAppId = model.ResolveConnectedAppIDForSignup(s)
}

// applySignupConnectedAppFromRequest resolves signup_app from JSON body field
// (user.SignupApp) or query string for password registration flows.
func applySignupConnectedAppFromRequest(c *gin.Context, user *model.User, bodyRef string) {
	if user == nil || user.SignupConnectedAppId > 0 {
		return
	}
	ref := strings.TrimSpace(bodyRef)
	if ref == "" && c != nil {
		ref = strings.TrimSpace(c.Query("signup_app"))
	}
	user.SignupConnectedAppId = model.ResolveConnectedAppIDForSignup(ref)
}
