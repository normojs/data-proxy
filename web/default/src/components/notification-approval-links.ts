import type { EnterpriseQuotaRequestNotification } from '@/lib/api'

export type ApprovalNotificationOpenTarget = '/enterprise' | '/quota-requests'

export type ApprovalNotificationOpenLink = {
  target: ApprovalNotificationOpenTarget
  search:
    | {
        tab: 'quota-requests'
        quota_request_id: number
        quota_request_status?: string
      }
    | {
        quota_request_id: number
        status?: string
      }
}

export type ApprovalNotificationAuditSearch = {
  tab: 'audit'
  audit_target_type: 'quota_request'
  audit_target_id: number
  audit_action?: string
}

export function approvalAuditAction(status: string) {
  switch (status) {
    case 'approved':
      return 'quota_request.approve'
    case 'rejected':
      return 'quota_request.reject'
    case 'withdrawn':
      return 'quota_request.withdraw'
    case 'expired':
      return 'quota_request.expire'
    default:
      return undefined
  }
}

export function buildApprovalNotificationOpenLink(
  item: Pick<EnterpriseQuotaRequestNotification, 'quota_request_id' | 'status'>,
  showAuditLinks?: boolean
): ApprovalNotificationOpenLink {
  if (item.status === 'pending') {
    return {
      target: '/enterprise',
      search: {
        tab: 'quota-requests',
        quota_request_id: item.quota_request_id,
        quota_request_status: 'pending',
      },
    }
  }

  if (item.status === 'expiring_soon') {
    if (showAuditLinks) {
      return {
        target: '/enterprise',
        search: {
          tab: 'quota-requests',
          quota_request_id: item.quota_request_id,
        },
      }
    }

    return {
      target: '/quota-requests',
      search: {
        quota_request_id: item.quota_request_id,
      },
    }
  }

  return {
    target: '/quota-requests',
    search: {
      quota_request_id: item.quota_request_id,
      status: item.status,
    },
  }
}

export function buildApprovalNotificationAuditSearch(
  item: Pick<EnterpriseQuotaRequestNotification, 'quota_request_id' | 'status'>
): ApprovalNotificationAuditSearch {
  return {
    tab: 'audit',
    audit_target_type: 'quota_request',
    audit_target_id: item.quota_request_id,
    audit_action: approvalAuditAction(item.status),
  }
}
