package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/mcp/catalog"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MCPToolSourceBuiltin  = "builtin"
	MCPToolSourceCustom   = "custom"
	MCPToolSourcePlugin   = "plugin"
	MCPToolSourceOpenAPI  = "openapi"
	MCPToolSourceMCPProxy = "mcp_proxy"
)

const (
	MCPToolPriceUnitPerCall  = "per_call"
	MCPToolPriceUnitPer1K    = "per_1k_tokens"
	MCPToolPriceUnitPerMB    = "per_mb"
	MCPToolStatusDisabled    = 0
	MCPToolStatusEnabled     = 1
	MCPToolCallStatusPending = "pending"
	MCPToolCallStatusSuccess = "success"
	MCPToolCallStatusError   = "error"
	MCPToolCallStatusTimeout = "timeout"
)

var (
	ErrMCPUserQuotaInsufficient  = errors.New("MCP user quota is insufficient")
	ErrMCPTokenQuotaInsufficient = errors.New("MCP token quota is insufficient")
)

type MCPTool struct {
	Id          int    `json:"id"`
	Name        string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex"`
	DisplayName string `json:"display_name" gorm:"type:varchar(256);not null"`
	Description string `json:"description" gorm:"type:text"`
	Category    string `json:"category" gorm:"type:varchar(64);not null;index"`

	Source     string `json:"source" gorm:"type:varchar(32);not null;default:'builtin'"`
	PluginId   *int   `json:"plugin_id"`
	OpenAPIUrl string `json:"openapi_url" gorm:"column:openapi_url;type:varchar(512);default:''"`

	InputSchema string `json:"input_schema" gorm:"type:text;not null"`

	PricePerCall float64 `json:"price_per_call" gorm:"type:decimal(10,4);not null;default:0"`
	PriceUnit    string  `json:"price_unit" gorm:"type:varchar(32);not null;default:'per_call'"`
	FreeQuota    int     `json:"free_quota" gorm:"not null;default:0"`

	IsRemote  bool `json:"is_remote" gorm:"not null;default:false;index"`
	Status    int  `json:"status" gorm:"not null;default:1;index:idx_mcp_tools_status_sort,priority:1"`
	SortOrder int  `json:"sort_order" gorm:"not null;default:0;index:idx_mcp_tools_status_sort,priority:2"`

	CreatedAt int64          `json:"created_at" gorm:"bigint"`
	UpdatedAt int64          `json:"updated_at" gorm:"bigint"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (MCPTool) TableName() string {
	return "mcp_tools"
}

func (tool *MCPTool) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	tool.CreatedAt = now
	tool.UpdatedAt = now
	return nil
}

func (tool *MCPTool) BeforeUpdate(tx *gorm.DB) error {
	tool.UpdatedAt = common.GetTimestamp()
	return nil
}

type MCPToolCall struct {
	Id       int64  `json:"id"`
	UserId   int    `json:"user_id" gorm:"not null;index"`
	TokenId  int    `json:"token_id" gorm:"index"`
	ToolId   int    `json:"tool_id" gorm:"not null;index"`
	ToolName string `json:"tool_name" gorm:"type:varchar(128);not null;index"`

	RequestId     string `json:"request_id" gorm:"type:varchar(64);not null;index"`
	RequestParams string `json:"request_params" gorm:"type:text"`
	RequestIP     string `json:"request_ip" gorm:"type:varchar(64);default:''"`

	Status        string `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	ResultSummary string `json:"result_summary" gorm:"type:varchar(512);default:''"`
	ErrorCode     string `json:"error_code" gorm:"type:varchar(64);default:''"`
	ErrorMessage  string `json:"error_message" gorm:"type:varchar(512);default:''"`
	Metadata      string `json:"metadata" gorm:"type:text"`

	DurationMS int `json:"duration_ms"`
	ResultSize int `json:"result_size"`

	BridgeSessionId string `json:"bridge_session_id" gorm:"type:varchar(64);default:'';index"`
	TargetClient    string `json:"target_client" gorm:"type:varchar(128);default:'';index"`

	Cost      float64 `json:"cost" gorm:"type:decimal(10,4);not null;default:0"`
	Quota     int     `json:"quota" gorm:"not null;default:0"`
	PriceUnit string  `json:"price_unit" gorm:"type:varchar(32);not null;default:'per_call'"`
	FreeUsed  bool    `json:"free_used" gorm:"not null;default:false"`
	SettledAt int64   `json:"settled_at" gorm:"bigint;not null;default:0;index"`

	CreatedAt int64 `json:"created_at" gorm:"bigint;index"`
}

func (MCPToolCall) TableName() string {
	return "mcp_tool_calls"
}

func (call *MCPToolCall) BeforeCreate(tx *gorm.DB) error {
	call.CreatedAt = common.GetTimestamp()
	return nil
}

type MCPUserDailyQuota struct {
	Id        int64  `json:"id"`
	UserId    int    `json:"user_id" gorm:"not null;uniqueIndex:idx_mcp_user_daily_quota_user_tool_date,priority:1;index"`
	ToolId    int    `json:"tool_id" gorm:"not null;uniqueIndex:idx_mcp_user_daily_quota_user_tool_date,priority:2;index"`
	QuotaDate string `json:"quota_date" gorm:"type:date;not null;uniqueIndex:idx_mcp_user_daily_quota_user_tool_date,priority:3;index"`
	UsedCount int    `json:"used_count" gorm:"not null;default:0"`
	FreeLimit int    `json:"free_limit" gorm:"not null;default:0"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt int64  `json:"updated_at" gorm:"bigint"`
}

func (MCPUserDailyQuota) TableName() string {
	return "mcp_user_daily_quota"
}

func (quota *MCPUserDailyQuota) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	quota.CreatedAt = now
	quota.UpdatedAt = now
	return nil
}

func (quota *MCPUserDailyQuota) BeforeUpdate(tx *gorm.DB) error {
	quota.UpdatedAt = common.GetTimestamp()
	return nil
}

type MCPToolFilter struct {
	Category string
	Source   string
	Status   *int
	Keyword  string
}

type MCPToolCallFilter struct {
	UserId          int
	TokenId         int
	ToolIds         []int
	ToolName        string
	Status          string
	RequestId       string
	BridgeSessionId string
	TargetClient    string
	StartTime       int64
	EndTime         int64
	Keyword         string
}

func ListEnabledMCPTools() ([]MCPTool, error) {
	var tools []MCPTool
	err := DB.Where("status = ?", MCPToolStatusEnabled).
		Order("sort_order asc, id asc").
		Find(&tools).Error
	return tools, err
}

func EnabledMCPToolExists(name string) (bool, error) {
	var count int64
	err := DB.Model(&MCPTool{}).
		Where("name = ? AND status = ?", name, MCPToolStatusEnabled).
		Count(&count).Error
	return count > 0, err
}

func GetEnabledMCPToolByName(name string) (*MCPTool, error) {
	var tool MCPTool
	err := DB.Where("name = ? AND status = ?", name, MCPToolStatusEnabled).First(&tool).Error
	return &tool, err
}

func GetEnabledMCPToolById(id int) (*MCPTool, error) {
	var tool MCPTool
	err := DB.Where("id = ? AND status = ?", id, MCPToolStatusEnabled).First(&tool).Error
	return &tool, err
}

func GetMCPToolById(id int) (*MCPTool, error) {
	var tool MCPTool
	err := DB.First(&tool, "id = ?", id).Error
	return &tool, err
}

func ListMCPToolsByIds(ids []int) (map[int]MCPTool, error) {
	if len(ids) == 0 {
		return map[int]MCPTool{}, nil
	}
	uniqueIds := make([]int, 0, len(ids))
	seen := map[int]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		uniqueIds = append(uniqueIds, id)
	}
	if len(uniqueIds) == 0 {
		return map[int]MCPTool{}, nil
	}
	var tools []MCPTool
	if err := DB.Where("id IN ?", uniqueIds).Find(&tools).Error; err != nil {
		return nil, err
	}
	result := make(map[int]MCPTool, len(tools))
	for _, tool := range tools {
		result[tool.Id] = tool
	}
	return result, nil
}

func CreateMCPToolCall(call *MCPToolCall) error {
	return DB.Create(call).Error
}

func UpdateMCPToolCallStatus(id int64, status string, updates map[string]any) error {
	if updates == nil {
		updates = map[string]any{}
	}
	updates["status"] = status
	return DB.Model(&MCPToolCall{}).Where("id = ?", id).Updates(updates).Error
}

func ReserveMCPToolCallFreeQuota(callId int64, userId int, toolId int, freeQuota int) (bool, error) {
	if callId <= 0 || userId <= 0 || toolId <= 0 || freeQuota <= 0 {
		return false, nil
	}
	reserved := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var call MCPToolCall
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND user_id = ? AND tool_id = ?", callId, userId, toolId).
			First(&call).Error; err != nil {
			return err
		}
		if call.FreeUsed {
			reserved = true
			return nil
		}
		quotaDate := mcpDailyQuotaDateFromUnix(call.CreatedAt)

		var tool MCPTool
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&tool, "id = ?", toolId).Error; err != nil {
			return err
		}
		if tool.FreeQuota <= 0 {
			return nil
		}

		if err := ensureMCPUserDailyQuota(tx, userId, toolId, quotaDate, tool.FreeQuota); err != nil {
			return err
		}

		now := common.GetTimestamp()
		if err := tx.Model(&MCPUserDailyQuota{}).
			Where("user_id = ? AND tool_id = ? AND quota_date = ?", userId, toolId, quotaDate).
			Updates(map[string]any{
				"free_limit": tool.FreeQuota,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}

		result := tx.Model(&MCPUserDailyQuota{}).
			Where("user_id = ? AND tool_id = ? AND quota_date = ? AND used_count < ?", userId, toolId, quotaDate, tool.FreeQuota).
			Updates(map[string]any{
				"used_count": gorm.Expr("used_count + ?", 1),
				"updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}

		result = tx.Model(&MCPToolCall{}).
			Where("id = ? AND user_id = ? AND tool_id = ? AND free_used = ?", callId, userId, toolId, false).
			Update("free_used", true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("failed to mark MCP tool call %d as free quota used", callId)
		}
		reserved = result.RowsAffected > 0
		return nil
	})
	return reserved, err
}

func ReleaseMCPToolCallFreeQuota(callId int64) (bool, error) {
	if callId <= 0 {
		return false, nil
	}
	released := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var call MCPToolCall
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", callId).
			First(&call).Error; err != nil {
			return err
		}
		if !call.FreeUsed {
			return nil
		}

		quotaDate := mcpDailyQuotaDateFromUnix(call.CreatedAt)
		now := common.GetTimestamp()
		result := tx.Model(&MCPUserDailyQuota{}).
			Where("user_id = ? AND tool_id = ? AND quota_date = ? AND used_count > 0", call.UserId, call.ToolId, quotaDate).
			Updates(map[string]any{
				"used_count": gorm.Expr("used_count - ?", 1),
				"updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}

		result = tx.Model(&MCPToolCall{}).
			Where("id = ? AND free_used = ?", call.Id, true).
			Update("free_used", false)
		if result.Error != nil {
			return result.Error
		}
		released = result.RowsAffected > 0
		return nil
	})
	return released, err
}

func ensureMCPUserDailyQuota(tx *gorm.DB, userId int, toolId int, quotaDate string, freeLimit int) error {
	if tx == nil {
		tx = DB
	}
	quota := &MCPUserDailyQuota{
		UserId:    userId,
		ToolId:    toolId,
		QuotaDate: quotaDate,
		UsedCount: 0,
		FreeLimit: freeLimit,
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "tool_id"},
			{Name: "quota_date"},
		},
		DoNothing: true,
	}).Create(quota).Error
}

func mcpDailyQuotaDate(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	return now.Local().Format("2006-01-02")
}

func mcpDailyQuotaDateFromUnix(timestamp int64) string {
	if timestamp <= 0 {
		return mcpDailyQuotaDate(time.Now())
	}
	return mcpDailyQuotaDate(time.Unix(timestamp, 0))
}

func SettleMCPToolCall(id int64, quota int) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	now := common.GetTimestamp()
	result := DB.Model(&MCPToolCall{}).
		Where("id = ? AND settled_at = 0", id).
		Updates(map[string]any{
			"quota":      quota,
			"settled_at": now,
		})
	return result.RowsAffected > 0, result.Error
}

func SettleMCPToolCallQuota(callId int64, userId int, tokenId int, quota int, cost float64, tokenUnlimited bool, priceUnit string) (bool, error) {
	if callId <= 0 {
		return false, nil
	}
	now := common.GetTimestamp()
	settled := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var call MCPToolCall
		if err := tx.Where("id = ?", callId).First(&call).Error; err != nil {
			return err
		}
		result := tx.Model(&MCPToolCall{}).
			Where("id = ? AND settled_at = 0", callId).
			Updates(map[string]any{
				"quota":      quota,
				"cost":       cost,
				"price_unit": priceUnit,
				"settled_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		settled = true
		call.Quota = quota
		call.Cost = cost
		call.PriceUnit = priceUnit
		call.SettledAt = now
		if quota <= 0 {
			return nil
		}
		userResult := tx.Model(&User{}).Where("id = ? AND quota >= ?", userId, quota).Updates(map[string]any{
			"quota":         gorm.Expr("quota - ?", quota),
			"used_quota":    gorm.Expr("used_quota + ?", quota),
			"request_count": gorm.Expr("request_count + ?", 1),
		})
		if userResult.Error != nil {
			return userResult.Error
		}
		if userResult.RowsAffected == 0 {
			return ErrMCPUserQuotaInsufficient
		}
		if !tokenUnlimited && tokenId > 0 {
			tokenResult := tx.Model(&Token{}).Where("id = ? AND remain_quota >= ?", tokenId, quota).Updates(map[string]any{
				"remain_quota":  gorm.Expr("remain_quota - ?", quota),
				"used_quota":    gorm.Expr("used_quota + ?", quota),
				"accessed_time": common.GetTimestamp(),
			})
			if tokenResult.Error != nil {
				return tokenResult.Error
			}
			if tokenResult.RowsAffected == 0 {
				return ErrMCPTokenQuotaInsufficient
			}
		}
		if err := recordMCPToolCallBillingEvent(tx, call, userId, tokenId, quota, priceUnit); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	if settled && quota > 0 {
		afterMCPToolCallQuotaSettled(userId, tokenId, quota, tokenUnlimited)
	}
	return settled, nil
}

func RefundMCPToolCallQuota(callId int64, userId int, tokenId int, quota int, tokenUnlimited bool, reason string) (bool, error) {
	if callId <= 0 || quota <= 0 {
		return false, nil
	}
	refunded := false
	refundQuota := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		var call MCPToolCall
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", callId).First(&call).Error; err != nil {
			return err
		}
		refundEventId := fmt.Sprintf("%s:%d:refund", BillingEventSourceMCPToolCall, call.Id)
		var refundCount int64
		if err := tx.Model(&BillingEvent{}).Where("event_id = ?", refundEventId).Count(&refundCount).Error; err != nil {
			return err
		}
		if refundCount > 0 {
			return nil
		}
		if call.SettledAt <= 0 || call.Quota <= 0 {
			return nil
		}
		refundQuota = quota
		if call.Quota < refundQuota {
			refundQuota = call.Quota
		}
		if refundQuota <= 0 {
			return nil
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).Updates(map[string]any{
			"quota":         gorm.Expr("quota + ?", refundQuota),
			"used_quota":    gorm.Expr("CASE WHEN used_quota >= ? THEN used_quota - ? ELSE 0 END", refundQuota, refundQuota),
			"request_count": gorm.Expr("CASE WHEN request_count > 0 THEN request_count - 1 ELSE 0 END"),
		}).Error; err != nil {
			return err
		}
		if !tokenUnlimited && tokenId > 0 {
			if err := tx.Model(&Token{}).Where("id = ?", tokenId).Updates(map[string]any{
				"remain_quota":  gorm.Expr("remain_quota + ?", refundQuota),
				"used_quota":    gorm.Expr("CASE WHEN used_quota >= ? THEN used_quota - ? ELSE 0 END", refundQuota, refundQuota),
				"accessed_time": common.GetTimestamp(),
			}).Error; err != nil {
				return err
			}
		}
		if err := recordMCPToolCallRefundBillingEvent(tx, call, userId, tokenId, refundQuota, reason); err != nil {
			return err
		}
		if err := tx.Model(&MCPToolCall{}).Where("id = ?", call.Id).Updates(map[string]any{
			"quota": 0,
			"cost":  0,
		}).Error; err != nil {
			return err
		}
		refunded = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if refunded {
		afterMCPToolCallQuotaRefunded(userId, tokenId, refundQuota, tokenUnlimited)
	}
	return refunded, nil
}

func recordMCPToolCallBillingEvent(tx *gorm.DB, call MCPToolCall, userId int, tokenId int, quota int, priceUnit string) error {
	if quota <= 0 {
		return nil
	}
	if strings.TrimSpace(priceUnit) == "" {
		priceUnit = MCPToolPriceUnitPerCall
	}
	metadataBytes, err := common.Marshal(map[string]any{
		"tool_id":           call.ToolId,
		"tool_name":         call.ToolName,
		"status":            call.Status,
		"free_used":         call.FreeUsed,
		"bridge_session_id": call.BridgeSessionId,
		"target_client":     call.TargetClient,
	})
	if err != nil {
		return err
	}
	_, err = CreateBillingEventIfNotExists(tx, &BillingEvent{
		EventId:       fmt.Sprintf("%s:%d:settlement", BillingEventSourceMCPToolCall, call.Id),
		UserId:        userId,
		TokenId:       tokenId,
		Source:        BillingEventSourceMCPToolCall,
		SourceId:      fmt.Sprintf("%d", call.Id),
		EventType:     BillingEventTypeDebit,
		Status:        BillingEventStatusSettled,
		RequestId:     call.RequestId,
		BillingSource: "wallet",
		PriceUnit:     priceUnit,
		Currency:      "quota",
		AmountQuota:   quota,
		QuotaDelta:    -quota,
		Cost:          call.Cost,
		Metadata:      string(metadataBytes),
		CreatedAt:     common.GetTimestamp(),
	})
	return err
}

func recordMCPToolCallRefundBillingEvent(tx *gorm.DB, call MCPToolCall, userId int, tokenId int, quota int, reason string) error {
	if quota <= 0 {
		return nil
	}
	metadataBytes, err := common.Marshal(map[string]any{
		"tool_id":           call.ToolId,
		"tool_name":         call.ToolName,
		"status":            call.Status,
		"reason":            reason,
		"original_call_id":  call.Id,
		"original_quota":    call.Quota,
		"bridge_session_id": call.BridgeSessionId,
		"target_client":     call.TargetClient,
	})
	if err != nil {
		return err
	}
	_, err = CreateBillingEventIfNotExists(tx, &BillingEvent{
		EventId:       fmt.Sprintf("%s:%d:refund", BillingEventSourceMCPToolCall, call.Id),
		UserId:        userId,
		TokenId:       tokenId,
		Source:        BillingEventSourceMCPToolCall,
		SourceId:      fmt.Sprintf("%d", call.Id),
		EventType:     BillingEventTypeCredit,
		Status:        BillingEventStatusSettled,
		RequestId:     call.RequestId,
		BillingSource: "wallet",
		PriceUnit:     "mcp_refund",
		Currency:      "quota",
		AmountQuota:   quota,
		QuotaDelta:    quota,
		Cost:          modelBillingEventCost(quota),
		Metadata:      string(metadataBytes),
		CreatedAt:     common.GetTimestamp(),
	})
	return err
}

func ListMCPTools(filter MCPToolFilter, offset int, limit int) ([]MCPTool, int64, error) {
	query := DB.Model(&MCPTool{})
	query = applyMCPToolFilter(query, filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tools []MCPTool
	err := query.Order("sort_order asc, id asc").
		Limit(limit).
		Offset(offset).
		Find(&tools).Error
	return tools, total, err
}

func UpdateMCPToolFields(id int, updates map[string]any) (*MCPTool, error) {
	if err := DB.Model(&MCPTool{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetMCPToolById(id)
}

func CreateMCPTool(tool *MCPTool) error {
	return DB.Select("*").Create(tool).Error
}

func GetMCPToolByName(name string) (*MCPTool, error) {
	var tool MCPTool
	err := DB.Where("name = ?", strings.TrimSpace(name)).First(&tool).Error
	return &tool, err
}

func MCPToolNameExistsUnscoped(name string) (bool, error) {
	var count int64
	err := DB.Unscoped().Model(&MCPTool{}).
		Where("name = ?", strings.TrimSpace(name)).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}

func ArchiveMCPTool(id int) (*MCPTool, error) {
	if err := DB.Model(&MCPTool{}).
		Where("id = ?", id).
		Update("status", MCPToolStatusDisabled).Error; err != nil {
		return nil, err
	}
	return GetMCPToolById(id)
}

func DeleteMCPTool(id int) error {
	return DB.Delete(&MCPTool{}, id).Error
}

func ListMCPToolCalls(filter MCPToolCallFilter, offset int, limit int) ([]MCPToolCall, int64, error) {
	query := DB.Model(&MCPToolCall{})
	query = applyMCPToolCallFilter(query, filter)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var calls []MCPToolCall
	err := query.Order("created_at desc, id desc").
		Limit(limit).
		Offset(offset).
		Find(&calls).Error
	return calls, total, err
}

func ListMCPToolCallsAfterId(lastId int64, limit int) ([]MCPToolCall, error) {
	items := make([]MCPToolCall, 0, limit)
	err := DB.Where("id > ?", lastId).
		Order("id asc").
		Limit(limit).
		Find(&items).Error
	return items, err
}

func ListMCPToolCallsByIds(ids []int64) (map[int64]MCPToolCall, error) {
	if len(ids) == 0 {
		return map[int64]MCPToolCall{}, nil
	}
	uniqueIds := make([]int64, 0, len(ids))
	seen := map[int64]bool{}
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		uniqueIds = append(uniqueIds, id)
	}
	if len(uniqueIds) == 0 {
		return map[int64]MCPToolCall{}, nil
	}
	var calls []MCPToolCall
	if err := DB.Where("id IN ?", uniqueIds).Find(&calls).Error; err != nil {
		return nil, err
	}
	result := make(map[int64]MCPToolCall, len(calls))
	for _, call := range calls {
		result[call.Id] = call
	}
	return result, nil
}

func MCPToolCallsCountAfterId(lastId int64) (int64, error) {
	var count int64
	err := DB.Model(&MCPToolCall{}).Where("id > ?", lastId).Limit(1).Count(&count).Error
	return count, err
}

func GetMCPToolCallById(tx *gorm.DB, id int64) (MCPToolCall, bool, error) {
	if tx == nil {
		tx = DB
	}
	var call MCPToolCall
	if id <= 0 {
		return call, false, nil
	}
	err := tx.Where("id = ?", id).First(&call).Error
	if err == nil {
		return call, true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return call, false, nil
	}
	return call, false, err
}

func applyMCPToolFilter(query *gorm.DB, filter MCPToolFilter) *gorm.DB {
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if filter.Source != "" {
		query = query.Where("source = ?", filter.Source)
	}
	if filter.Status != nil {
		query = query.Where("status = ?", *filter.Status)
	}
	if filter.Keyword != "" {
		keyword := "%" + filter.Keyword + "%"
		query = query.Where("name LIKE ? OR display_name LIKE ? OR description LIKE ?", keyword, keyword, keyword)
	}
	return query
}

func applyMCPToolCallFilter(query *gorm.DB, filter MCPToolCallFilter) *gorm.DB {
	if filter.UserId > 0 {
		query = query.Where("user_id = ?", filter.UserId)
	}
	if filter.TokenId > 0 {
		query = query.Where("token_id = ?", filter.TokenId)
	}
	if len(filter.ToolIds) > 0 {
		query = query.Where("tool_id IN ?", uniquePositiveInts(filter.ToolIds))
	}
	if strings.TrimSpace(filter.ToolName) != "" {
		query = query.Where("tool_name = ?", strings.TrimSpace(filter.ToolName))
	}
	if strings.TrimSpace(filter.Status) != "" {
		query = query.Where("status = ?", strings.TrimSpace(filter.Status))
	}
	if strings.TrimSpace(filter.RequestId) != "" {
		query = query.Where("request_id = ?", strings.TrimSpace(filter.RequestId))
	}
	if strings.TrimSpace(filter.BridgeSessionId) != "" {
		query = query.Where("bridge_session_id = ?", strings.TrimSpace(filter.BridgeSessionId))
	}
	if strings.TrimSpace(filter.TargetClient) != "" {
		query = query.Where("target_client = ?", strings.TrimSpace(filter.TargetClient))
	}
	if filter.StartTime > 0 {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime > 0 {
		query = query.Where("created_at <= ?", filter.EndTime)
	}
	if strings.TrimSpace(filter.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(filter.Keyword) + "%"
		query = query.Where("tool_name LIKE ? OR request_id LIKE ? OR bridge_session_id LIKE ? OR target_client LIKE ? OR result_summary LIKE ? OR error_message LIKE ?",
			keyword, keyword, keyword, keyword, keyword, keyword)
	}
	return query
}

func uniquePositiveInts(values []int) []int {
	result := make([]int, 0, len(values))
	seen := map[int]bool{}
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func SeedBuiltinMCPTools() error {
	defs := catalog.BuiltinTools()
	if len(defs) == 0 {
		return nil
	}

	tools := make([]MCPTool, 0, len(defs))
	for _, def := range defs {
		schemaBytes, err := common.Marshal(def.InputSchema)
		if err != nil {
			return err
		}
		tools = append(tools, MCPTool{
			Name:         def.Name,
			DisplayName:  def.DisplayName,
			Description:  def.Description,
			Category:     def.Category,
			Source:       def.Source,
			InputSchema:  string(schemaBytes),
			PricePerCall: def.PricePerCall,
			PriceUnit:    def.PriceUnit,
			FreeQuota:    def.FreeQuota,
			IsRemote:     def.IsRemote,
			Status:       MCPToolStatusEnabled,
			SortOrder:    def.SortOrder,
		})
	}

	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"display_name",
			"description",
			"category",
			"source",
			"input_schema",
			"price_unit",
			"is_remote",
			"sort_order",
			"updated_at",
		}),
	}).Create(&tools).Error
}
