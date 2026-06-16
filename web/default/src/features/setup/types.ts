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
export type SetupUsageMode = 'external' | 'self' | 'demo'

export interface SetupStatus {
  status: boolean
  root_init: boolean
  database_type: string
  database_configured?: boolean
  database_source?: 'env' | 'runtime-config' | 'sqlite-default' | string
  redis_enabled?: boolean
  redis_configured?: boolean
  redis_source?: 'env' | 'runtime-config' | string
  runtime_config_loaded?: boolean
  runtime_config_restart_required?: boolean
  runtime_config_path?: string
  restart_required?: boolean
  restart_supported?: boolean
  restart_scheduled?: boolean
  restart_delay_ms?: number
  // Some backends also echo mode flags; they are optional here.
  SelfUseModeEnabled?: boolean
  DemoSiteEnabled?: boolean
}

export type SetupDatabaseType = 'sqlite' | 'mysql' | 'postgres'

export interface SetupRuntimeConfigPayload {
  database_type: SetupDatabaseType
  sql_dsn?: string
  sqlite_path?: string
  redis_enabled: boolean
  redis_conn_string?: string
}

export interface SetupFormValues {
  username: string
  password: string
  confirmPassword: string
  usageMode: SetupUsageMode
}

export interface SetupResponse {
  success: boolean
  message?: string
  data?: SetupStatus
}
