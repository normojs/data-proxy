/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { z } from 'zod'
import type { TFunction } from 'i18next'
import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'
import {
  REDEMPTION_VALIDATION,
  getRedemptionFormErrorMessages,
} from '../constants'
import { type RedemptionFormData, type Redemption } from '../types'

// ============================================================================
// Form Schema (use getRedemptionFormSchema(t) in components for i18n messages)
// ============================================================================

export function getRedemptionFormSchema(t: TFunction) {
  const msg = getRedemptionFormErrorMessages(t)
  return z
    .object({
      name: z
        .string()
        .min(REDEMPTION_VALIDATION.NAME_MIN_LENGTH, msg.NAME_LENGTH_INVALID)
        .max(REDEMPTION_VALIDATION.NAME_MAX_LENGTH, msg.NAME_LENGTH_INVALID),
      reward_type: z.enum(['quota', 'model_token_package']),
      quota_dollars: z.number().min(0, t('Quota must be a positive number')),
      package_models_text: z.string().optional(),
      package_tokens: z.number().min(0).optional(),
      package_input_ratio: z.number().min(0).optional(),
      package_output_ratio: z.number().min(0).optional(),
      package_cache_ratio: z.number().min(0).optional(),
      package_duration_days: z.number().min(0).optional(),
      expired_time: z.date().optional(),
      count: z
        .number()
        .min(REDEMPTION_VALIDATION.COUNT_MIN, msg.COUNT_INVALID)
        .max(REDEMPTION_VALIDATION.COUNT_MAX, msg.COUNT_INVALID)
        .optional(),
    })
    .superRefine((data, ctx) => {
      if (data.reward_type === 'model_token_package') {
        if (!data.package_tokens || data.package_tokens <= 0) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['package_tokens'],
            message: t('Package tokens must be greater than 0'),
          })
        }
        const models = (data.package_models_text || '')
          .split(/[\n,]/)
          .map((item) => item.trim())
          .filter(Boolean)
        if (models.length === 0) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            path: ['package_models_text'],
            message: t('At least one model is required'),
          })
        }
      }
    })
}

export type RedemptionFormValues = {
  name: string
  reward_type: 'quota' | 'model_token_package'
  quota_dollars: number
  package_models_text?: string
  package_tokens?: number
  package_input_ratio?: number
  package_output_ratio?: number
  package_cache_ratio?: number
  package_duration_days?: number
  expired_time?: Date
  count?: number
}

// ============================================================================
// Form Defaults
// ============================================================================

export const REDEMPTION_FORM_DEFAULT_VALUES: RedemptionFormValues = {
  name: '',
  reward_type: 'quota',
  quota_dollars: 10,
  package_models_text: '',
  package_tokens: 1000000,
  package_input_ratio: 1,
  package_output_ratio: 1,
  package_cache_ratio: 1,
  package_duration_days: 0,
  expired_time: undefined,
  count: 1,
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload
 */
export function transformFormDataToPayload(
  data: RedemptionFormValues
): RedemptionFormData {
  const isPackage = data.reward_type === 'model_token_package'
  const models = (data.package_models_text || '')
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
  return {
    name: data.name,
    quota: isPackage ? 0 : parseQuotaFromDollars(data.quota_dollars),
    expired_time: data.expired_time
      ? Math.floor(data.expired_time.getTime() / 1000)
      : 0,
    count: data.count || 1,
    reward_type: data.reward_type,
    package_models: isPackage ? models : undefined,
    package_tokens: isPackage ? data.package_tokens || 0 : undefined,
    package_input_ratio: isPackage ? data.package_input_ratio ?? 1 : undefined,
    package_output_ratio: isPackage
      ? data.package_output_ratio ?? 1
      : undefined,
    package_cache_ratio: isPackage ? data.package_cache_ratio ?? 1 : undefined,
    package_duration_seconds: isPackage
      ? Math.round((data.package_duration_days || 0) * 86400)
      : undefined,
  }
}

/**
 * Transform redemption data to form defaults
 */
export function transformRedemptionToFormDefaults(
  redemption: Redemption
): RedemptionFormValues {
  const rewardType =
    redemption.reward_type === 'model_token_package'
      ? 'model_token_package'
      : 'quota'
  let modelsText = ''
  if (redemption.package_models?.length) {
    modelsText = redemption.package_models.join(', ')
  } else if (redemption.package_models_json) {
    try {
      const parsed = JSON.parse(redemption.package_models_json)
      if (Array.isArray(parsed)) modelsText = parsed.join(', ')
    } catch {
      modelsText = redemption.package_models_json
    }
  }
  return {
    name: redemption.name,
    reward_type: rewardType,
    quota_dollars: quotaUnitsToDollars(redemption.quota),
    package_models_text: modelsText,
    package_tokens: redemption.package_tokens || 0,
    package_input_ratio: redemption.package_input_ratio ?? 1,
    package_output_ratio: redemption.package_output_ratio ?? 1,
    package_cache_ratio: redemption.package_cache_ratio ?? 1,
    package_duration_days: redemption.package_duration_seconds
      ? Math.round(redemption.package_duration_seconds / 86400)
      : 0,
    expired_time:
      redemption.expired_time > 0
        ? new Date(redemption.expired_time * 1000)
        : undefined,
    count: 1,
  }
}
