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
*/
import z from 'zod'
import { createFileRoute } from '@tanstack/react-router'
import { QuotaRequests } from '@/features/quota-requests'

const quotaRequestsSearchSchema = z.object({
  quota_request_id: z.number().optional().catch(undefined),
  status: z.string().optional().catch(''),
  project_id: z.number().optional().catch(undefined),
  target_type: z.string().optional().catch(''),
  target_id: z.number().optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/quota-requests/')({
  validateSearch: quotaRequestsSearchSchema,
  component: QuotaRequests,
})
