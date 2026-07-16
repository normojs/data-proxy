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

// ============================================================================
// Redemption Schema & Types
// ============================================================================

export const redemptionSchema = z.object({
  id: z.number(),
  user_id: z.number(),
  name: z.string(),
  key: z.string(),
  status: z.number(), // 1: enabled, 2: disabled, 3: used
  quota: z.number(),
  created_time: z.number(),
  redeemed_time: z.number(),
  expired_time: z.number(), // 0 for never expires
  used_user_id: z.number(),
  reward_type: z.string().optional(),
  package_models_json: z.string().optional(),
  package_tokens: z.number().optional(),
  package_input_ratio: z.number().optional(),
  package_output_ratio: z.number().optional(),
  package_cache_ratio: z.number().optional(),
  package_expired_at: z.number().optional(),
  package_duration_seconds: z.number().optional(),
  result_package_id: z.number().optional(),
  package_models: z.array(z.string()).optional(),
})

export type Redemption = z.infer<typeof redemptionSchema>

// ============================================================================
// API Request/Response Types
// ============================================================================

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}

export interface GetRedemptionsParams {
  p?: number
  page_size?: number
}

export interface GetRedemptionsResponse {
  success: boolean
  message?: string
  data?: {
    items: Redemption[]
    total: number
    page: number
    page_size: number
  }
}

export interface SearchRedemptionsParams {
  keyword?: string
  p?: number
  page_size?: number
}

export interface RedemptionFormData {
  id?: number
  name: string
  quota: number
  expired_time: number
  count?: number // Only for create
  status?: number // Only for status update
  reward_type?: string
  package_models?: string[]
  package_tokens?: number
  package_input_ratio?: number
  package_output_ratio?: number
  package_cache_ratio?: number
  package_duration_seconds?: number
}

// ============================================================================
// Dialog Types
// ============================================================================

export type RedemptionsDialogType = 'create' | 'update' | 'delete' | 'view'
