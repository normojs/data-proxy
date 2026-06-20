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
import { useState, useMemo } from 'react'
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth-store'
import { useNotificationStore } from '@/stores/notification-store'
import {
  type EnterpriseQuotaRequestNotification,
  getEnterpriseQuotaRequestNotifications,
  getNotice,
  markEnterpriseQuotaRequestNotificationsRead,
} from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { useStatus } from '@/hooks/use-status'

export type NotificationTab = 'notice' | 'announcements' | 'approvals'

const APPROVAL_NOTIFICATION_PAGE_SIZE = 20

function hashString(input: string): string {
  let hash = 0
  if (!input) return '0'

  for (let i = 0; i < input.length; i += 1) {
    const chr = input.charCodeAt(i)
    hash = (hash << 5) - hash + chr
    hash |= 0
  }

  return hash.toString(36)
}

/**
 * Generate a unique key for an announcement
 * Prefer backend id, fall back to a content hash so edits register
 */
export function getAnnouncementKey(item: Record<string, unknown>): string {
  if (!item) return ''

  if (item.id !== undefined && item.id !== null) {
    return `id:${item.id}`
  }

  const fingerprint = JSON.stringify({
    publishDate: (item?.publishDate as string) || '',
    content: ((item?.content as string) || '').trim(),
    extra: ((item?.extra as string) || '').trim(),
    type: (item?.type as string) || '',
    title: ((item?.title as string) || '').trim(),
    link: ((item?.link as string) || '').trim(),
  })
  return `hash:${hashString(fingerprint)}`
}

/**
 * Hook to manage notifications (Notice + Announcements)
 * Provides unread counts and read status management
 */
export function useNotifications() {
  const queryClient = useQueryClient()
  const user = useAuthStore((state) => state.auth.user)
  const [popoverOpen, setPopoverOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<NotificationTab>('notice')
  const [approvalUnreadOnly, setApprovalUnreadOnly] = useState(false)

  // Fetch Notice from API
  const {
    data: noticeResponse,
    isLoading: noticeLoading,
    refetch: refetchNotice,
  } = useQuery({
    queryKey: ['notice'],
    queryFn: getNotice,
    staleTime: 1000 * 60 * 5, // 5 minutes
  })

  // Fetch Announcements from status
  const { status, loading: statusLoading } = useStatus()
  const announcementsEnabled = status?.announcements_enabled ?? false
  const enterpriseGovernanceEnabled =
    status?.enterprise_governance_enabled === true
  const rawAnnouncements: Record<string, unknown>[] = announcementsEnabled
    ? ((status?.announcements || []) as Record<string, unknown>[]).slice(0, 20)
    : []

  // Notification store
  const {
    lastReadNotice,
    markNoticeRead,
    markAnnouncementsRead,
    dismissAnnouncementPopups,
    isAnnouncementRead,
    isAnnouncementPopupDismissed,
  } = useNotificationStore()

  const announcementIsRead = (key: string) => {
    return isAnnouncementRead(key)
  }

  const announcements = useMemo(
    () =>
      rawAnnouncements.map((item) => {
        const notificationKey = getAnnouncementKey(item)
        return {
          ...item,
          mustRead: item.mustRead === true,
          popup: item.popup === true,
          notificationKey,
          read: announcementIsRead(notificationKey),
        }
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [rawAnnouncements, isAnnouncementRead]
  )

  const {
    data: approvalNotificationsResponse,
    isLoading: approvalsLoading,
    refetch: refetchApprovals,
    fetchNextPage: fetchNextApprovalNotificationsPage,
    hasNextPage: hasMoreApprovalNotifications,
    isFetchingNextPage: approvalsFetchingNextPage,
  } = useInfiniteQuery({
    queryKey: [
      'notifications',
      'enterprise-quota-requests',
      { unreadOnly: approvalUnreadOnly },
    ],
    queryFn: ({ pageParam }) =>
      getEnterpriseQuotaRequestNotifications({
        page: pageParam,
        page_size: APPROVAL_NOTIFICATION_PAGE_SIZE,
        unread_only: approvalUnreadOnly || undefined,
      }),
    initialPageParam: 1,
    getNextPageParam: (lastPage) => {
      if (!lastPage.success || !lastPage.data?.has_more) return undefined
      return (lastPage.data.page || 1) + 1
    },
    enabled: Boolean(user && enterpriseGovernanceEnabled),
    staleTime: 1000 * 30,
    refetchInterval: popoverOpen ? false : 1000 * 60,
    retry: false,
  })

  const approvalNotifications: EnterpriseQuotaRequestNotification[] =
    approvalNotificationsResponse?.pages.flatMap((page) =>
      page.success ? (page.data?.items ?? []) : []
    ) ?? []

  const approvalNotificationsUnread =
    approvalNotificationsResponse?.pages.find((page) => page.success)?.data
      ?.unread_count ?? 0

  const markApprovalsReadMutation = useMutation({
    mutationFn: markEnterpriseQuotaRequestNotificationsRead,
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ['notifications', 'enterprise-quota-requests'],
      })
    },
  })

  // Extract notice content
  const noticeContent = noticeResponse?.success
    ? (noticeResponse.data || '').trim()
    : ''

  // Calculate unread counts
  const unreadCounts = useMemo(() => {
    const noticeUnread =
      noticeContent && noticeContent !== lastReadNotice ? 1 : 0

    const announcementsUnread = announcements.filter(
      (item) => !item.read
    ).length

    return {
      notice: noticeUnread,
      announcements: announcementsUnread,
      approvals: approvalNotificationsUnread,
      total: noticeUnread + announcementsUnread + approvalNotificationsUnread,
    }
  }, [
    noticeContent,
    lastReadNotice,
    announcements,
    approvalNotificationsUnread,
  ])

  const popupAnnouncement = useMemo(() => {
    return (
      announcements.find(
        (item) =>
          item.mustRead &&
          item.popup &&
          !item.read &&
          !isAnnouncementPopupDismissed(item.notificationKey as string)
      ) ?? null
    )
  }, [announcements, isAnnouncementPopupDismissed])

  const markAnnouncementsAsRead = (keys?: string[]) => {
    const targetKeys =
      keys ??
      announcements
        .filter((item) => !item.read)
        .map((item) => item.notificationKey as string)
    const uniqueKeys = [...new Set(targetKeys.filter(Boolean))]
    if (uniqueKeys.length > 0) {
      markAnnouncementsRead(uniqueKeys)
    }
  }

  const dismissAnnouncementPopupsLocal = (keys: string[]) => {
    const uniqueKeys = [...new Set(keys.filter(Boolean))]
    if (uniqueKeys.length > 0) {
      dismissAnnouncementPopups(uniqueKeys)
    }
  }

  // Handle popover open
  const handleOpenPopover = (tab?: NotificationTab) => {
    const nextTab = tab || activeTab

    // Mark currently visible content as read when opening the notification center
    if (noticeContent) {
      markNoticeRead(noticeContent)
    }

    setActiveTab(nextTab)
    setPopoverOpen(true)
  }

  const handlePopoverOpenChange = (open: boolean) => {
    if (open) {
      handleOpenPopover(activeTab)
      return
    }

    setPopoverOpen(false)
  }

  // Handle tab change - mark announcements as read when switching to that tab
  const handleTabChange = (tab: NotificationTab) => {
    setActiveTab(tab)
  }

  const markApprovalsAsRead = (keys?: string[]) => {
    const targetKeys =
      keys ??
      approvalNotifications.filter((item) => !item.read).map((item) => item.key)
    const uniqueKeys = [...new Set(targetKeys.filter(Boolean))]
    if (uniqueKeys.length > 0) {
      markApprovalsReadMutation.mutate(uniqueKeys)
    }
  }

  return {
    // Data
    notice: noticeContent,
    announcements,
    approvalNotifications,
    approvalNotificationsEnabled: Boolean(user && enterpriseGovernanceEnabled),
    approvalAuditLinksEnabled: Boolean(
      user && user.role >= ROLE.ADMIN && enterpriseGovernanceEnabled
    ),
    popupAnnouncement,
    loading: noticeLoading || statusLoading,
    approvalsLoading,
    approvalsUnreadOnly: approvalUnreadOnly,
    setApprovalsUnreadOnly: setApprovalUnreadOnly,
    hasMoreApprovalNotifications: Boolean(hasMoreApprovalNotifications),
    approvalsFetchingNextPage,

    // Unread counts
    unreadCount: unreadCounts.total,
    unreadNoticeCount: unreadCounts.notice,
    unreadAnnouncementsCount: unreadCounts.announcements,
    unreadApprovalCount: unreadCounts.approvals,

    // Popover state
    popoverOpen,
    setPopoverOpen: handlePopoverOpenChange,
    activeTab,
    setActiveTab: handleTabChange,

    // Actions
    openPopover: handleOpenPopover,
    closePopover: () => setPopoverOpen(false),
    markAnnouncementsAsRead,
    markApprovalsAsRead,
    loadMoreApprovalNotifications: fetchNextApprovalNotificationsPage,
    dismissAnnouncementPopups: dismissAnnouncementPopupsLocal,
    refetchNotice,
    refetchApprovals,
  }
}
