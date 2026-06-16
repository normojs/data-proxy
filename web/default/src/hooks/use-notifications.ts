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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { useNotificationStore } from '@/stores/notification-store'
import { api } from '@/lib/api'
import { getNotice } from '@/lib/api'
import { useStatus } from '@/hooks/use-status'

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
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const user = useAuthStore((state) => state.auth.user)
  const [popoverOpen, setPopoverOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<'notice' | 'announcements'>(
    'notice'
  )

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
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const rawAnnouncements: Record<string, unknown>[] = announcementsEnabled
    ? ((status?.announcements || []) as Record<string, unknown>[]).slice(0, 20)
    : []

  // Notification store
  const {
    lastReadNotice,
    markNoticeRead,
    markAnnouncementsRead,
    isAnnouncementRead,
  } = useNotificationStore()

  const readStateQuery = useQuery({
    queryKey: ['notification-read-state', user?.id],
    enabled: Boolean(user?.id),
    queryFn: async () => {
      const res = await api.get<{
        success: boolean
        data?: { announcement_keys?: string[] }
      }>('/api/notifications/read-state', {
        skipErrorHandler: true,
        skipBusinessError: true,
      })
      if (!res.data.success) return []
      return res.data.data?.announcement_keys ?? []
    },
    staleTime: 1000 * 60,
  })

  const serverReadKeys = useMemo(
    () => new Set(readStateQuery.data ?? []),
    [readStateQuery.data]
  )

  const announcementIsRead = (key: string) => {
    if (user?.id && readStateQuery.isSuccess) {
      return serverReadKeys.has(key)
    }
    return isAnnouncementRead(key)
  }

  const announcements = useMemo(
    () =>
      rawAnnouncements.map((item) => {
        const notificationKey = getAnnouncementKey(item)
        return {
          ...item,
          notificationKey,
          read: announcementIsRead(notificationKey),
        }
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [rawAnnouncements, serverReadKeys, isAnnouncementRead, user?.id]
  )

  const markAnnouncementsMutation = useMutation({
    mutationFn: async (keys: string[]) => {
      if (!user?.id) return { success: true }
      const res = await api.post(
        '/api/notifications/read',
        { announcement_keys: keys },
        { skipErrorHandler: true, skipBusinessError: true }
      )
      return res.data as { success: boolean; message?: string }
    },
    onSuccess: (response, keys) => {
      markAnnouncementsRead(keys)
      if (user?.id) {
        queryClient.invalidateQueries({
          queryKey: ['notification-read-state', user.id],
        })
      }
      if (!response.success) {
        toast.error(response.message || t('Failed to mark as read'))
      }
    },
    onError: (_error, keys) => {
      markAnnouncementsRead(keys)
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
      total: noticeUnread + announcementsUnread,
    }
  }, [noticeContent, lastReadNotice, announcements])

  const markAnnouncementsAsRead = (keys?: string[]) => {
    const targetKeys =
      keys ??
      announcements
        .filter((item) => !item.read)
        .map((item) => item.notificationKey as string)
    const uniqueKeys = [...new Set(targetKeys.filter(Boolean))]
    if (uniqueKeys.length > 0) {
      markAnnouncementsMutation.mutate(uniqueKeys)
    }
  }

  // Handle popover open
  const handleOpenPopover = (tab?: 'notice' | 'announcements') => {
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
  const handleTabChange = (tab: 'notice' | 'announcements') => {
    setActiveTab(tab)
  }

  return {
    // Data
    notice: noticeContent,
    announcements,
    loading: noticeLoading || statusLoading,

    // Unread counts
    unreadCount: unreadCounts.total,
    unreadNoticeCount: unreadCounts.notice,
    unreadAnnouncementsCount: unreadCounts.announcements,

    // Popover state
    popoverOpen,
    setPopoverOpen: handlePopoverOpenChange,
    activeTab,
    setActiveTab: handleTabChange,

    // Actions
    openPopover: handleOpenPopover,
    closePopover: () => setPopoverOpen(false),
    markAnnouncementsAsRead,
    refetchNotice,
  }
}
