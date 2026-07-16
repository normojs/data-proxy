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
export type ModelTokenPackageSku = {
  id: number
  name: string
  description?: string
  models?: string[]
  models_json?: string
  total_tokens: number
  input_ratio: number
  output_ratio: number
  cache_ratio: number
  priority: number
  duration_seconds: number
  price_quota: number
  status: string
  sort_order: number
  created_by?: number
  created_at?: number
  updated_at?: number
}

export type ModelTokenPackageSkuPayload = {
  name: string
  description?: string
  models: string[]
  total_tokens: number
  input_ratio: number
  output_ratio: number
  cache_ratio: number
  priority: number
  duration_seconds: number
  price_quota: number
  status: 'enabled' | 'disabled'
  sort_order: number
}

export type ApiResponse<T = unknown> = {
  success: boolean
  message?: string
  data?: T
}
