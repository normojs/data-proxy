package controller

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type tokenAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type tokenPageResponse struct {
	Items []tokenResponseItem `json:"items"`
}

type tokenResponseItem struct {
	ID                     int    `json:"id"`
	Name                   string `json:"name"`
	Key                    string `json:"key"`
	Status                 int    `json:"status"`
	RemainQuota            int    `json:"remain_quota"`
	UnlimitedQuota         bool   `json:"unlimited_quota"`
	QuotaHardLimitEnabled  bool   `json:"quota_hard_limit_enabled"`
	Group                  string `json:"group"`
	GroupAvailable         bool   `json:"group_available"`
	GroupUnavailableReason string `json:"group_unavailable_reason"`
}

type tokenKeyResponse struct {
	Key string `json:"key"`
}

type sqliteColumnInfo struct {
	Name string `gorm:"column:name"`
	Type string `gorm:"column:type"`
}

type legacyToken struct {
	Id                 int    `gorm:"primaryKey"`
	UserId             int    `gorm:"index"`
	Key                string `gorm:"column:key;type:char(48);uniqueIndex"`
	Status             int    `gorm:"default:1"`
	Name               string `gorm:"index"`
	CreatedTime        int64  `gorm:"bigint"`
	AccessedTime       int64  `gorm:"bigint"`
	ExpiredTime        int64  `gorm:"bigint;default:-1"`
	RemainQuota        int    `gorm:"default:0"`
	UnlimitedQuota     bool
	ModelLimitsEnabled bool
	ModelLimits        string  `gorm:"type:text"`
	AllowIps           *string `gorm:"default:''"`
	UsedQuota          int     `gorm:"default:0"`
	Group              string  `gorm:"column:group;default:''"`
	CrossGroupRetry    bool
	DeletedAt          gorm.DeletedAt `gorm:"index"`
}

func (legacyToken) TableName() string {
	return "tokens"
}

func openTokenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func migrateTokenControllerTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.Enterprise{},
		&model.EnterpriseOrgUnit{},
		&model.EnterpriseOrgMembership{},
		&model.EnterpriseProject{},
		&model.EnterpriseProjectOrgUnit{},
		&model.EnterpriseProjectMember{},
		&model.EnterpriseAuditLog{},
	); err != nil {
		t.Fatalf("failed to migrate token table: %v", err)
	}
	if err := model.EnsureDefaultEnterprise(); err != nil {
		t.Fatalf("failed to ensure default enterprise: %v", err)
	}
}

func setupTokenControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := openTokenControllerTestDB(t)
	migrateTokenControllerTestDB(t, db)
	return db
}

func openTokenControllerExternalDB(t *testing.T, dialect string, dsn string) (*gorm.DB, *bool) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.RedisEnabled = false
	common.UsingSQLite = false
	common.UsingMySQL = dialect == "mysql"
	common.UsingPostgreSQL = dialect == "postgres"

	var (
		db  *gorm.DB
		err error
	)
	switch dialect {
	case "mysql":
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres":
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	default:
		t.Fatalf("unsupported dialect %q", dialect)
	}
	if err != nil {
		t.Fatalf("failed to open %s db: %v", dialect, err)
	}

	model.DB = db
	model.LOG_DB = db

	if db.Migrator().HasTable("tokens") {
		t.Skipf("refusing to run %s migration compatibility test against external database because tokens table already exists", dialect)
	}

	managedTokensTable := new(bool)

	t.Cleanup(func() {
		if *managedTokensTable && db.Migrator().HasTable("tokens") {
			_ = db.Migrator().DropTable("tokens")
		}
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db, managedTokensTable
}

func seedToken(t *testing.T, db *gorm.DB, userID int, name string, rawKey string) *model.Token {
	t.Helper()

	user := model.User{
		Id:       userID,
		Username: fmt.Sprintf("token-owner-%d", userID),
		Status:   common.UserStatusEnabled,
		AffCode:  fmt.Sprintf("token-owner-aff-%d", userID),
	}
	if err := db.Where("id = ?", userID).FirstOrCreate(&user).Error; err != nil {
		t.Fatalf("failed to ensure token owner user: %v", err)
	}

	token := &model.Token{
		UserId:         userID,
		Name:           name,
		Key:            rawKey,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}
	return token
}

func seedEnterpriseProjectForTokenTest(t *testing.T, db *gorm.DB, userID int, orgUnitSlug string, projectSlug string) *model.EnterpriseProject {
	t.Helper()

	enterprise, err := model.GetDefaultEnterprise()
	if err != nil {
		t.Fatalf("failed to get default enterprise: %v", err)
	}
	if err := db.Create(&model.User{Id: userID, Username: fmt.Sprintf("user-%d", userID), Status: common.UserStatusEnabled, AffCode: fmt.Sprintf("aff-%d", userID)}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	orgUnit := model.EnterpriseOrgUnit{
		EnterpriseId: enterprise.Id,
		Name:         orgUnitSlug,
		Slug:         orgUnitSlug,
		Path:         "/1/",
		Depth:        1,
		Status:       model.OrgUnitStatusEnabled,
	}
	if err := db.Create(&orgUnit).Error; err != nil {
		t.Fatalf("failed to seed org unit: %v", err)
	}
	if err := db.Create(&model.EnterpriseOrgMembership{
		EnterpriseId: enterprise.Id,
		UserId:       userID,
		OrgUnitId:    orgUnit.Id,
		IsPrimary:    true,
	}).Error; err != nil {
		t.Fatalf("failed to seed org membership: %v", err)
	}
	project := model.EnterpriseProject{
		EnterpriseId: enterprise.Id,
		Name:         projectSlug,
		Slug:         projectSlug,
		Status:       model.EnterpriseProjectStatusEnabled,
	}
	if err := db.Create(&project).Error; err != nil {
		t.Fatalf("failed to seed project: %v", err)
	}
	if err := db.Create(&model.EnterpriseProjectOrgUnit{
		EnterpriseId: enterprise.Id,
		ProjectId:    project.Id,
		OrgUnitId:    orgUnit.Id,
	}).Error; err != nil {
		t.Fatalf("failed to seed project org unit: %v", err)
	}
	return &project
}

func newAuthenticatedContext(t *testing.T, method string, target string, body any, userID int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var requestBody *bytes.Reader
	if body != nil {
		payload, err := common.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	} else {
		requestBody = bytes.NewReader(nil)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	ctx.Set("id", userID)
	return ctx, recorder
}

func decodeAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) tokenAPIResponse {
	t.Helper()

	var response tokenAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return response
}

func getSQLiteColumnType(t *testing.T, db *gorm.DB, tableName string, columnName string) string {
	t.Helper()

	var columns []sqliteColumnInfo
	if err := db.Raw("PRAGMA table_info(" + tableName + ")").Scan(&columns).Error; err != nil {
		t.Fatalf("failed to inspect %s schema: %v", tableName, err)
	}

	for _, column := range columns {
		if column.Name == columnName {
			return strings.ToLower(column.Type)
		}
	}

	t.Fatalf("column %s not found in %s schema", columnName, tableName)
	return ""
}

func getTokenKeyColumnType(t *testing.T, db *gorm.DB, dialect string) string {
	t.Helper()

	switch dialect {
	case "sqlite":
		return getSQLiteColumnType(t, db, "tokens", "key")
	case "mysql":
		var columnType string
		if err := db.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			"tokens", "key").Scan(&columnType).Error; err != nil {
			t.Fatalf("failed to inspect mysql token key column: %v", err)
		}
		return strings.ToLower(columnType)
	case "postgres":
		var dataType string
		var maxLength sql.NullInt64
		if err := db.Raw(`SELECT data_type, character_maximum_length
			FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			"tokens", "key").Row().Scan(&dataType, &maxLength); err != nil {
			t.Fatalf("failed to inspect postgres token key column: %v", err)
		}
		switch strings.ToLower(dataType) {
		case "character varying":
			return fmt.Sprintf("varchar(%d)", maxLength.Int64)
		case "character":
			return fmt.Sprintf("char(%d)", maxLength.Int64)
		default:
			if maxLength.Valid {
				return fmt.Sprintf("%s(%d)", strings.ToLower(dataType), maxLength.Int64)
			}
			return strings.ToLower(dataType)
		}
	default:
		t.Fatalf("unsupported dialect %q", dialect)
		return ""
	}
}

func runTokenMigrationCompatibilityTest(t *testing.T, db *gorm.DB, dialect string, managedTokensTable *bool) {
	t.Helper()

	legacyKey := strings.Repeat("a", 48)
	longKey := strings.Repeat("b", 64)

	if err := db.AutoMigrate(&legacyToken{}); err != nil {
		t.Fatalf("failed to create legacy token schema: %v", err)
	}
	if managedTokensTable != nil {
		*managedTokensTable = true
	}
	if err := db.Create(&legacyToken{
		UserId:             7,
		Key:                legacyKey,
		Status:             common.TokenStatusEnabled,
		Name:               "legacy-token",
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		RemainQuota:        100,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		AllowIps:           common.GetPointer(""),
		UsedQuota:          0,
		Group:              "default",
		CrossGroupRetry:    false,
	}).Error; err != nil {
		t.Fatalf("failed to seed legacy token row: %v", err)
	}

	if got := getTokenKeyColumnType(t, db, dialect); got != "char(48)" {
		t.Fatalf("expected legacy key column type char(48), got %q", got)
	}

	migrateTokenControllerTestDB(t, db)

	if got := getTokenKeyColumnType(t, db, dialect); got != "varchar(128)" {
		t.Fatalf("expected migrated key column type varchar(128), got %q", got)
	}

	var migratedToken model.Token
	if err := db.First(&migratedToken, "name = ?", "legacy-token").Error; err != nil {
		t.Fatalf("failed to load migrated token row: %v", err)
	}
	if migratedToken.Key != legacyKey {
		t.Fatalf("expected migrated token key %q, got %q", legacyKey, migratedToken.Key)
	}
	if migratedToken.Name != "legacy-token" {
		t.Fatalf("expected migrated token name to be preserved, got %q", migratedToken.Name)
	}

	inserted := model.Token{
		UserId:             8,
		Name:               "long-token",
		Key:                longKey,
		Status:             common.TokenStatusEnabled,
		CreatedTime:        1,
		AccessedTime:       1,
		ExpiredTime:        -1,
		RemainQuota:        200,
		UnlimitedQuota:     true,
		ModelLimitsEnabled: false,
		ModelLimits:        "",
		AllowIps:           common.GetPointer(""),
		UsedQuota:          0,
		Group:              "default",
		CrossGroupRetry:    false,
	}
	if err := db.Create(&inserted).Error; err != nil {
		t.Fatalf("failed to insert long token after migration: %v", err)
	}

	var fetched model.Token
	if err := db.First(&fetched, "id = ?", inserted.Id).Error; err != nil {
		t.Fatalf("failed to fetch long token after migration: %v", err)
	}
	if fetched.Key != longKey {
		t.Fatalf("expected long token key %q, got %q", longKey, fetched.Key)
	}
}

func TestTokenAutoMigrateUsesVarchar128KeyColumn(t *testing.T) {
	db := setupTokenControllerTestDB(t)

	if got := getTokenKeyColumnType(t, db, "sqlite"); got != "varchar(128)" {
		t.Fatalf("expected key column type varchar(128), got %q", got)
	}
}

func TestTokenMigrationFromChar48ToVarchar128(t *testing.T) {
	db := openTokenControllerTestDB(t)
	runTokenMigrationCompatibilityTest(t, db, "sqlite", nil)
}

func TestTokenMigrationFromChar48ToVarchar128MySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run mysql migration compatibility test")
	}

	db, managedTokensTable := openTokenControllerExternalDB(t, "mysql", dsn)
	runTokenMigrationCompatibilityTest(t, db, "mysql", managedTokensTable)
}

func TestTokenMigrationFromChar48ToVarchar128Postgres(t *testing.T) {
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set TEST_POSTGRES_DSN to run postgres migration compatibility test")
	}

	db, managedTokensTable := openTokenControllerExternalDB(t, "postgres", dsn)
	runTokenMigrationCompatibilityTest(t, db, "postgres", managedTokensTable)
}

func TestGetAllTokensMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "list-token", "abcd1234efgh5678")
	seedToken(t, db, 2, "other-user-token", "zzzz1234yyyy5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/?p=1&size=10", nil, 1)
	GetAllTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode token page response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one token, got %d", len(page.Items))
	}
	if page.Items[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked key %q, got %q", token.GetMaskedKey(), page.Items[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("list response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestSearchTokensMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "searchable-token", "ijkl1234mnop5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/search?keyword=searchable-token&p=1&size=10", nil, 1)
	SearchTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one search result, got %d", len(page.Items))
	}
	if page.Items[0].Key != token.GetMaskedKey() {
		t.Fatalf("expected masked search key %q, got %q", token.GetMaskedKey(), page.Items[0].Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("search response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestGetTokenMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "detail-token", "qrst1234uvwx5678")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/"+strconv.Itoa(token.Id), nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token detail response: %v", err)
	}
	if detail.Key != token.GetMaskedKey() {
		t.Fatalf("expected masked detail key %q, got %q", token.GetMaskedKey(), detail.Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("detail response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestGetAllTokensAnnotatesUnavailableGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	if err := db.Create(&model.User{
		Id:          1,
		Username:    "bound-user",
		Status:      common.UserStatusEnabled,
		Group:       "vip",
		TokenGroups: `["default"]`,
		AffCode:     "bound-user-aff",
	}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	token := seedToken(t, db, 1, "legacy-vip-token", "legacy1234vip5678")
	if err := db.Model(token).Update("group", "vip").Error; err != nil {
		t.Fatalf("failed to update token group: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/?p=1&size=10", nil, 1)
	GetAllTokens(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page tokenPageResponse
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode token page response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly one token, got %d", len(page.Items))
	}
	if page.Items[0].Group != "vip" {
		t.Fatalf("expected token group vip, got %q", page.Items[0].Group)
	}
	if page.Items[0].GroupAvailable {
		t.Fatal("expected token group to be marked unavailable")
	}
	if !strings.Contains(page.Items[0].GroupUnavailableReason, "vip") {
		t.Fatalf("expected unavailable reason to mention vip, got %q", page.Items[0].GroupUnavailableReason)
	}
}

func TestUpdateTokenMasksKeyInResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "editable-token", "yzab1234cdef5678")

	body := map[string]any{
		"id":                   token.Id,
		"name":                 "updated-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token update response: %v", err)
	}
	if detail.Key != token.GetMaskedKey() {
		t.Fatalf("expected masked update key %q, got %q", token.GetMaskedKey(), detail.Key)
	}
	if strings.Contains(recorder.Body.String(), token.Key) {
		t.Fatalf("update response leaked raw token key: %s", recorder.Body.String())
	}
}

func TestUpdateTokenRejectsBoundUnavailableGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	if err := db.Create(&model.User{
		Id:          1,
		Username:    "bound-update-user",
		Status:      common.UserStatusEnabled,
		Group:       "default",
		TokenGroups: `["default"]`,
		AffCode:     "bound-update-user-aff",
	}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	token := seedToken(t, db, 1, "bound-update-token", "boundupdate123456")

	body := map[string]any{
		"id":                   token.Id,
		"name":                 "updated-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "vip",
		"cross_group_retry":    false,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/token/", body, 1)
	UpdateToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected unavailable group update to fail, got body: %s", recorder.Body.String())
	}
	if !strings.Contains(response.Message, "vip") {
		t.Fatalf("expected failure message to mention vip, got %q", response.Message)
	}

	var after model.Token
	if err := db.First(&after, token.Id).Error; err != nil {
		t.Fatalf("failed to reload token: %v", err)
	}
	if after.Group != token.Group {
		t.Fatalf("expected token group to remain %q, got %q", token.Group, after.Group)
	}
}

func TestAddAndUpdateTokenQuotaHardLimit(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	if err := db.Create(&model.User{Id: 1, Username: "user-1", Status: common.UserStatusEnabled, AffCode: "aff-1"}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	createBody := map[string]any{
		"name":                     "hard-limit-token",
		"expired_time":             -1,
		"remain_quota":             123,
		"unlimited_quota":          true,
		"quota_hard_limit_enabled": true,
		"model_limits_enabled":     false,
		"model_limits":             "",
		"group":                    "default",
		"cross_group_retry":        false,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", createBody, 1)
	AddToken(ctx)
	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected add token success, got message: %s", response.Message)
	}

	var created model.Token
	if err := db.Where("name = ?", "hard-limit-token").First(&created).Error; err != nil {
		t.Fatalf("failed to load created token: %v", err)
	}
	if !created.QuotaHardLimitEnabled {
		t.Fatal("expected created token hard limit to be enabled")
	}
	if created.RemainQuota != 123 {
		t.Fatalf("expected created hard limit quota 123, got %d", created.RemainQuota)
	}

	updateBody := map[string]any{
		"id":                       created.Id,
		"name":                     "hard-limit-token-updated",
		"expired_time":             -1,
		"remain_quota":             456,
		"unlimited_quota":          true,
		"quota_hard_limit_enabled": true,
		"model_limits_enabled":     false,
		"model_limits":             "",
		"group":                    "default",
		"cross_group_retry":        false,
	}
	ctx, recorder = newAuthenticatedContext(t, http.MethodPut, "/api/token/", updateBody, 1)
	UpdateToken(ctx)
	response = decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected update token success, got message: %s", response.Message)
	}
	var detail tokenResponseItem
	if err := common.Unmarshal(response.Data, &detail); err != nil {
		t.Fatalf("failed to decode token update response: %v", err)
	}
	if !detail.QuotaHardLimitEnabled {
		t.Fatal("expected update response hard limit to be enabled")
	}
	if !detail.UnlimitedQuota {
		t.Fatal("expected update response unlimited quota to remain enabled")
	}
	if detail.RemainQuota != 456 {
		t.Fatalf("expected update response quota 456, got %d", detail.RemainQuota)
	}
}

func TestAddTokenRejectsBoundUnavailableGroup(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	if err := db.Create(&model.User{
		Id:          1,
		Username:    "bound-token-user",
		Status:      common.UserStatusEnabled,
		Group:       "vip",
		TokenGroups: `["default"]`,
		AffCode:     "bound-token-user-aff",
	}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	body := map[string]any{
		"name":                 "bad-group-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "vip",
		"cross_group_retry":    false,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatal("expected unavailable token group to be rejected")
	}
	if !strings.Contains(response.Message, "vip") {
		t.Fatalf("expected error message to mention vip, got %q", response.Message)
	}
}

func TestTokenEnterpriseProjectsListsUserDepartmentProjects(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	project := seedEnterpriseProjectForTokenTest(t, db, 1, "engineering", "inference-platform")
	seedEnterpriseProjectForTokenTest(t, db, 2, "sales", "sales-automation")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/token/enterprise-projects", nil, 1)
	ListTokenEnterpriseProjects(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	var projects []tokenEnterpriseProjectItem
	if err := common.Unmarshal(response.Data, &projects); err != nil {
		t.Fatalf("failed to decode project list: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected one available project, got %d", len(projects))
	}
	if projects[0].Id != project.Id {
		t.Fatalf("expected project %d, got %d", project.Id, projects[0].Id)
	}
}

func TestAddAndUpdateTokenDefaultProject(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	project := seedEnterpriseProjectForTokenTest(t, db, 1, "engineering", "inference-platform")

	createBody := map[string]any{
		"name":                 "project-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
		"default_project_id":   project.Id,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", createBody, 1)
	AddToken(ctx)
	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected add token success, got message: %s", response.Message)
	}
	var created model.Token
	if err := db.Where("user_id = ? AND name = ?", 1, "project-token").First(&created).Error; err != nil {
		t.Fatalf("failed to load created token: %v", err)
	}
	if created.DefaultProjectId != project.Id {
		t.Fatalf("expected default_project_id %d, got %d", project.Id, created.DefaultProjectId)
	}
	var createAudit model.EnterpriseAuditLog
	if err := db.Where("action = ? AND target_type = ? AND target_id = ?", "token.default_project.update", "token", created.Id).First(&createAudit).Error; err != nil {
		t.Fatalf("expected create default project audit: %v", err)
	}
	assertAuditProjectChangeForTokenTest(t, createAudit, 0, project.Id)

	updateBody := map[string]any{
		"id":                   created.Id,
		"name":                 "project-token-updated",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
		"default_project_id":   0,
	}
	ctx, recorder = newAuthenticatedContext(t, http.MethodPut, "/api/token/", updateBody, 1)
	UpdateToken(ctx)
	response = decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected update token success, got message: %s", response.Message)
	}
	var updated model.Token
	if err := db.First(&updated, created.Id).Error; err != nil {
		t.Fatalf("failed to load updated token: %v", err)
	}
	if updated.DefaultProjectId != 0 {
		t.Fatalf("expected default_project_id reset to 0, got %d", updated.DefaultProjectId)
	}
	var audits []model.EnterpriseAuditLog
	if err := db.Where("action = ? AND target_type = ? AND target_id = ?", "token.default_project.update", "token", created.Id).
		Order("id asc").
		Find(&audits).Error; err != nil {
		t.Fatalf("failed to load token project audits: %v", err)
	}
	if len(audits) != 2 {
		t.Fatalf("expected 2 token project audits, got %d", len(audits))
	}
	assertAuditProjectChangeForTokenTest(t, audits[1], project.Id, 0)
}

func assertAuditProjectChangeForTokenTest(t *testing.T, audit model.EnterpriseAuditLog, beforeProjectId int, afterProjectId int) {
	t.Helper()
	var before map[string]int
	if err := common.Unmarshal([]byte(audit.BeforeJson), &before); err != nil {
		t.Fatalf("failed to decode before audit: %v", err)
	}
	var after map[string]int
	if err := common.Unmarshal([]byte(audit.AfterJson), &after); err != nil {
		t.Fatalf("failed to decode after audit: %v", err)
	}
	if before["default_project_id"] != beforeProjectId {
		t.Fatalf("expected before default_project_id %d, got %d", beforeProjectId, before["default_project_id"])
	}
	if after["default_project_id"] != afterProjectId {
		t.Fatalf("expected after default_project_id %d, got %d", afterProjectId, after["default_project_id"])
	}
}

func TestAddTokenRejectsUnavailableDefaultProject(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	otherProject := seedEnterpriseProjectForTokenTest(t, db, 2, "sales", "sales-automation")
	if err := db.Create(&model.User{Id: 1, Username: "user-1", Status: common.UserStatusEnabled, AffCode: "aff-1"}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	body := map[string]any{
		"name":                 "bad-project-token",
		"expired_time":         -1,
		"remain_quota":         100,
		"unlimited_quota":      true,
		"model_limits_enabled": false,
		"model_limits":         "",
		"group":                "default",
		"cross_group_retry":    false,
		"default_project_id":   otherProject.Id,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/", body, 1)
	AddToken(ctx)
	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected unavailable project to be rejected")
	}
	if !strings.Contains(response.Message, "默认项目") {
		t.Fatalf("expected default project error, got %q", response.Message)
	}
}

func TestGetTokenKeyRequiresOwnershipAndReturnsFullKey(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	token := seedToken(t, db, 1, "owned-token", "owner1234token5678")

	authorizedCtx, authorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 1)
	authorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(authorizedCtx)

	authorizedResponse := decodeAPIResponse(t, authorizedRecorder)
	if !authorizedResponse.Success {
		t.Fatalf("expected authorized key fetch to succeed, got message: %s", authorizedResponse.Message)
	}

	var keyData tokenKeyResponse
	if err := common.Unmarshal(authorizedResponse.Data, &keyData); err != nil {
		t.Fatalf("failed to decode token key response: %v", err)
	}
	if keyData.Key != token.GetFullKey() {
		t.Fatalf("expected full key %q, got %q", token.GetFullKey(), keyData.Key)
	}

	unauthorizedCtx, unauthorizedRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/token/"+strconv.Itoa(token.Id)+"/key", nil, 2)
	unauthorizedCtx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(token.Id)}}
	GetTokenKey(unauthorizedCtx)

	unauthorizedResponse := decodeAPIResponse(t, unauthorizedRecorder)
	if unauthorizedResponse.Success {
		t.Fatalf("expected unauthorized key fetch to fail")
	}
	if strings.Contains(unauthorizedRecorder.Body.String(), token.Key) {
		t.Fatalf("unauthorized key response leaked raw token key: %s", unauthorizedRecorder.Body.String())
	}
}
