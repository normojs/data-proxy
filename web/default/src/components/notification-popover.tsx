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
import type { TFunction } from 'i18next'
import { Bell, Megaphone } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getAnnouncementColorClass } from '@/lib/colors'
import { formatDateTimeObject } from '@/lib/time'
import { cn } from '@/lib/utils'
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
  activeTab: 'notice' | 'announcements'
  onTabChange: (tab: 'notice' | 'announcements') => void
  notice: string
  announcements: AnnouncementItem[]
  loading: boolean
  onMarkAllAnnouncementsAsRead?: () => void
  onMarkAnnouncementRead?: (key: string) => void
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
  loading,
  onMarkAllAnnouncementsAsRead,
  onMarkAnnouncementRead,
  className,
}: NotificationPopoverProps) {
  const { t } = useTranslation()
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
          <PopoverTitle>{t('System Announcements')}</PopoverTitle>
          <p className='text-muted-foreground text-xs'>
            {t('Latest platform updates and notices')}
          </p>
        </PopoverHeader>

        <Tabs
          value={activeTab}
          onValueChange={onTabChange as (value: string) => void}
        >
          <TabsList className='grid w-full grid-cols-2'>
            <TabsTrigger value='notice' className='gap-1.5'>
              <Bell className='size-3.5' />
              {t('Notice')}
            </TabsTrigger>
            <TabsTrigger value='announcements' className='gap-1.5'>
              <Megaphone className='size-3.5' />
              {t('Timeline')}
            </TabsTrigger>
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
