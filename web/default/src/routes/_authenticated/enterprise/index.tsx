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
import z from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { ROLE } from '@/lib/roles'
import { Enterprise } from '@/features/enterprise'

const enterpriseSearchSchema = z.object({
  tab: z
    .enum([
      'overview',
      'organization',
      'projects',
      'policy-groups',
      'quota-policies',
      'quota-requests',
      'notifications',
      'webhooks',
      'deliveries',
      'usage',
      'audit',
    ])
    .optional()
    .catch(undefined),
  quota_request_id: z.number().optional().catch(undefined),
  quota_request_status: z.string().optional().catch(''),
  audit_target_type: z.string().optional().catch(''),
  audit_target_id: z.number().optional().catch(undefined),
  audit_action: z.string().optional().catch(''),
})

export const Route = createFileRoute('/_authenticated/enterprise/')({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()

    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({
        to: '/403',
      })
    }
  },
  validateSearch: enterpriseSearchSchema,
  component: Enterprise,
})
