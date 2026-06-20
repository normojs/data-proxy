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
import { Link } from '@tanstack/react-router'
import type { TFunction } from 'i18next'
import { Bell, ClipboardCheck, Megaphone, ExternalLink } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type {
  ApprovalNotification,
  ConnectedAppRequestNotification,
} from '@/lib/api'
import { getAnnouncementColorClass } from '@/lib/colors'
import { formatDateTimeObject } from '@/lib/time'
import { cn } from '@/lib/utils'
import type { NotificationTab } from '@/hooks/use-notifications'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Markdown } from '@/components/ui/markdown'
import {
  Popover,
  PopoverContent,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  buildApprovalNotificationAuditSearch,
  buildApprovalNotificationOpenLink,
} from './notification-approval-links'

interface AnnouncementItem {
  notificationKey?: string
  read?: boolean
  mustRead?: boolean
  popup?: boolean
  type?: string
  content?: string
  extra?: string
  publishDate?: string | Date
}

interface NotificationPopoverProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  unreadCount: number
  activeTab: NotificationTab
  onTabChange: (tab: NotificationTab) => void
  notice: string
  announcements: AnnouncementItem[]
  approvalNotifications?: ApprovalNotification[]
  showApprovals?: boolean
  showApprovalAuditLinks?: boolean
  loading: boolean
  approvalsLoading?: boolean
  approvalsUnreadOnly?: boolean
  approvalsHasMore?: boolean
  approvalsLoadingMore?: boolean
  onMarkAllAnnouncementsAsRead?: () => void
  onMarkAnnouncementRead?: (key: string) => void
  onMarkAllApprovalsAsRead?: () => void
  onMarkApprovalRead?: (key: string) => void
  onApprovalsUnreadOnlyChange?: (value: boolean) => void
  onLoadMoreApprovals?: () => void
  className?: string
}

interface RequiredAnnouncementDialogProps {
  announcement?: AnnouncementItem | null
  onDismiss?: (key: string) => void
  onMarkRead?: (key: string) => void
  onMarkAllRead?: () => void
}

/**
 * Get relative time string from a date
 */
function getRelativeTime(publishDate: string | Date, t: TFunction): string {
  if (!publishDate) return ''

  const now = new Date()
  const pubDate = new Date(publishDate)

  // If invalid date, return original string
  if (isNaN(pubDate.getTime()))
    return typeof publishDate === 'string' ? publishDate : ''

  const diffMs = now.getTime() - pubDate.getTime()
  const diffSeconds = Math.floor(diffMs / 1000)
  const diffMinutes = Math.floor(diffSeconds / 60)
  const diffHours = Math.floor(diffMinutes / 60)
  const diffDays = Math.floor(diffHours / 24)
  const diffWeeks = Math.floor(diffDays / 7)
  const diffMonths = Math.floor(diffDays / 30)
  const diffYears = Math.floor(diffDays / 365)

  // If future time, show specific date
  if (diffMs < 0) return formatDateTimeObject(pubDate)

  // Return relative time based on difference
  if (diffSeconds < 60) return t('Just now')
  if (diffMinutes < 60)
    return diffMinutes === 1
      ? t('1 minute ago')
      : t('{{count}} minutes ago', { count: diffMinutes })
  if (diffHours < 24)
    return diffHours === 1
      ? t('1 hour ago')
      : t('{{count}} hours ago', { count: diffHours })
  if (diffDays < 7)
    return diffDays === 1
      ? t('1 day ago')
      : t('{{count}} days ago', { count: diffDays })
  if (diffWeeks < 4)
    return diffWeeks === 1
      ? t('1 week ago')
      : t('{{count}} weeks ago', { count: diffWeeks })
  if (diffMonths < 12)
    return diffMonths === 1
      ? t('1 month ago')
      : t('{{count}} months ago', { count: diffMonths })
  if (diffYears < 2) return t('1 year ago')

  // Over 2 years, show specific date
  return formatDateTimeObject(pubDate)
}

/**
 * Announcement status dot indicator
 */
function AnnouncementDot({ type }: { type?: string }) {
  return (
    <span
      className={cn(
        'mt-1.5 inline-block size-2 shrink-0 rounded-full',
        getAnnouncementColorClass(type)
      )}
    />
  )
}

/**
 * Empty state component
 */
function EmptyState({
  icon,
  title,
  description,
}: {
  icon: React.ReactNode
  title: string
  description?: string
}) {
  return (
    <Empty className='min-h-48 border-0 p-4'>
      <EmptyHeader>
        <EmptyMedia variant='icon'>{icon}</EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        {description ? (
          <EmptyDescription>{description}</EmptyDescription>
        ) : null}
      </EmptyHeader>
    </Empty>
  )
}

/**
 * Notice tab content
 */
function NoticeContent({
  notice,
  loading,
  t,
}: {
  notice: string
  loading: boolean
  t: TFunction
}) {
  if (loading) {
    return (
      <EmptyState
        icon={<Bell />}
        title={t('Loading...')}
        description={t('Latest platform updates and notices')}
      />
    )
  }

  if (!notice) {
    return (
      <EmptyState icon={<Bell />} title={t('No announcements at this time')} />
    )
  }

  return (
    <ScrollArea className='h-[min(52vh,28rem)] pr-3'>
      <Markdown>{notice}</Markdown>
    </ScrollArea>
  )
}

/**
 * Announcements tab content
 */
function AnnouncementsContent({
  announcements,
  loading,
  onMarkAllAnnouncementsAsRead,
  onMarkAnnouncementRead,
  t,
}: {
  announcements: AnnouncementItem[]
  loading: boolean
  onMarkAllAnnouncementsAsRead?: () => void
  onMarkAnnouncementRead?: (key: string) => void
  t: TFunction
}) {
  if (loading) {
    return (
      <EmptyState
        icon={<Megaphone />}
        title={t('Loading...')}
        description={t('Latest platform updates and notices')}
      />
    )
  }

  if (announcements.length === 0) {
    return (
      <EmptyState icon={<Megaphone />} title={t('No system announcements')} />
    )
  }

  const hasUnread = announcements.some((item) => !item.read)

  return (
    <ScrollArea className='h-[min(52vh,28rem)] pr-3'>
      <div className='flex flex-col'>
        <div className='flex items-center justify-between gap-2 pb-2'>
          <span className='text-muted-foreground text-xs'>
            {t('{{count}} unread announcements', {
              count: announcements.filter((item) => !item.read).length,
            })}
          </span>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={!hasUnread}
            onClick={onMarkAllAnnouncementsAsRead}
          >
            {t('Mark all as read')}
          </Button>
        </div>
        {announcements.map((item, idx) => {
          const publishDate = item.publishDate
            ? new Date(item.publishDate)
            : null
          const relativeTime = publishDate
            ? getRelativeTime(publishDate, t)
            : ''
          const absoluteTime = publishDate
            ? formatDateTimeObject(publishDate)
            : ''

          return (
            <div key={idx}>
              <div className='py-3'>
                <div className='flex items-start gap-3'>
                  <AnnouncementDot type={item.type} />
                  <div className='flex min-w-0 flex-1 flex-col gap-2'>
                    <div className='flex flex-wrap items-center gap-1.5'>
                      {item.mustRead ? (
                        <Badge variant='secondary'>
                          {t('Required reading')}
                        </Badge>
                      ) : null}
                      {!item.read ? (
                        <Badge variant='outline'>{t('Unread')}</Badge>
                      ) : (
                        <Badge variant='outline'>{t('Read')}</Badge>
                      )}
                    </div>
                    <div className='text-sm'>
                      <Markdown>{item.content || ''}</Markdown>
                    </div>

                    {item.extra ? (
                      <div className='text-muted-foreground text-xs'>
                        <Markdown>{item.extra}</Markdown>
                      </div>
                    ) : null}

                    {absoluteTime ? (
                      <div className='text-muted-foreground text-xs'>
                        {relativeTime ? `${relativeTime} • ` : null}
                        {absoluteTime}
                      </div>
                    ) : null}
                    {!item.read && item.notificationKey ? (
                      <div>
                        <Button
                          type='button'
                          size='sm'
                          variant='outline'
                          onClick={() =>
                            onMarkAnnouncementRead?.(item.notificationKey!)
                          }
                        >
                          {t('Mark as read')}
                        </Button>
                      </div>
                    ) : null}
                  </div>
                </div>
              </div>
              {idx < announcements.length - 1 ? <Separator /> : null}
            </div>
          )
        })}
      </div>
    </ScrollArea>
  )
}

function approvalStatusLabel(status: string) {
  switch (status) {
    case 'approved':
      return 'Approved'
    case 'rejected':
      return 'Rejected'
    case 'withdrawn':
      return 'Withdrawn'
    case 'expired':
      return 'Expired'
    case 'expiring_soon':
      return 'Expiring soon'
    default:
      return 'Pending'
  }
}

function approvalStatusClassName(status: string) {
  switch (status) {
    case 'approved':
      return 'border-emerald-500/40 text-emerald-700 dark:text-emerald-300'
    case 'rejected':
      return 'border-destructive/40 text-destructive'
    case 'withdrawn':
      return 'border-muted-foreground/40 text-muted-foreground'
    case 'expired':
      return 'border-amber-500/40 text-amber-700 dark:text-amber-300'
    case 'expiring_soon':
      return 'border-orange-500/40 text-orange-700 dark:text-orange-300'
    default:
      return 'border-blue-500/40 text-blue-700 dark:text-blue-300'
  }
}

function formatApprovalNumber(value: number | undefined) {
  return new Intl.NumberFormat().format(value ?? 0)
}

function approvalNotificationTitle(item: ApprovalNotification, t: TFunction) {
  return item.title_key ? t(item.title_key) : t(item.title)
}

function approvalNotificationContent(item: ApprovalNotification, t: TFunction) {
  return item.content_key
    ? t(item.content_key, item.content_params ?? {})
    : t(item.content)
}

function isConnectedAppNotification(
  item: ApprovalNotification
): item is ConnectedAppRequestNotification {
  return item.kind === 'connected_app_request'
}

function connectedAppOpenSearch(item: ConnectedAppRequestNotification) {
  return {
    tab: 'requests',
    connected_app_request_id: item.request_id,
  } as const
}

function connectedAppAuditSearch(item: ConnectedAppRequestNotification) {
  return {
    tab: 'audit',
    connected_app_request_id: item.request_id,
  } as const
}

function ApprovalsContent({
  approvals,
  loading,
  onMarkAllApprovalsAsRead,
  onMarkApprovalRead,
  unreadOnly,
  onUnreadOnlyChange,
  hasMore,
  loadingMore,
  onLoadMore,
  showAuditLinks,
  onOpenChange,
  t,
}: {
  approvals: ApprovalNotification[]
  loading: boolean
  onMarkAllApprovalsAsRead?: () => void
  onMarkApprovalRead?: (key: string) => void
  unreadOnly?: boolean
  onUnreadOnlyChange?: (value: boolean) => void
  hasMore?: boolean
  loadingMore?: boolean
  onLoadMore?: () => void
  showAuditLinks?: boolean
  onOpenChange: (open: boolean) => void
  t: TFunction
}) {
  if (loading) {
    return (
      <EmptyState
        icon={<ClipboardCheck />}
        title={t('Loading...')}
        description={t('Quota approval updates and audit events')}
      />
    )
  }

  if (approvals.length === 0) {
    return (
      <EmptyState
        icon={<ClipboardCheck />}
        title={t('No approval updates')}
        description={t('Quota request status changes will appear here.')}
      />
    )
  }

  const unreadCount = approvals.filter((item) => !item.read).length
  const hasUnread = unreadCount > 0

  return (
    <ScrollArea className='h-[min(52vh,28rem)] pr-3'>
      <div className='flex flex-col'>
        <div className='flex items-center justify-between gap-2 pb-2'>
          <span className='text-muted-foreground text-xs'>
            {t('{{count}} unread approval updates', { count: unreadCount })}
          </span>
          <div className='flex items-center gap-2'>
            <Button
              type='button'
              size='sm'
              variant={unreadOnly ? 'secondary' : 'outline'}
              onClick={() => onUnreadOnlyChange?.(!unreadOnly)}
            >
              {unreadOnly ? t('Show all') : t('Unread only')}
            </Button>
            <Button
              type='button'
              size='sm'
              variant='outline'
              disabled={!hasUnread}
              onClick={onMarkAllApprovalsAsRead}
            >
              {t('Mark all as read')}
            </Button>
          </div>
        </div>
        {approvals.map((item, idx) => {
          const createdAt = item.created_at
            ? new Date(item.created_at * 1000)
            : null
          const isConnectedApp = isConnectedAppNotification(item)
          const expiresAt =
            !isConnectedApp && item.expires_at
              ? new Date(item.expires_at * 1000)
              : null
          const relativeTime = createdAt ? getRelativeTime(createdAt, t) : ''
          const absoluteTime = createdAt ? formatDateTimeObject(createdAt) : ''
          const openLink = !isConnectedApp
            ? buildApprovalNotificationOpenLink(item, showAuditLinks)
            : null
          const auditSearch = !isConnectedApp
            ? buildApprovalNotificationAuditSearch(item)
            : null

          return (
            <div key={item.key}>
              <div className='py-3'>
                <div className='flex items-start gap-3'>
                  <span
                    className={cn(
                      'mt-1.5 inline-block size-2 shrink-0 rounded-full',
                      item.read ? 'bg-muted-foreground/40' : 'bg-primary'
                    )}
                  />
                  <div className='flex min-w-0 flex-1 flex-col gap-2'>
                    <div className='flex flex-wrap items-center gap-1.5'>
                      <Badge
                        variant='outline'
                        className={approvalStatusClassName(item.status)}
                      >
                        {t(approvalStatusLabel(item.status))}
                      </Badge>
                      {!item.read ? (
                        <Badge variant='outline'>{t('Unread')}</Badge>
                      ) : (
                        <Badge variant='outline'>{t('Read')}</Badge>
                      )}
                    </div>
                    <div className='min-w-0'>
                      <div className='truncate text-sm font-medium'>
                        {approvalNotificationTitle(item, t)}
                      </div>
                      <p className='text-muted-foreground mt-1 text-xs'>
                        {approvalNotificationContent(item, t)}
                      </p>
                    </div>
                    <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs'>
                      {isConnectedApp ? (
                        <>
                          <span>
                            {t('App')} {item.slug}
                          </span>
                          <span>
                            {t('Scopes')} {item.requested_scopes.length}
                          </span>
                          {item.applicant_name ? (
                            <span>
                              {t('Applicant')} {item.applicant_name}
                            </span>
                          ) : null}
                          {item.audit_log_id ? (
                            <span>
                              {t('Audit')} #{item.audit_log_id}
                            </span>
                          ) : null}
                        </>
                      ) : (
                        <>
                          <span>
                            {t('Delta')}{' '}
                            {formatApprovalNumber(item.limit_delta)}
                          </span>
                          {expiresAt ? (
                            <span>
                              {t('Expires')} {formatDateTimeObject(expiresAt)}
                            </span>
                          ) : null}
                          {item.audit_log_id ? (
                            <span>
                              {t('Audit')} #{item.audit_log_id}
                            </span>
                          ) : null}
                        </>
                      )}
                    </div>
                    {absoluteTime ? (
                      <div className='text-muted-foreground text-xs'>
                        {relativeTime ? `${relativeTime} • ` : null}
                        {absoluteTime}
                      </div>
                    ) : null}
                    <div className='flex flex-wrap gap-2'>
                      {isConnectedApp ? (
                        <Button
                          render={
                            showAuditLinks ? (
                              <Link
                                to='/system-settings/operations/$section'
                                params={{ section: 'connected-apps' }}
                                search={connectedAppOpenSearch(item)}
                                onClick={() => {
                                  if (!item.read) onMarkApprovalRead?.(item.key)
                                  onOpenChange(false)
                                }}
                              />
                            ) : (
                              <Link
                                to='/profile'
                                onClick={() => {
                                  if (!item.read) onMarkApprovalRead?.(item.key)
                                  onOpenChange(false)
                                }}
                              />
                            )
                          }
                          type='button'
                          size='sm'
                          variant='outline'
                        >
                          <ExternalLink className='size-3.5' />
                          {t('Open')}
                        </Button>
                      ) : (
                        <Button
                          render={
                            <Link
                              to={openLink!.target}
                              search={openLink!.search}
                              onClick={() => {
                                if (!item.read) onMarkApprovalRead?.(item.key)
                                onOpenChange(false)
                              }}
                            />
                          }
                          type='button'
                          size='sm'
                          variant='outline'
                        >
                          <ExternalLink className='size-3.5' />
                          {t('Open')}
                        </Button>
                      )}
                      {showAuditLinks && item.audit_log_id ? (
                        <Button
                          render={
                            isConnectedApp ? (
                              <Link
                                to='/system-settings/operations/$section'
                                params={{ section: 'connected-apps' }}
                                search={connectedAppAuditSearch(item)}
                                onClick={() => {
                                  if (!item.read) onMarkApprovalRead?.(item.key)
                                  onOpenChange(false)
                                }}
                              />
                            ) : (
                              <Link
                                to='/enterprise'
                                search={auditSearch!}
                                onClick={() => {
                                  if (!item.read) onMarkApprovalRead?.(item.key)
                                  onOpenChange(false)
                                }}
                              />
                            )
                          }
                          type='button'
                          size='sm'
                          variant='ghost'
                        >
                          {t('Audit')}
                        </Button>
                      ) : null}
                      {!item.read ? (
                        <Button
                          type='button'
                          size='sm'
                          variant='outline'
                          onClick={() => onMarkApprovalRead?.(item.key)}
                        >
                          {t('Mark as read')}
                        </Button>
                      ) : null}
                    </div>
                  </div>
                </div>
              </div>
              {idx < approvals.length - 1 ? <Separator /> : null}
            </div>
          )
        })}
        {hasMore ? (
          <div className='flex justify-center py-3'>
            <Button
              type='button'
              size='sm'
              variant='outline'
              disabled={loadingMore}
              onClick={onLoadMore}
            >
              {loadingMore ? t('Loading...') : t('Load more')}
            </Button>
          </div>
        ) : null}
      </div>
    </ScrollArea>
  )
}

/**
 * Required-reading popup for announcements that must be acknowledged explicitly.
 */
export function RequiredAnnouncementDialog({
  announcement,
  onDismiss,
  onMarkRead,
  onMarkAllRead,
}: RequiredAnnouncementDialogProps) {
  const { t } = useTranslation()

  if (!announcement) return null

  const notificationKey = announcement.notificationKey
  const handleDismiss = () => {
    if (notificationKey) {
      onDismiss?.(notificationKey)
    }
  }
  const publishDate = announcement.publishDate
    ? new Date(announcement.publishDate)
    : null
  const relativeTime = publishDate ? getRelativeTime(publishDate, t) : ''
  const absoluteTime = publishDate ? formatDateTimeObject(publishDate) : ''

  return (
    <AlertDialog
      open={true}
      onOpenChange={(open) => {
        if (!open) {
          handleDismiss()
        }
      }}
    >
      <AlertDialogContent className='max-w-[calc(100%-2rem)] sm:max-w-lg'>
        <AlertDialogHeader>
          <AlertDialogMedia>
            <Megaphone className='size-5' />
          </AlertDialogMedia>
          <AlertDialogTitle>{t('Required announcement')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t(
              'Close this popup to stop showing it on this device, or mark announcements as read.'
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className='space-y-3'>
          <div className='flex flex-wrap items-center gap-1.5'>
            <Badge variant='secondary'>{t('Required reading (popup)')}</Badge>
            {absoluteTime ? (
              <span className='text-muted-foreground text-xs'>
                {relativeTime ? `${relativeTime} • ` : null}
                {absoluteTime}
              </span>
            ) : null}
          </div>

          <ScrollArea className='max-h-[min(50vh,24rem)] pr-3'>
            <div className='text-sm'>
              <Markdown>{announcement.content || ''}</Markdown>
            </div>

            {announcement.extra ? (
              <div className='text-muted-foreground mt-3 text-xs'>
                <Markdown>{announcement.extra}</Markdown>
              </div>
            ) : null}
          </ScrollArea>
        </div>

        <AlertDialogFooter>
          <Button type='button' variant='outline' onClick={handleDismiss}>
            {t('Close')}
          </Button>
          <Button type='button' variant='secondary' onClick={onMarkAllRead}>
            {t('Mark all as read')}
          </Button>
          <AlertDialogAction
            disabled={!notificationKey}
            onClick={() => {
              if (notificationKey) {
                onMarkRead?.(notificationKey)
              }
            }}
          >
            {t('I have read this announcement')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

/**
 * Notification popover with Notice and Announcements tabs
 */
export function NotificationPopover({
  open,
  onOpenChange,
  unreadCount,
  activeTab,
  onTabChange,
  notice,
  announcements,
  approvalNotifications = [],
  showApprovals = false,
  showApprovalAuditLinks = false,
  loading,
  approvalsLoading,
  approvalsUnreadOnly,
  approvalsHasMore,
  approvalsLoadingMore,
  onMarkAllAnnouncementsAsRead,
  onMarkAnnouncementRead,
  onMarkAllApprovalsAsRead,
  onMarkApprovalRead,
  onApprovalsUnreadOnlyChange,
  onLoadMoreApprovals,
  className,
}: NotificationPopoverProps) {
  const { t } = useTranslation()
  const activeValue =
    activeTab === 'approvals' && !showApprovals ? 'notice' : activeTab
  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger
        render={
          <Button
            variant='ghost'
            size='icon'
            className={cn('relative size-9', className)}
            aria-label={t('Notifications')}
          />
        }
      >
        <Bell className='size-[1.2rem]' />
        {unreadCount > 0 ? (
          <Badge
            variant='destructive'
            className='absolute -top-1 -right-1 flex h-5 min-w-5 items-center justify-center px-1 text-[10px] font-semibold tabular-nums'
          >
            {unreadCount > 99 ? '99+' : unreadCount}
          </Badge>
        ) : null}
      </PopoverTrigger>

      <PopoverContent
        align='end'
        sideOffset={8}
        className='w-[min(26rem,calc(100vw-1rem))] gap-3 p-3'
      >
        <PopoverHeader className='gap-1 px-1'>
          <PopoverTitle>{t('Notifications')}</PopoverTitle>
          <p className='text-muted-foreground text-xs'>
            {t('Platform notices, timeline updates, and approval status')}
          </p>
        </PopoverHeader>

        <Tabs
          value={activeValue}
          onValueChange={onTabChange as (value: string) => void}
        >
          <TabsList
            className={cn(
              'grid w-full',
              showApprovals ? 'grid-cols-3' : 'grid-cols-2'
            )}
          >
            <TabsTrigger value='notice' className='gap-1.5'>
              <Bell className='size-3.5' />
              {t('Notice')}
            </TabsTrigger>
            <TabsTrigger value='announcements' className='gap-1.5'>
              <Megaphone className='size-3.5' />
              {t('Timeline')}
            </TabsTrigger>
            {showApprovals ? (
              <TabsTrigger value='approvals' className='gap-1.5'>
                <ClipboardCheck className='size-3.5' />
                {t('Approvals')}
              </TabsTrigger>
            ) : null}
          </TabsList>

          <TabsContent value='notice' className='mt-2'>
            <NoticeContent notice={notice} loading={loading} t={t} />
          </TabsContent>

          <TabsContent value='announcements' className='mt-2'>
            <AnnouncementsContent
              announcements={announcements}
              loading={loading}
              onMarkAllAnnouncementsAsRead={onMarkAllAnnouncementsAsRead}
              onMarkAnnouncementRead={onMarkAnnouncementRead}
              t={t}
            />
          </TabsContent>

          {showApprovals ? (
            <TabsContent value='approvals' className='mt-2'>
              <ApprovalsContent
                approvals={approvalNotifications}
                loading={approvalsLoading ?? loading}
                onMarkAllApprovalsAsRead={onMarkAllApprovalsAsRead}
                onMarkApprovalRead={onMarkApprovalRead}
                unreadOnly={approvalsUnreadOnly}
                onUnreadOnlyChange={onApprovalsUnreadOnlyChange}
                hasMore={approvalsHasMore}
                loadingMore={approvalsLoadingMore}
                onLoadMore={onLoadMoreApprovals}
                showAuditLinks={showApprovalAuditLinks}
                onOpenChange={onOpenChange}
                t={t}
              />
            </TabsContent>
          ) : null}
        </Tabs>

        <div className='flex justify-end'>
          <Button size='sm' onClick={() => onOpenChange(false)}>
            {t('Close')}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
