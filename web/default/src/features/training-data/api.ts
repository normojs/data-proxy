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
  ApiResponse,
  BuildTrainingDatasetPayload,
  BuildTrainingDatasetResponse,
  PaginatedResponse,
  TrainingDatasetListParams,
  TrainingDatasetVersion,
  TrainingSample,
  TrainingSampleListParams,
  TrainingSamplePreview,
} from './types'

function buildQueryParams(params: Record<string, unknown>): string {
  const query = new URLSearchParams()
  Object.entries(params).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return
    query.set(key, String(value))
  })
  return query.toString()
}

export async function getTrainingDatasets(
  params: TrainingDatasetListParams = {}
): Promise<PaginatedResponse<TrainingDatasetVersion>> {
  const res = await api.get(
    `/api/training/datasets?${buildQueryParams({
      p: params.p || 1,
      page_size: params.page_size || 10,
      name: params.name,
      status: params.status,
    })}`
  )
  return res.data
}

export async function buildTrainingDataset(
  payload: BuildTrainingDatasetPayload
): Promise<ApiResponse<BuildTrainingDatasetResponse>> {
  const res = await api.post('/api/training/datasets/build', payload)
  return res.data
}

export async function getTrainingSamples(
  params: TrainingSampleListParams = {}
): Promise<PaginatedResponse<TrainingSample>> {
  const res = await api.get(
    `/api/training/samples?${buildQueryParams({
      p: params.p || 1,
      page_size: params.page_size || 20,
      dataset_version_id: params.dataset_version_id,
      request_id: params.request_id,
      model_name: params.model_name,
      min_quality_score: params.min_quality_score,
      review_status: params.review_status,
    })}`
  )
  return res.data
}

export async function getTrainingSamplePreview(
  id: number
): Promise<ApiResponse<TrainingSamplePreview>> {
  const res = await api.get(`/api/training/samples/${id}/preview`)
  return res.data
}

export async function approveTrainingSample(
  id: number,
  comment = ''
): Promise<ApiResponse<{ sample: TrainingSample }>> {
  const res = await api.post(`/api/training/samples/${id}/approve`, {
    comment,
  })
  return res.data
}

export async function rejectTrainingSample(
  id: number,
  comment = ''
): Promise<ApiResponse<{ sample: TrainingSample }>> {
  const res = await api.post(`/api/training/samples/${id}/reject`, {
    comment,
  })
  return res.data
}

export function getTrainingDatasetExportUrl(id: number): string {
  return `/api/training/datasets/${id}/export`
}
