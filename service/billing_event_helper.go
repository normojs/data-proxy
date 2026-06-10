package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

func billingEventID(source string, sourceId string, phase string) string {
	eventId := fmt.Sprintf("%s:%s:%s", source, sourceId, phase)
	if len(eventId) <= 128 {
		return eventId
	}
	sum := sha256.Sum256([]byte(sourceId))
	return fmt.Sprintf("%s:%s:%s", source, hex.EncodeToString(sum[:16]), phase)
}

func truncateBillingEventString(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:16])
}

func billingEventCost(quota int) float64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}
