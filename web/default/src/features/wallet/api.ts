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
import { api } from '@/lib/api'
import type {
  RedemptionRequest,
  PaymentRequest,
  AmountRequest,
  AffiliateTransferRequest,
  ApiResponse,
  TopupInfoResponse,
  RedemptionResponse,
  AmountResponse,
  PaymentResponse,
  StripePaymentResponse,
  WechatPayPaymentResponse,
  AffiliateCodeResponse,
  AffiliateTransferResponse,
  BillingHistoryResponse,
  CompleteOrderRequest,
  CreemPaymentRequest,
  CreemPaymentResponse,
  WaffoPaymentRequest,
  WaffoPaymentResponse,
  WaffoPancakePaymentRequest,
  WaffoPancakePaymentResponse,
  BillingEvent,
  BillingEventListParams,
  PaginatedData,
} from './types'

// ============================================================================
// Wallet API Functions
// ============================================================================

/**
 * Check if API response is successful
 */
export function isApiSuccess(response: ApiResponse): boolean {
  return response.success === true || response.message === 'success'
}

/**
 * Get topup configuration info
 */
export async function getTopupInfo(): Promise<TopupInfoResponse> {
  const res = await api.get('/api/user/topup/info')
  return res.data
}

/**
 * Redeem a topup code
 */
export async function redeemTopupCode(
  request: RedemptionRequest
): Promise<RedemptionResponse> {
  const res = await api.post('/api/user/topup', request)
  return res.data
}

/**
 * Calculate payment amount for regular payment
 */
export async function calculateAmount(
  request: AmountRequest
): Promise<AmountResponse> {
  const res = await api.post('/api/user/amount', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Calculate payment amount for Stripe payment
 */
export async function calculateStripeAmount(
  request: AmountRequest
): Promise<AmountResponse> {
  const res = await api.post('/api/user/stripe/amount', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request regular payment
 */
export async function requestPayment(
  request: PaymentRequest
): Promise<PaymentResponse> {
  const res = await api.post('/api/user/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return {
    ...res.data,
    url: res.data.url || (res as unknown as { url?: string }).url,
  }
}

/**
 * Request Stripe payment
 */
export async function requestStripePayment(
  request: PaymentRequest
): Promise<StripePaymentResponse> {
  const res = await api.post('/api/user/stripe/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request direct WeChat Pay Native QR payment
 */
export async function requestWechatPayPayment(
  request: AmountRequest
): Promise<WechatPayPaymentResponse> {
  const res = await api.post('/api/user/wechat-pay/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request Creem payment
 */
export async function requestCreemPayment(
  request: CreemPaymentRequest
): Promise<CreemPaymentResponse> {
  const res = await api.post('/api/user/creem/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request Waffo payment
 */
export async function requestWaffoPayment(
  request: WaffoPaymentRequest
): Promise<WaffoPaymentResponse> {
  const res = await api.post('/api/user/waffo/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Calculate payment amount for Waffo Pancake payment
 */
export async function calculateWaffoPancakeAmount(
  request: AmountRequest
): Promise<AmountResponse> {
  const res = await api.post('/api/user/waffo-pancake/amount', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Request Waffo Pancake payment
 */
export async function requestWaffoPancakePayment(
  request: WaffoPancakePaymentRequest
): Promise<WaffoPancakePaymentResponse> {
  const res = await api.post('/api/user/waffo-pancake/pay', request, {
    skipBusinessError: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * Get affiliate code
 */
export async function getAffiliateCode(): Promise<AffiliateCodeResponse> {
  const res = await api.get('/api/user/aff')
  return res.data
}

/**
 * Transfer affiliate quota to balance
 */
export async function transferAffiliateQuota(
  request: AffiliateTransferRequest
): Promise<AffiliateTransferResponse> {
  const res = await api.post('/api/user/aff_transfer', request)
  return res.data
}

/**
 * Get billing history for current user
 */
export async function getUserBillingHistory(
  page: number,
  pageSize: number,
  keyword?: string
): Promise<ApiResponse<BillingHistoryResponse>> {
  const params = new URLSearchParams({
    p: page.toString(),
    page_size: pageSize.toString(),
  })
  if (keyword) {
    params.append('keyword', keyword)
  }
  const res = await api.get(`/api/user/topup/self?${params.toString()}`)
  return res.data
}

/**
 * Get billing history for all users (admin only)
 */
export async function getAllBillingHistory(
  page: number,
  pageSize: number,
  keyword?: string
): Promise<ApiResponse<BillingHistoryResponse>> {
  const params = new URLSearchParams({
    p: page.toString(),
    page_size: pageSize.toString(),
  })
  if (keyword) {
    params.append('keyword', keyword)
  }
  const res = await api.get(`/api/user/topup?${params.toString()}`)
  return res.data
}

/**
 * Complete a pending order (admin only)
 */
export async function completeOrder(
  request: CompleteOrderRequest
): Promise<ApiResponse> {
  const res = await api.post('/api/user/topup/complete', request)
  return res.data
}

/**
 * Get unified billing ledger events for the current user.
 *
 * The backend defaults to scope=self; wallet callers intentionally do not pass
 * scope=all so ordinary users can only inspect their own ledger.
 */
export async function getUserBillingEvents(
  params: BillingEventListParams
): Promise<ApiResponse<PaginatedData<BillingEvent>>> {
  const query = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value == null || value === '') return
    query.set(key, String(value))
  })
  const res = await api.get(
    `/api/billing/events/${query.toString() ? `?${query.toString()}` : ''}`
  )
  return res.data
}

export type ModelTokenPackage = {
  id: number
  user_id: number
  name: string
  models?: string[]
  models_json?: string
  total_tokens: number
  remaining_tokens: number
  used_tokens: number
  input_ratio: number
  output_ratio: number
  cache_ratio: number
  priority: number
  status: string
  expired_at: number
  source: string
  remark?: string
  created_at?: number
  updated_at?: number
}

export type ModelTokenPackageLedger = {
  id: number
  package_id: number
  user_id: number
  request_id: string
  model: string
  prompt_tokens: number
  completion_tokens: number
  cache_tokens: number
  input_ratio: number
  output_ratio: number
  cache_ratio: number
  delta_tokens: number
  reason: string
  created_at: number
}

export type QuotaOverviewPackageItem = {
  id: number
  name: string
  models?: string[]
  remaining_tokens: number
  total_tokens: number
  status: string
  expired_at: number
}

export type QuotaOverviewSubscriptionItem = {
  id: number
  plan_id: number
  amount_total: number
  amount_used: number
  amount_remaining: number
  status: string
  end_time: number
  source: string
}

export type QuotaOverviewAPIKeyItem = {
  id: number
  name: string
  remain_quota: number
  used_quota: number
  unlimited_quota: boolean
  quota_hard_limit_enabled: boolean
  status: number
  expired_time: number
}

export type QuotaOverview = {
  wallet: {
    quota: number
    used_quota: number
    request_count: number
    unit: string
    status: string
  }
  model_token_packages: {
    active_count: number
    remaining_tokens: number
    used_tokens: number
    total_packages: number
    unit: string
    status: string
    top_packages: QuotaOverviewPackageItem[]
  }
  subscriptions: {
    active_count: number
    remaining_quota: number
    total_quota: number
    used_quota: number
    unit: string
    status: string
    items: QuotaOverviewSubscriptionItem[]
  }
  api_key_hard_limits: {
    limited_count: number
    remaining_quota: number
    unit: string
    status: string
    items: QuotaOverviewAPIKeyItem[]
  }
  units: {
    wallet: string
    model_token_package: string
    subscription: string
    api_key_hard_limit: string
  }
  links: {
    wallet: string
    model_token_packages: string
    subscriptions: string
    api_keys: string
  }
}

export async function getSelfQuotaOverview(): Promise<
  ApiResponse<QuotaOverview>
> {
  const res = await api.get('/api/user/quota-overview')
  return res.data
}

export async function getSelfModelTokenPackages(
  includeInactive = true
): Promise<ApiResponse<ModelTokenPackage[]>> {
  const res = await api.get(
    `/api/user/model-token-packages?include_inactive=${includeInactive}`
  )
  return res.data
}

export async function getSelfModelTokenPackageLedger(
  packageId: number,
  params: { p?: number; page_size?: number } = {}
): Promise<ApiResponse<PaginatedData<ModelTokenPackageLedger>>> {
  const query = new URLSearchParams()
  if (params.p) query.set('p', String(params.p))
  if (params.page_size) query.set('page_size', String(params.page_size))
  const qs = query.toString()
  const res = await api.get(
    `/api/user/model-token-packages/${packageId}/ledger${qs ? `?${qs}` : ''}`
  )
  return res.data
}
