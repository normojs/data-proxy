package service

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"gorm.io/gorm"
)

var EnterpriseNotificationPreferenceEventTypes = []string{
	"quota_request.submit",
	"quota_request.approve",
	"quota_request.reject",
	"quota_request.withdraw",
	"quota_request.expire",
	"quota_request.expiring_soon",
}

var EnterpriseNotificationPreferenceChannels = []string{
	model.EnterpriseNotificationPreferenceChannelEmail,
	model.EnterpriseNotificationPreferenceChannelWebhook,
}

type EnterpriseNotificationRecipientScope struct {
	Applicant        bool     `json:"applicant"`
	EnterpriseAdmins bool     `json:"enterprise_admins"`
	ExplicitEmails   []string `json:"explicit_emails"`
}

type EnterpriseNotificationPreferenceItem struct {
	Id                 int                                  `json:"id"`
	EnterpriseId       int                                  `json:"enterprise_id"`
	Channel            string                               `json:"channel"`
	EventType          string                               `json:"event_type"`
	Enabled            bool                                 `json:"enabled"`
	RecipientScope     EnterpriseNotificationRecipientScope `json:"recipient_scope"`
	RecipientScopeJson string                               `json:"recipient_scope_json"`
	CreatedAt          int64                                `json:"created_at"`
	UpdatedAt          int64                                `json:"updated_at"`
}

type EnterpriseNotificationPreferenceUpsertInput struct {
	Channel        string
	EventType      string
	Enabled        bool
	RecipientScope EnterpriseNotificationRecipientScope
}

func ListEnterpriseNotificationPreferences(enterpriseId int) ([]EnterpriseNotificationPreferenceItem, error) {
	var preferences []model.EnterpriseNotificationPreference
	if err := model.DB.Where("enterprise_id = ?", enterpriseId).Find(&preferences).Error; err != nil {
		return nil, err
	}
	byKey := map[string]model.EnterpriseNotificationPreference{}
	for _, preference := range preferences {
		byKey[enterpriseNotificationPreferenceKey(preference.Channel, preference.EventType)] = preference
	}
	items := make([]EnterpriseNotificationPreferenceItem, 0, len(EnterpriseNotificationPreferenceChannels)*len(EnterpriseNotificationPreferenceEventTypes))
	for _, channel := range EnterpriseNotificationPreferenceChannels {
		for _, eventType := range EnterpriseNotificationPreferenceEventTypes {
			preference, ok := byKey[enterpriseNotificationPreferenceKey(channel, eventType)]
			if ok {
				item, err := EnterpriseNotificationPreferenceToItem(preference)
				if err != nil {
					return nil, err
				}
				items = append(items, item)
				continue
			}
			items = append(items, EnterpriseNotificationPreferenceItem{
				EnterpriseId:       enterpriseId,
				Channel:            channel,
				EventType:          eventType,
				Enabled:            false,
				RecipientScope:     EnterpriseNotificationRecipientScope{},
				RecipientScopeJson: "{}",
			})
		}
	}
	return items, nil
}

func UpsertEnterpriseNotificationPreference(enterpriseId int, input EnterpriseNotificationPreferenceUpsertInput) (model.EnterpriseNotificationPreference, model.EnterpriseNotificationPreference, error) {
	channel := strings.TrimSpace(input.Channel)
	eventType := strings.TrimSpace(input.EventType)
	if !isEnterpriseNotificationPreferenceChannel(channel) {
		return model.EnterpriseNotificationPreference{}, model.EnterpriseNotificationPreference{}, errors.New("invalid notification channel")
	}
	if !isEnterpriseNotificationPreferenceEventType(eventType) {
		return model.EnterpriseNotificationPreference{}, model.EnterpriseNotificationPreference{}, errors.New("invalid notification event type")
	}
	scope := input.RecipientScope
	scope.ExplicitEmails = normalizeEnterpriseNotificationEmails(scope.ExplicitEmails)
	scopeJson, err := common.Marshal(scope)
	if err != nil {
		return model.EnterpriseNotificationPreference{}, model.EnterpriseNotificationPreference{}, err
	}
	preference, ok, err := GetEnterpriseNotificationPreferenceWithDB(model.DB, enterpriseId, channel, eventType)
	if err != nil {
		return model.EnterpriseNotificationPreference{}, model.EnterpriseNotificationPreference{}, err
	}
	before := preference
	if !ok {
		preference = model.EnterpriseNotificationPreference{
			EnterpriseId: enterpriseId,
			Channel:      channel,
			EventType:    eventType,
		}
	}
	preference.Enabled = input.Enabled
	preference.RecipientScopeJson = string(scopeJson)
	if !ok {
		if err := model.DB.Create(&preference).Error; err != nil {
			return model.EnterpriseNotificationPreference{}, model.EnterpriseNotificationPreference{}, err
		}
		return before, preference, nil
	}
	if err := model.DB.Save(&preference).Error; err != nil {
		return model.EnterpriseNotificationPreference{}, model.EnterpriseNotificationPreference{}, err
	}
	return before, preference, nil
}

func EnterpriseNotificationPreferenceToItem(preference model.EnterpriseNotificationPreference) (EnterpriseNotificationPreferenceItem, error) {
	scope, err := parseEnterpriseNotificationRecipientScope(preference.RecipientScopeJson)
	if err != nil {
		return EnterpriseNotificationPreferenceItem{}, err
	}
	return EnterpriseNotificationPreferenceItem{
		Id:                 preference.Id,
		EnterpriseId:       preference.EnterpriseId,
		Channel:            preference.Channel,
		EventType:          preference.EventType,
		Enabled:            preference.Enabled,
		RecipientScope:     scope,
		RecipientScopeJson: preference.RecipientScopeJson,
		CreatedAt:          preference.CreatedAt,
		UpdatedAt:          preference.UpdatedAt,
	}, nil
}

func GetEnterpriseNotificationPreferenceWithDB(db *gorm.DB, enterpriseId int, channel string, eventType string) (model.EnterpriseNotificationPreference, bool, error) {
	var preference model.EnterpriseNotificationPreference
	err := db.Where("enterprise_id = ? AND channel = ? AND event_type = ?", enterpriseId, strings.TrimSpace(channel), strings.TrimSpace(eventType)).First(&preference).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return model.EnterpriseNotificationPreference{}, false, nil
		}
		return model.EnterpriseNotificationPreference{}, false, err
	}
	return preference, true, nil
}

func enterpriseNotificationPreferenceKey(channel string, eventType string) string {
	return strings.TrimSpace(channel) + ":" + strings.TrimSpace(eventType)
}

func isEnterpriseNotificationPreferenceChannel(channel string) bool {
	for _, allowed := range EnterpriseNotificationPreferenceChannels {
		if channel == allowed {
			return true
		}
	}
	return false
}

func isEnterpriseNotificationPreferenceEventType(eventType string) bool {
	for _, allowed := range EnterpriseNotificationPreferenceEventTypes {
		if eventType == allowed {
			return true
		}
	}
	return false
}

func EnterpriseNotificationChannelEnabledWithDB(db *gorm.DB, enterpriseId int, channel string, eventType string) (bool, error) {
	preference, ok, err := GetEnterpriseNotificationPreferenceWithDB(db, enterpriseId, channel, eventType)
	if err != nil || !ok {
		return false, err
	}
	return preference.Enabled, nil
}

func EnterpriseNotificationEmailScopeWithDB(db *gorm.DB, enterpriseId int, eventType string) (EnterpriseNotificationRecipientScope, bool, error) {
	preference, ok, err := GetEnterpriseNotificationPreferenceWithDB(db, enterpriseId, model.EnterpriseNotificationPreferenceChannelEmail, eventType)
	if err != nil || !ok || !preference.Enabled {
		return EnterpriseNotificationRecipientScope{}, false, err
	}
	scope, err := parseEnterpriseNotificationRecipientScope(preference.RecipientScopeJson)
	if err != nil {
		return EnterpriseNotificationRecipientScope{}, false, err
	}
	return scope, true, nil
}

func parseEnterpriseNotificationRecipientScope(value string) (EnterpriseNotificationRecipientScope, error) {
	scope := EnterpriseNotificationRecipientScope{}
	value = strings.TrimSpace(value)
	if value == "" {
		return scope, nil
	}
	if err := common.Unmarshal([]byte(value), &scope); err != nil {
		return EnterpriseNotificationRecipientScope{}, err
	}
	scope.ExplicitEmails = normalizeEnterpriseNotificationEmails(scope.ExplicitEmails)
	return scope, nil
}

func normalizeEnterpriseNotificationEmails(emails []string) []string {
	result := make([]string, 0, len(emails))
	seen := map[string]struct{}{}
	for _, email := range emails {
		normalized := strings.ToLower(strings.TrimSpace(email))
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func ListEnterpriseNotificationAdminEmailsWithDB(db *gorm.DB) ([]string, error) {
	var users []model.User
	if err := db.Select("id, email").Where("status = ? AND role >= ? AND email <> ''", common.UserStatusEnabled, common.RoleAdminUser).Find(&users).Error; err != nil {
		return nil, err
	}
	emails := make([]string, 0, len(users))
	for _, user := range users {
		emails = append(emails, user.Email)
	}
	return normalizeEnterpriseNotificationEmails(emails), nil
}
