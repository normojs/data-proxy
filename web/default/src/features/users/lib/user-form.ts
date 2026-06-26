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
import { quotaUnitsToDollars } from '@/lib/format'
import { DEFAULT_GROUP } from '../constants'
import { type UserFormData, type User } from '../types'

// ============================================================================
// Form Schema
// ============================================================================

export const userFormSchema = z.object({
  username: z.string().min(1, 'Username is required'),
  display_name: z.string().optional(),
  password: z.string().optional(),
  role: z.number().optional(),
  quota_dollars: z.number().min(0).optional(),
  group: z.string().optional(),
  token_groups: z.array(z.string()).optional(),
  remark: z.string().optional(),
})

export type UserFormValues = z.infer<typeof userFormSchema>

// ============================================================================
// Form Defaults
// ============================================================================

export const USER_FORM_DEFAULT_VALUES: UserFormValues = {
  username: '',
  display_name: '',
  password: '',
  role: 1, // Default to common user
  quota_dollars: 0,
  group: DEFAULT_GROUP,
  token_groups: [],
  remark: '',
}

export function parseUserTokenGroups(value?: string | null): string[] {
  const raw = (value || '').trim()
  if (!raw) return []

  try {
    const parsed = JSON.parse(raw)
    if (Array.isArray(parsed)) {
      return Array.from(
        new Set(
          parsed
            .map((item) => (typeof item === 'string' ? item.trim() : ''))
            .filter(Boolean)
        )
      )
    }
  } catch {
    /* fall back to comma separated values */
  }

  return Array.from(
    new Set(
      raw
        .split(/[,，\n]/)
        .map((item) => item.trim())
        .filter(Boolean)
    )
  )
}

// ============================================================================
// Form Data Transformation
// ============================================================================

/**
 * Transform form data to API payload
 */
export function transformFormDataToPayload(
  data: UserFormValues,
  userId?: number
): UserFormData & { id?: number } {
  const payload: UserFormData & { id?: number } = {
    username: data.username,
    display_name: data.display_name || data.username,
    password: data.password || undefined,
  }

  payload.group = data.group || DEFAULT_GROUP
  payload.token_groups = data.token_groups || []

  // For create: send initial role and group restrictions
  if (userId === undefined) {
    payload.role = data.role || 1 // Default to common user
  } else {
    // For update: quota is adjusted atomically via /api/user/manage, not sent here
    payload.remark = data.remark || undefined
    payload.id = userId
  }

  return payload
}

/**
 * Transform user data to form defaults
 */
export function transformUserToFormDefaults(user: User): UserFormValues {
  return {
    username: user.username,
    display_name: user.display_name,
    password: '',
    role: user.role,
    quota_dollars: quotaUnitsToDollars(user.quota),
    group: user.group || DEFAULT_GROUP,
    token_groups: parseUserTokenGroups(user.token_groups),
    remark: user.remark || '',
  }
}
