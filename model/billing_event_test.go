package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBillingEventSummaryDimensionAliasAvoidsMySQLKeyReservedWord(t *testing.T) {
	buildSQL := func(selectExpr string, groupExpr string) string {
		tx := DB.Session(&gorm.Session{DryRun: true}).
			Model(&BillingEvent{}).
			Select(selectExpr+", "+billingEventAggregateSelect(), billingEventAggregateArgs()...).
			Group(groupExpr).
			Order("total_events DESC, dimension_key ASC").
			Scan(&[]BillingEventDimensionAggregate{})
		return strings.ToLower(tx.Statement.SQL.String())
	}

	for _, sql := range []string{
		buildSQL("source AS dimension_key", "source"),
		buildSQL("event_type AS dimension_key", "event_type"),
	} {
		require.Contains(t, sql, " as dimension_key")
		require.Contains(t, sql, "dimension_key asc")
		require.NotContains(t, sql, " as key")
		require.NotContains(t, sql, " key asc")
	}
}
