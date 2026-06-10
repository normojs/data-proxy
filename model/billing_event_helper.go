package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

func modelBillingEventID(source string, sourceId string, phase string) string {
	eventId := fmt.Sprintf("%s:%s:%s", source, sourceId, phase)
	if len(eventId) <= 128 {
		return eventId
	}
	sum := sha256.Sum256([]byte(sourceId))
	return fmt.Sprintf("%s:%s:%s", source, hex.EncodeToString(sum[:16]), phase)
}

func modelBillingEventCost(quota int) float64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	return float64(quota) / common.QuotaPerUnit
}

func modelBillingEventSourceId(sourceId string) string {
	sourceId = strings.TrimSpace(sourceId)
	if sourceId == "" {
		return ""
	}
	if len(sourceId) <= 128 {
		return sourceId
	}
	sum := sha256.Sum256([]byte(sourceId))
	return hex.EncodeToString(sum[:16])
}
