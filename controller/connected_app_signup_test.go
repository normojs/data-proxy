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
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestApplySignupConnectedAppFromSessionAndRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:signup_ctrl_%s?mode=memory&cache=shared", t.Name())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	prev := model.DB
	prevSQLite, prevMySQL, prevPG := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	model.DB = db
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	t.Cleanup(func() {
		model.DB = prev
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = prevSQLite, prevMySQL, prevPG
	})
	require.NoError(t, db.AutoMigrate(&model.ConnectedApp{}))
	require.NoError(t, db.Create(&model.ConnectedApp{
		Id:                77,
		Slug:              "niaoweisi",
		Name:              "鸟维斯",
		Status:            model.ConnectedAppStatusEnabled,
		AuthorizationFlow: model.ConnectedAppAuthorizationFlowDeviceCode,
		ClientId:          "niaoweisi",
	}).Error)

	store := cookie.NewStore([]byte("test-secret-key-32bytes-long!!"))
	r := gin.New()
	r.Use(sessions.Sessions("test", store))

	// session path
	r.GET("/session", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("signup_app", "niaoweisi")
		_ = s.Save()
		user := &model.User{}
		applySignupConnectedAppFromSession(s, user)
		c.JSON(200, gin.H{"id": user.SignupConnectedAppId})
	})
	// request body/query path
	r.GET("/request", func(c *gin.Context) {
		user := &model.User{}
		applySignupConnectedAppFromRequest(c, user, "")
		c.JSON(200, gin.H{"id": user.SignupConnectedAppId})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/session", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), `"id":77`)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/request?signup_app=niaoweisi", nil)
	r.ServeHTTP(w2, req2)
	require.Equal(t, 200, w2.Code)
	require.Contains(t, w2.Body.String(), `"id":77`)

	// already set is sticky
	user := &model.User{SignupConnectedAppId: 9}
	applySignupConnectedAppFromRequest(nil, user, "niaoweisi")
	require.Equal(t, 9, user.SignupConnectedAppId)
}
