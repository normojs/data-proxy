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
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { QUOTA_TYPE_VALUES } from '../constants'
import type { PricingActualPrice, PricingModel, TokenUnit } from '../types'
import { stripTrailingZeros } from './price'

export function actualPriceAmount(
  actual: PricingActualPrice | undefined,
  model: PricingModel,
  tokenUnit: TokenUnit
): number | undefined {
  if (!actual) return undefined
  if (model.quota_type === QUOTA_TYPE_VALUES.REQUEST) {
    return actual.effective_price_per_request
  }
  return tokenUnit === 'K'
    ? actual.effective_price_per_1k_tokens
    : actual.effective_price_per_1m_tokens
}

export function actualPriceUnitLabel(
  model: PricingModel,
  tokenUnit: TokenUnit,
  requestLabel: string
): string {
  if (model.quota_type === QUOTA_TYPE_VALUES.REQUEST) {
    return requestLabel
  }
  return tokenUnit === 'K' ? '1K tokens' : '1M tokens'
}

export function formatActualPriceValue(
  amount: number | null | undefined
): string {
  if (amount == null || !Number.isFinite(amount)) {
    return '—'
  }
  return stripTrailingZeros(
    formatBillingCurrencyFromUSD(amount, {
      digitsLarge: 4,
      digitsSmall: 8,
      abbreviate: false,
    })
  )
}

export function formatActualPriceCount(value: number | null | undefined) {
  if (value == null || !Number.isFinite(value)) {
    return '0'
  }
  return new Intl.NumberFormat().format(value)
}

export function actualPriceWindowLabel(
  actual: PricingActualPrice | undefined,
  fallback: string
): string {
  const seconds = actual?.window_seconds
  if (!seconds || seconds <= 0) {
    return fallback
  }
  if (seconds % 3600 === 0) {
    return `${seconds / 3600}h`
  }
  if (seconds % 60 === 0) {
    return `${seconds / 60}m`
  }
  return `${seconds}s`
}
