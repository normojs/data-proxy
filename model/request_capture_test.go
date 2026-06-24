package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRequestCaptureModelsAutoMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&RequestCaptureRecord{},
		&RequestCaptureArtifact{},
		&RequestDiagnosticReport{},
		&TrainingDatasetVersion{},
		&TrainingSample{},
	))

	assert.True(t, db.Migrator().HasTable(&RequestCaptureRecord{}))
	assert.True(t, db.Migrator().HasTable(&RequestCaptureArtifact{}))
	assert.True(t, db.Migrator().HasTable(&RequestDiagnosticReport{}))
	assert.True(t, db.Migrator().HasTable(&TrainingDatasetVersion{}))
	assert.True(t, db.Migrator().HasTable(&TrainingSample{}))

	record := RequestCaptureRecord{
		RequestId:      "req-capture-test",
		ModelName:      "deepseek-ai/DeepSeek-V4-Flash",
		RequestPath:    "/v1/responses",
		ProtocolChain:  "responses->chat",
		CaptureLevel:   RequestCaptureLevelMetadata,
		CaptureStatus:  RequestCaptureStatusPending,
		MetadataJson:   `{"source":"test"}`,
		ConversionJson: `{"mode":"responses_to_chat"}`,
	}
	require.NoError(t, db.Create(&record).Error)
	require.NotZero(t, record.Id)

	artifact := RequestCaptureArtifact{
		CaptureId:  record.Id,
		RequestId:  record.RequestId,
		Kind:       RequestCaptureArtifactKindRawBundle,
		Status:     RequestCaptureArtifactStatusAvailable,
		Provider:   "s3",
		Bucket:     "data-proxy-captures",
		StorageKey: "raw/2026/06/23/13/re/q-/req-capture-test.bundle.tar.zst.enc",
	}
	require.NoError(t, db.Create(&artifact).Error)
	require.NotZero(t, artifact.Id)
}
