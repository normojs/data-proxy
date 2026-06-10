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
import { createFileRoute } from '@tanstack/react-router'
import { Wallet } from '@/features/wallet'

const arraySearchSchema = z
  .preprocess((value) => {
    if (value == null || value === '') return undefined
    return Array.isArray(value) ? value : [value]
  }, z.array(z.string()).optional())
  .catch([])

const walletSearchSchema = z.object({
  show_history: z.boolean().optional(),
  ledgerPage: z.number().optional().catch(1),
  ledgerPageSize: z.number().optional().catch(undefined),
  ledgerFilter: z.string().optional().catch(''),
  ledgerSourceKind: arraySearchSchema.optional(),
  ledgerEventType: arraySearchSchema.optional(),
  ledgerBillingSource: arraySearchSchema.optional(),
  ledgerUsageKind: arraySearchSchema.optional(),
  ledgerRequestId: z.string().optional().catch(''),
  ledgerSourceId: z.string().optional().catch(''),
  ledgerTokenId: z.string().optional().catch(''),
  ledgerStartTime: z.number().optional(),
  ledgerEndTime: z.number().optional(),
})

export const Route = createFileRoute('/_authenticated/wallet/')({
  component: RouteComponent,
  validateSearch: walletSearchSchema,
})

function RouteComponent() {
  const { show_history } = Route.useSearch()
  return <Wallet initialShowHistory={show_history} />
}
