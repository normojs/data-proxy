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
import assert from 'node:assert/strict'
import { pathToFileURL } from 'node:url'

const moduleUrl = pathToFileURL(
  new URL('../src/components/notification-approval-links.ts', import.meta.url)
    .pathname
).href

const {
  approvalAuditAction,
  buildApprovalNotificationAuditSearch,
  buildApprovalNotificationOpenLink,
} = await import(moduleUrl)

function notification(status, id = 42) {
  return {
    quota_request_id: id,
    status,
  }
}

assert.deepEqual(
  buildApprovalNotificationOpenLink(notification('pending'), true),
  {
    target: '/enterprise',
    search: {
      tab: 'quota-requests',
      quota_request_id: 42,
      quota_request_status: 'pending',
    },
  },
  'admin pending notification should open enterprise quota request filter'
)

assert.deepEqual(
  buildApprovalNotificationOpenLink(notification('approved'), false),
  {
    target: '/quota-requests',
    search: {
      quota_request_id: 42,
      status: 'approved',
    },
  },
  'applicant decision notification should open own quota request filter'
)

assert.deepEqual(
  buildApprovalNotificationOpenLink(notification('expiring_soon'), true),
  {
    target: '/enterprise',
    search: {
      tab: 'quota-requests',
      quota_request_id: 42,
    },
  },
  'admin expiring soon notification should open enterprise quota request detail filter'
)

assert.deepEqual(
  buildApprovalNotificationOpenLink(notification('expiring_soon'), false),
  {
    target: '/quota-requests',
    search: {
      quota_request_id: 42,
    },
  },
  'applicant expiring soon notification should open own quota request detail filter without synthetic status'
)

assert.deepEqual(
  buildApprovalNotificationAuditSearch(notification('expired')),
  {
    tab: 'audit',
    audit_target_type: 'quota_request',
    audit_target_id: 42,
    audit_action: 'quota_request.expire',
  },
  'expired notification audit link should filter quota request expiry audit events'
)

for (const [status, action] of [
  ['approved', 'quota_request.approve'],
  ['rejected', 'quota_request.reject'],
  ['withdrawn', 'quota_request.withdraw'],
  ['expired', 'quota_request.expire'],
  ['pending', undefined],
  ['expiring_soon', undefined],
]) {
  assert.equal(approvalAuditAction(status), action, `audit action for ${status}`)
}

console.log('approval notification link smoke ok')
