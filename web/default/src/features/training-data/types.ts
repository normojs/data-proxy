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

export type ApiResponse<T> = {
  success: boolean
  message?: string
  data?: T
}

export type PaginatedData<T> = {
  items: T[]
  page: number
  page_size: number
  total: number
}

export type PaginatedResponse<T> = ApiResponse<PaginatedData<T>>

export type TrainingDatasetVersion = {
  id: number
  name: string
  version: string
  status: string
  output_format: string
  provider: string
  bucket: string
  storage_key: string
  sha256: string
  size_bytes: number
  sample_count: number
  source_scope_json: string
  redaction_policy_json: string
  build_manifest_json: string
  last_error: string
  built_at: number
  expires_at: number
  created_at: number
  updated_at: number
}

export type TrainingSample = {
  id: number
  dataset_version_id: number
  request_id: string
  capture_id: number
  artifact_id: number
  model_name: string
  source_hash: string
  redaction_status: string
  quality_score: number
  review_status: string
  review_comment: string
  reviewed_by: number
  reviewed_at: number
  tags_json: string
  metadata_json: string
  created_at: number
  updated_at: number
}

export type TrainingDatasetListParams = {
  p?: number
  page_size?: number
  name?: string
  status?: string
}

export type TrainingSampleListParams = {
  p?: number
  page_size?: number
  dataset_version_id?: number
  request_id?: string
  model_name?: string
  min_quality_score?: number
  review_status?: string
}

export type BuildTrainingDatasetPayload = {
  name?: string
  version?: string
  limit?: number
  max_decoded_bundle_bytes?: number
  include_errored?: boolean
}

export type BuildTrainingDatasetResponse = {
  dataset: TrainingDatasetVersion
  samples: number
  skipped: number
  errors?: string[]
}

export type TrainingSamplePreview = {
  sample: TrainingSample
  dataset: TrainingDatasetVersion
  line: Record<string, unknown>
}
