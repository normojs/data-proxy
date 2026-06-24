package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

func TestBillingEventSourceMatrixCoversAllLedgerSources(t *testing.T) {
	matrix := GetBillingEventSourceMatrix()
	require.NotZero(t, matrix.CheckedAt)
	require.Len(t, matrix.Items, 10)
	require.Equal(t, matrix.TotalSources, len(matrix.Items))

	expectedSources := []string{
		model.BillingEventSourceMCPToolCall,
		model.BillingEventSourceTunnelMCP,
		model.BillingEventSourceTunnelHTTP,
		model.BillingEventSourceModelRequest,
		model.BillingEventSourceAsyncTask,
		model.BillingEventSourceViolationFee,
		model.BillingEventSourceWalletTopUp,
		model.BillingEventSourceWalletAdjust,
		model.BillingEventSourceSubscription,
		model.BillingEventSourceLedgerRepair,
	}
	bySource := make(map[string]bool, len(matrix.Items))
	for _, item := range matrix.Items {
		require.NotEmpty(t, item.Source)
		require.NotEmpty(t, item.EventSource)
		require.NotEmpty(t, item.Label)
		require.NotEmpty(t, item.Status)
		require.NotEmpty(t, item.Notes, "missing notes for %s", item.Source)
		require.False(t, bySource[item.Source], "duplicate source %s", item.Source)
		bySource[item.Source] = true
	}
	for _, source := range expectedSources {
		require.True(t, bySource[source], "missing billing source matrix entry for %s", source)
	}
	require.Equal(t, 7, matrix.ReadySources)
	require.Equal(t, 2, matrix.RecordOnlySources)
	require.Equal(t, 0, matrix.PlannedSources)
	require.Equal(t, 1, matrix.AuditOnlySources)
}

func TestBillingEventSourceMatrixReadySourcesHaveHandlers(t *testing.T) {
	matrix := GetBillingEventSourceMatrix()
	for _, item := range matrix.Items {
		if item.Status != BillingEventSourceCapabilityReady {
			continue
		}
		require.NotEmpty(t, item.BackfillSources, "ready source %s must expose handler sources", item.Source)
		require.True(t, item.SupportsBackfill, "ready source %s must support backfill", item.Source)
		require.True(t, item.SupportsReconciliation, "ready source %s must support reconciliation", item.Source)
		require.True(t, item.SupportsMissingBackfill, "ready source %s must support missing backfill", item.Source)
		require.True(t, item.SupportsRepairAudit, "ready source %s must support repair audit", item.Source)
		require.True(t, item.SupportsAuditRelation, "ready source %s must support audit relation", item.Source)
		for _, source := range item.BackfillSources {
			handler, ok := getBillingEventSourceHandler(source)
			require.True(t, ok, "missing source handler for %s", source)
			require.NotNil(t, handler.Backfill, "missing backfill handler for %s", source)
			require.NotNil(t, handler.Reconcile, "missing reconcile handler for %s", source)
			require.NotNil(t, handler.MissingBackfillInput, "missing missing-backfill handler for %s", source)
		}
	}
}
