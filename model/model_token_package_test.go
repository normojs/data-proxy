package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func withModelTokenPackageTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ModelTokenPackage{}, &ModelTokenPackageLedger{}))
	original := DB
	DB = db
	t.Cleanup(func() {
		DB = original
	})
}

func TestCreateAndConsumeModelTokenPackage(t *testing.T) {
	withModelTokenPackageTestDB(t)

	pkg, err := CreateModelTokenPackage(ModelTokenPackageCreateInput{
		UserId:      9001,
		Name:        "test-pack",
		Models:      []string{"gpt-4o", "gpt-4o-mini"},
		TotalTokens: 1000,
		InputRatio:  1,
		OutputRatio: 1,
		CacheRatio:  0.5,
		Source:      ModelTokenPackageSourceAdminGrant,
		CreatedBy:   1,
	})
	require.NoError(t, err)
	require.NotNil(t, pkg)
	assert.EqualValues(t, 1000, pkg.RemainingTokens)
	assert.Equal(t, []string{"gpt-4o", "gpt-4o-mini"}, pkg.Models)
	assert.Equal(t, 0.5, pkg.CacheRatio)

	updated, err := ConsumeModelTokenPackage(ModelTokenPackageConsumeInput{
		PackageId:        pkg.Id,
		UserId:           9001,
		RequestId:        "req-1",
		Model:            "gpt-4o",
		PromptTokens:     100,
		CompletionTokens: 50,
		CacheTokens:      20,
		ConsumeTokens:    160,
	})
	require.NoError(t, err)
	assert.EqualValues(t, 840, updated.RemainingTokens)
	assert.EqualValues(t, 160, updated.UsedTokens)
	assert.Equal(t, ModelTokenPackageStatusActive, updated.Status)

	_, err = ConsumeModelTokenPackage(ModelTokenPackageConsumeInput{
		PackageId:     pkg.Id,
		UserId:        9001,
		ConsumeTokens: 9000,
	})
	require.ErrorIs(t, err, ErrModelTokenPackageInsufficient)

	// exhaust
	_, err = ConsumeModelTokenPackage(ModelTokenPackageConsumeInput{
		PackageId:     pkg.Id,
		UserId:        9001,
		ConsumeTokens: 840,
	})
	require.NoError(t, err)
	loaded, err := GetModelTokenPackageById(pkg.Id)
	require.NoError(t, err)
	assert.Equal(t, ModelTokenPackageStatusExhausted, loaded.Status)
	assert.EqualValues(t, 0, loaded.RemainingTokens)
}

func TestPackageCoversModel(t *testing.T) {
	pkg := ModelTokenPackage{ModelsJson: `["gpt-4o","claude-3-5-sonnet"]`}
	pkg.Models = ParseModelTokenPackageModels(pkg.ModelsJson)
	assert.True(t, PackageCoversModel(pkg, "gpt-4o"))
	assert.True(t, PackageCoversModel(pkg, "GPT-4O"))
	assert.False(t, PackageCoversModel(pkg, "gpt-5"))
}

func TestEncodeModelTokenPackageModels(t *testing.T) {
	raw, err := EncodeModelTokenPackageModels([]string{" gpt-4o ", "gpt-4o", "gpt-4o-mini"})
	require.NoError(t, err)
	models := ParseModelTokenPackageModels(raw)
	assert.Equal(t, []string{"gpt-4o", "gpt-4o-mini"}, models)
}

func TestDisableModelTokenPackage(t *testing.T) {
	withModelTokenPackageTestDB(t)
	pkg, err := CreateModelTokenPackage(ModelTokenPackageCreateInput{
		UserId:      9002,
		Models:      []string{"gpt-4o"},
		TotalTokens: 100,
		InputRatio:  1,
		OutputRatio: 1,
		CacheRatio:  1,
	})
	require.NoError(t, err)
	disabled, err := DisableModelTokenPackage(pkg.Id, 9002)
	require.NoError(t, err)
	assert.Equal(t, ModelTokenPackageStatusDisabled, disabled.Status)
}
