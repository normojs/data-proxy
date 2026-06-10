package model

type BridgeClientStats struct {
	TotalClients  int64 `gorm:"column:total_clients"`
	ActiveClients int64 `gorm:"column:active_clients"`
}

type BridgeAuditLogStats struct {
	TotalRequests int64   `gorm:"column:total_requests"`
	Success       int64   `gorm:"column:success"`
	Error         int64   `gorm:"column:error"`
	Timeout       int64   `gorm:"column:timeout"`
	Pending       int64   `gorm:"column:pending"`
	ResultSize    int64   `gorm:"column:result_size"`
	AvgDurationMS float64 `gorm:"column:avg_duration_ms"`
}

func GetBridgeClientStats(userId int, activeSince int64) (BridgeClientStats, error) {
	query := DB.Model(&BridgeClient{})
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}

	var stats BridgeClientStats
	err := query.Select(
		`COUNT(*) AS total_clients,
		COALESCE(SUM(CASE WHEN last_seen_at >= ? THEN 1 ELSE 0 END), 0) AS active_clients`,
		activeSince,
	).Scan(&stats).Error
	return stats, err
}

func GetBridgeAuditLogStats(filter BridgeAuditLogFilter) (BridgeAuditLogStats, error) {
	query := DB.Model(&BridgeAuditLog{})
	query = applyBridgeAuditLogFilter(query, filter)

	var stats BridgeAuditLogStats
	err := query.Select(
		`COUNT(*) AS total_requests,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS success,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS error,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS timeout,
		COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) AS pending,
		COALESCE(SUM(result_size), 0) AS result_size,
		COALESCE(AVG(duration_ms), 0) AS avg_duration_ms`,
		BridgeAuditStatusSuccess,
		BridgeAuditStatusError,
		BridgeAuditStatusTimeout,
		BridgeAuditStatusPending,
	).Scan(&stats).Error
	return stats, err
}

func ListRecentBridgeAuditErrors(filter BridgeAuditLogFilter, limit int) ([]BridgeAuditLog, error) {
	if limit <= 0 {
		limit = 5
	}
	query := DB.Model(&BridgeAuditLog{})
	query = applyBridgeAuditLogFilter(query, filter)

	var logs []BridgeAuditLog
	err := query.Where(
		"status IN ? OR error_message <> ''",
		[]string{BridgeAuditStatusError, BridgeAuditStatusTimeout},
	).
		Order("created_at desc, id desc").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
