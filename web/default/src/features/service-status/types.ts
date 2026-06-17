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

export type ServiceStatusKind =
  | 'normal'
  | 'degraded'
  | 'outage'
  | 'no_traffic'
  | 'only_probe'
  | 'low_confidence'
  | 'unknown'

export type ServiceSignalState =
  | 'observed'
  | 'not_observed'
  | 'not_configured'

export type ServiceConfidence = 'none' | 'low' | 'high'

export interface ServiceStatusSummary {
  total_channels: number
  normal: number
  degraded: number
  outage: number
  no_traffic: number
  only_probe: number
  low_confidence: number
  unknown: number
  active_alerts: number
}

export interface ServiceStatusSignals {
  real_traffic: ServiceSignalState
  probe: ServiceSignalState
  connectivity: ServiceSignalState
}

export interface ServiceStatusSeriesPoint {
  ts: number
  status: ServiceStatusKind
  request_count: number
  success_rate: number
  avg_latency_ms: number
  avg_ttft_ms: number
}

export interface ServiceStatusChannel {
  channel_id: number
  channel_name: string
  channel_type: number
  group: string
  configured_status: number
  status: ServiceStatusKind
  confidence: ServiceConfidence
  request_count: number
  success_rate: number
  avg_latency_ms: number
  avg_ttft_ms: number
  last_observed_at: number
  configured_models: string[]
  observed_models: string[]
  signals: ServiceStatusSignals
  series: ServiceStatusSeriesPoint[]
}

export interface ServiceStatusAlert {
  severity: 'critical' | 'warning'
  channel_id: number
  channel_name: string
  status: ServiceStatusKind
  request_count: number
  success_rate: number
  avg_latency_ms: number
  last_observed_at: number
}

export interface ServiceStatusData {
  summary: ServiceStatusSummary
  channels: ServiceStatusChannel[]
  alerts?: ServiceStatusAlert[]
  window_hours: number
  bucket_seconds: number
  updated_at: number
}

export interface ServiceStatusResponse {
  success: boolean
  message?: string
  data: ServiceStatusData
}
