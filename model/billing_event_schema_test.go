package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBillingEventPricingActualIndexAutoMigrate(t *testing.T) {
	require.True(
		t,
		DB.Migrator().HasIndex(&BillingEvent{}, "idx_billing_events_pricing_actual"),
		"billing_events should have the pricing actual sample lookup index",
	)
}
