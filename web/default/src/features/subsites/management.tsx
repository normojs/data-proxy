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
import { useEffect, useMemo, useState } from 'react'
import { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getRouteApi, useNavigate } from '@tanstack/react-router'
import {
  type ColumnDef,
  type SortingState,
  type VisibilityState,
  getCoreRowModel,
  getFacetedRowModel,
  getFacetedUniqueValues,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useMediaQuery } from '@/hooks'
import type { TFunction } from 'i18next'
import {
  Activity as ActivityIcon,
  CircleDollarSign,
  ExternalLink,
  FileText,
  Gauge,
  ListChecks,
  MoreHorizontal,
  Pencil,
  PlayCircle,
  Plus,
  RefreshCw,
  Settings2,
  Trash2,
  UserPlus,
  Users,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  formatCurrencyFromUSD,
  getCurrencyDisplay,
  getCurrencyLabel,
} from '@/lib/currency'
import dayjs from '@/lib/dayjs'
import {
  formatNumber,
  formatQuota,
  formatTimestampToDate,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'
import { cn } from '@/lib/utils'
import { useIsAdmin } from '@/hooks/use-admin'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { DataTableColumnHeader, DataTablePage } from '@/components/data-table'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { SectionPageLayout } from '@/components/layout'
import { LongText } from '@/components/long-text'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import {
  CHANNEL_STATUS,
  CHANNEL_TYPE_OPTIONS,
  CHANNEL_TYPES,
} from '@/features/channels/constants'
import {
  createManagedSubsite,
  createManagedSubsiteChannel,
  deleteManagedSubsiteChannel,
  deleteManagedSubsiteMember,
  getManagedSubsiteActivity,
  getManagedSubsiteChannels,
  getManagedSubsiteMembers,
  getManagedSubsite,
  getManagedSubsites,
  syncManagedSubsiteChannelModels,
  testManagedSubsiteChannel,
  updateManagedSubsite,
  updateManagedSubsiteChannel,
  updateManagedSubsiteChannelBalance,
  updateManagedSubsiteQuotaPolicy,
  upsertManagedSubsiteMember,
} from './api'
import type {
  ManagedSubsite,
  ManagedSubsiteActivity,
  ManagedSubsiteChannel,
  ManagedSubsiteChannelPayload,
  ManagedSubsiteMember,
  ManagedSubsiteMemberUpsertPayload,
  ManagedSubsitePayload,
  PublicSubsite,
  SubsiteMemberRole,
  SubsiteMemberStatus,
  SubsiteQuotaPolicyInfo,
  SubsiteRegistrationPolicy,
  SubsiteRuntimeStatus,
} from './types'

const route = getRouteApi('/_authenticated/dashboard/subsites')

const SUBSITE_STATUS_OPTIONS = ['draft', 'enabled', 'disabled'] as const
const REGISTRATION_POLICY_OPTIONS = ['open', 'invite', 'closed'] as const
const MEMBER_ROLE_OPTIONS = ['owner', 'admin', 'member'] as const
const MEMBER_STATUS_OPTIONS = ['active', 'disabled'] as const
const CHANNEL_STATUS_OPTIONS = [
  CHANNEL_STATUS.ENABLED,
  CHANNEL_STATUS.MANUAL_DISABLED,
] as const

const EMPTY_QUOTA_POLICY: SubsiteQuotaPolicyInfo = {
  site_daily_quota: 0,
  site_window_quota: 0,
  user_daily_quota: 0,
  user_window_quota: 0,
  site_daily_request_limit: 0,
  site_window_request_limit: 0,
  user_daily_request_limit: 0,
  user_window_request_limit: 0,
  site_window_seconds: 0,
  user_window_seconds: 0,
}

function getSubsiteFormSchema(t: TFunction) {
  return z.object({
    slug: z
      .string()
      .trim()
      .min(3, t('Slug must be at least 3 characters'))
      .max(64, t('Slug must be at most 64 characters'))
      .regex(
        /^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$/,
        t('Use lowercase letters, numbers, and hyphens only')
      )
      .refine((value) => !value.includes('--'), {
        message: t('Slug cannot contain consecutive hyphens'),
      }),
    name: z.string().trim().min(1, t('Please enter a name')),
    title: z.string().optional(),
    logo_url: z.string().optional(),
    favicon_url: z.string().optional(),
    theme_color: z
      .string()
      .refine(
        (value) =>
          !value ||
          /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/.test(value),
        { message: t('Use a valid hex color') }
      ),
    status: z.enum(SUBSITE_STATUS_OPTIONS),
    disabled_reason: z.string().optional(),
    starts_at: z.string().optional(),
    ends_at: z.string().optional(),
    registration_policy: z.enum(REGISTRATION_POLICY_OPTIONS),
    invite_code: z.string().optional(),
    email_domain_whitelist: z.string().optional(),
    announcement_icon: z.string().optional(),
    announcement_title: z.string().optional(),
    announcement_body: z.string().optional(),
    announcement_url: z.string().optional(),
    contact_url: z.string().optional(),
    owner_user_id: z.number().int().min(0).optional(),
  })
}

type SubsiteFormValues = z.infer<ReturnType<typeof getSubsiteFormSchema>>

function getQuotaPolicyFormSchema(t: TFunction) {
  return z.object({
    site_daily_quota: z.number().min(0, t('Value must be zero or greater')),
    site_window_quota: z.number().min(0, t('Value must be zero or greater')),
    user_daily_quota: z.number().min(0, t('Value must be zero or greater')),
    user_window_quota: z.number().min(0, t('Value must be zero or greater')),
    site_daily_request_limit: z
      .number()
      .int()
      .min(0, t('Value must be zero or greater')),
    site_window_request_limit: z
      .number()
      .int()
      .min(0, t('Value must be zero or greater')),
    user_daily_request_limit: z
      .number()
      .int()
      .min(0, t('Value must be zero or greater')),
    user_window_request_limit: z
      .number()
      .int()
      .min(0, t('Value must be zero or greater')),
    site_window_seconds: z
      .number()
      .int()
      .min(0, t('Value must be zero or greater')),
    user_window_seconds: z
      .number()
      .int()
      .min(0, t('Value must be zero or greater')),
  })
}

type QuotaPolicyFormValues = z.infer<
  ReturnType<typeof getQuotaPolicyFormSchema>
>

function getMemberFormSchema(t: TFunction) {
  return z.object({
    user_id: z.number().int().min(1, t('Please enter a user ID')),
    role: z.enum(MEMBER_ROLE_OPTIONS),
    status: z.enum(MEMBER_STATUS_OPTIONS),
  })
}

type MemberFormValues = z.infer<ReturnType<typeof getMemberFormSchema>>

function getChannelFormSchema(t: TFunction) {
  return z.object({
    name: z.string().trim().min(1, t('Please enter a channel name')),
    type: z.number().int().min(1, t('Please select a channel type')),
    key: z.string().optional(),
    base_url: z.string().optional(),
    models: z.string().trim().min(1, t('Please enter at least one model')),
    model_display_names: z.string().optional(),
    group: z.string().optional(),
    status: z
      .number()
      .int()
      .refine((value) => CHANNEL_STATUS_OPTIONS.includes(value as 1 | 2), {
        message: t('Unsupported channel status'),
      }),
    priority: z.number().int(),
    weight: z.number().int().min(0, t('Value must be zero or greater')),
    remark: z.string().optional(),
  })
}

type ChannelFormValues = z.infer<ReturnType<typeof getChannelFormSchema>>

function timestampToInput(timestamp?: number) {
  if (!timestamp || timestamp <= 0) return ''
  return dayjs(timestamp * 1000).format('YYYY-MM-DDTHH:mm')
}

function inputToTimestamp(value?: string) {
  if (!value) return 0
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 0
  return Math.floor(date.getTime() / 1000)
}

function subsiteToFormValues(item?: ManagedSubsite): SubsiteFormValues {
  const subsite = item?.subsite
  return {
    slug: subsite?.slug ?? '',
    name: subsite?.name ?? '',
    title: subsite?.title ?? '',
    logo_url: subsite?.logo_url ?? '',
    favicon_url: subsite?.favicon_url ?? '',
    theme_color: subsite?.theme_color ?? '',
    status:
      (subsite?.status as
        | (typeof SUBSITE_STATUS_OPTIONS)[number]
        | undefined) ?? 'draft',
    disabled_reason: subsite?.disabled_reason ?? '',
    starts_at: timestampToInput(subsite?.starts_at),
    ends_at: timestampToInput(subsite?.ends_at),
    registration_policy:
      (subsite?.registration_policy as SubsiteRegistrationPolicy | undefined) ??
      'open',
    invite_code: '',
    email_domain_whitelist: '',
    announcement_icon: subsite?.announcement_icon ?? '',
    announcement_title: subsite?.announcement_title ?? '',
    announcement_body: subsite?.announcement_body ?? '',
    announcement_url: subsite?.announcement_url ?? '',
    contact_url: subsite?.contact_url ?? '',
    owner_user_id: 0,
  }
}

function formValuesToPayload(values: SubsiteFormValues): ManagedSubsitePayload {
  const payload: ManagedSubsitePayload = {
    slug: values.slug.trim().toLowerCase(),
    name: values.name.trim(),
    title: values.title?.trim() ?? '',
    logo_url: values.logo_url?.trim() ?? '',
    favicon_url: values.favicon_url?.trim() ?? '',
    theme_color: values.theme_color?.trim() ?? '',
    status: values.status,
    disabled_reason: values.disabled_reason?.trim() ?? '',
    starts_at: inputToTimestamp(values.starts_at),
    ends_at: inputToTimestamp(values.ends_at),
    registration_policy: values.registration_policy,
    invite_code: values.invite_code?.trim() ?? '',
    email_domain_whitelist: values.email_domain_whitelist?.trim() ?? '',
    announcement_icon: values.announcement_icon?.trim() ?? '',
    announcement_title: values.announcement_title?.trim() ?? '',
    announcement_body: values.announcement_body ?? '',
    announcement_url: values.announcement_url?.trim() ?? '',
    contact_url: values.contact_url?.trim() ?? '',
  }

  if (values.owner_user_id && values.owner_user_id > 0) {
    payload.owner_user_id = values.owner_user_id
  }

  return payload
}

function quotaPolicyToFormValues(
  policy?: SubsiteQuotaPolicyInfo
): QuotaPolicyFormValues {
  const current = policy ?? EMPTY_QUOTA_POLICY
  return {
    site_daily_quota: quotaUnitsToDollars(current.site_daily_quota),
    site_window_quota: quotaUnitsToDollars(current.site_window_quota),
    user_daily_quota: quotaUnitsToDollars(current.user_daily_quota),
    user_window_quota: quotaUnitsToDollars(current.user_window_quota),
    site_daily_request_limit: current.site_daily_request_limit,
    site_window_request_limit: current.site_window_request_limit,
    user_daily_request_limit: current.user_daily_request_limit,
    user_window_request_limit: current.user_window_request_limit,
    site_window_seconds: current.site_window_seconds,
    user_window_seconds: current.user_window_seconds,
  }
}

function quotaFormValuesToPayload(
  values: QuotaPolicyFormValues
): SubsiteQuotaPolicyInfo {
  return {
    site_daily_quota: parseQuotaFromDollars(values.site_daily_quota),
    site_window_quota: parseQuotaFromDollars(values.site_window_quota),
    user_daily_quota: parseQuotaFromDollars(values.user_daily_quota),
    user_window_quota: parseQuotaFromDollars(values.user_window_quota),
    site_daily_request_limit: values.site_daily_request_limit,
    site_window_request_limit: values.site_window_request_limit,
    user_daily_request_limit: values.user_daily_request_limit,
    user_window_request_limit: values.user_window_request_limit,
    site_window_seconds: values.site_window_seconds,
    user_window_seconds: values.user_window_seconds,
  }
}

function runtimeStatusMeta(status: SubsiteRuntimeStatus): {
  label: string
  variant: 'success' | 'warning' | 'danger' | 'neutral'
} {
  switch (status) {
    case 'enabled':
      return { label: 'Enabled', variant: 'success' }
    case 'disabled':
      return { label: 'Disabled', variant: 'warning' }
    case 'not_started':
      return { label: 'Not started', variant: 'neutral' }
    case 'expired':
      return { label: 'Expired', variant: 'danger' }
    default:
      return { label: 'Draft', variant: 'neutral' }
  }
}

function registrationPolicyLabel(policy?: SubsiteRegistrationPolicy) {
  switch (policy) {
    case 'invite':
      return 'Invite only'
    case 'closed':
      return 'Closed'
    default:
      return 'Open registration'
  }
}

function memberRoleLabel(role?: SubsiteMemberRole) {
  switch (role) {
    case 'owner':
      return 'Owner'
    case 'admin':
      return 'Admin'
    default:
      return 'Member'
  }
}

function memberRoleVariant(
  role?: SubsiteMemberRole
): 'success' | 'info' | 'neutral' {
  switch (role) {
    case 'owner':
      return 'success'
    case 'admin':
      return 'info'
    default:
      return 'neutral'
  }
}

function memberStatusMeta(status?: SubsiteMemberStatus): {
  label: string
  variant: 'success' | 'warning'
} {
  return status === 'disabled'
    ? { label: 'Disabled', variant: 'warning' }
    : { label: 'Active', variant: 'success' }
}

function SubsiteLogo({ subsite }: { subsite: PublicSubsite }) {
  const initial =
    (subsite.title || subsite.name || subsite.slug).trim().slice(0, 1) || 'S'

  if (subsite.logo_url) {
    return (
      <img
        src={subsite.logo_url}
        alt=''
        className='size-8 rounded-md border object-cover'
      />
    )
  }

  return (
    <div className='bg-muted text-muted-foreground flex size-8 items-center justify-center rounded-md border text-xs font-semibold'>
      {initial.toUpperCase()}
    </div>
  )
}

function formatValidity(subsite: PublicSubsite) {
  if (!subsite.starts_at && !subsite.ends_at) return '-'
  const startsAt = subsite.starts_at
    ? formatTimestampToDate(subsite.starts_at)
    : 'Any time'
  const endsAt = subsite.ends_at
    ? formatTimestampToDate(subsite.ends_at)
    : 'No end'
  return `${startsAt} - ${endsAt}`
}

function quotaLimitLabel(value?: number) {
  if (!value || value <= 0) return 'Unlimited'
  return formatQuota(value)
}

function callsLimitLabel(value?: number) {
  if (!value || value <= 0) return 'Unlimited'
  return formatNumber(value)
}

function UsageCell({ item }: { item: ManagedSubsite }) {
  const dailyLimit = item.quota_policy?.site_daily_quota ?? 0
  const percentage = dailyLimit > 0 ? (item.today_quota / dailyLimit) * 100 : 0

  return (
    <div className='min-w-[150px] space-y-1.5'>
      <div className='flex items-center justify-between gap-3 text-xs'>
        <span className='font-medium tabular-nums'>
          {formatNumber(item.today_calls)}
        </span>
        <span className='text-muted-foreground tabular-nums'>
          {formatQuota(item.today_quota)}
        </span>
      </div>
      {dailyLimit > 0 ? (
        <Progress value={Math.min(100, percentage)} className='h-1.5' />
      ) : (
        <div className='bg-muted h-1.5 rounded-full' />
      )}
      <div className='text-muted-foreground text-[11px]'>
        {dailyLimit > 0
          ? `${percentage.toFixed(1)}% ${formatQuota(dailyLimit)}`
          : 'No site daily limit'}
      </div>
    </div>
  )
}

function QuotaSummary({ policy }: { policy?: SubsiteQuotaPolicyInfo }) {
  if (!policy) {
    return (
      <StatusBadge label='Not configured' variant='neutral' copyable={false} />
    )
  }

  return (
    <div className='flex min-w-[170px] flex-col gap-1 text-xs'>
      <div className='flex items-center justify-between gap-3'>
        <span className='text-muted-foreground'>Site daily</span>
        <span className='font-medium tabular-nums'>
          {quotaLimitLabel(policy.site_daily_quota)}
        </span>
      </div>
      <div className='flex items-center justify-between gap-3'>
        <span className='text-muted-foreground'>User daily</span>
        <span className='font-medium tabular-nums'>
          {quotaLimitLabel(policy.user_daily_quota)}
        </span>
      </div>
      <div className='flex items-center justify-between gap-3'>
        <span className='text-muted-foreground'>Requests</span>
        <span className='font-medium tabular-nums'>
          {callsLimitLabel(policy.user_window_request_limit)}
        </span>
      </div>
    </div>
  )
}

function ActivityMetricTile(props: {
  label: string
  value: string
  description?: string
  variant?: 'neutral' | 'danger'
}) {
  return (
    <div className='rounded-lg border p-3'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div
        className={cn(
          'mt-1 text-lg leading-6 font-semibold tabular-nums',
          props.variant === 'danger' && 'text-destructive'
        )}
      >
        {props.value}
      </div>
      {props.description && (
        <div className='text-muted-foreground mt-1 truncate text-xs'>
          {props.description}
        </div>
      )}
    </div>
  )
}

function ActivityRecentLogsTable(props: {
  logs: ManagedSubsiteActivity['recent_logs']
}) {
  const { t } = useTranslation()
  if (props.logs.length === 0) {
    return (
      <div className='text-muted-foreground flex h-28 items-center justify-center rounded-lg border text-sm'>
        {t('No recent requests')}
      </div>
    )
  }

  return (
    <div className='rounded-lg border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Time')}</TableHead>
            <TableHead>{t('User')}</TableHead>
            <TableHead>{t('Model')}</TableHead>
            <TableHead>{t('Tokens')}</TableHead>
            <TableHead>{t('Quota')}</TableHead>
            <TableHead className='text-right'>{t('Status')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.logs.map((log) => (
            <TableRow key={log.id}>
              <TableCell className='text-muted-foreground text-xs'>
                {formatTimestampToDate(log.created_at)}
              </TableCell>
              <TableCell>
                <LongText className='max-w-[120px]'>
                  {log.username || '-'}
                </LongText>
              </TableCell>
              <TableCell>
                <LongText className='max-w-[150px] font-medium'>
                  {log.model_name || '-'}
                </LongText>
              </TableCell>
              <TableCell>
                <div className='space-y-0.5 text-xs'>
                  <div className='font-medium tabular-nums'>
                    {formatNumber(log.total_tokens)}
                  </div>
                  <div className='text-muted-foreground'>
                    {formatNumber(log.prompt_tokens)} /{' '}
                    {formatNumber(log.completion_tokens)}
                  </div>
                </div>
              </TableCell>
              <TableCell className='font-medium tabular-nums'>
                {formatQuota(log.quota)}
              </TableCell>
              <TableCell className='text-right'>
                <StatusBadge
                  label={log.status === 'error' ? t('Error') : t('Success')}
                  variant={log.status === 'error' ? 'danger' : 'success'}
                  copyable={false}
                />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

type ColumnsOptions = {
  onActivity: (item: ManagedSubsite) => void
  onChannels: (item: ManagedSubsite) => void
  onEdit: (item: ManagedSubsite) => void
  onMembers: (item: ManagedSubsite) => void
  onQuota: (item: ManagedSubsite) => void
  onRefresh: () => void
}

function useManagedSubsitesColumns(options: ColumnsOptions) {
  const { t } = useTranslation()
  const navigate = useNavigate()

  return useMemo<ColumnDef<ManagedSubsite>[]>(
    () => [
      {
        id: 'id',
        accessorFn: (row) => row.subsite.id,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title='ID' />
        ),
        cell: ({ row }) => (
          <TableId value={row.original.subsite.id} className='w-[64px]' />
        ),
        meta: { label: t('ID'), mobileHidden: true },
      },
      {
        id: 'subsite',
        accessorFn: (row) => row.subsite.name,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Subsite')} />
        ),
        cell: ({ row }) => {
          const subsite = row.original.subsite
          return (
            <div className='flex min-w-[220px] items-center gap-2.5'>
              <SubsiteLogo subsite={subsite} />
              <div className='min-w-0'>
                <div className='flex min-w-0 items-center gap-1.5'>
                  <LongText className='max-w-[170px] font-medium'>
                    {subsite.name}
                  </LongText>
                  <CopyButton
                    value={`/s/${subsite.slug}`}
                    size='icon'
                    tooltip={t('Copy subsite path')}
                    className='size-6'
                  />
                </div>
                <div className='text-muted-foreground flex items-center gap-1 text-xs'>
                  <span className='font-mono'>/{subsite.slug}</span>
                  {subsite.title && subsite.title !== subsite.name && (
                    <span className='truncate'>/ {subsite.title}</span>
                  )}
                </div>
              </div>
            </div>
          )
        },
        enableHiding: false,
        meta: { label: t('Subsite'), mobileTitle: true },
      },
      {
        id: 'runtime_status',
        accessorFn: (row) => row.subsite.runtime_status,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Status')} />
        ),
        cell: ({ row }) => {
          const meta = runtimeStatusMeta(row.original.subsite.runtime_status)
          return (
            <div className='flex flex-col gap-1'>
              <StatusBadge
                label={t(meta.label)}
                variant={meta.variant}
                copyable={false}
              />
              {row.original.subsite.status !==
                row.original.subsite.runtime_status && (
                <span className='text-muted-foreground text-xs'>
                  {t('Configured')}: {row.original.subsite.status}
                </span>
              )}
            </div>
          )
        },
        enableSorting: false,
        meta: { label: t('Status'), mobileBadge: true },
      },
      {
        id: 'owners',
        accessorFn: (row) => row.owner_usernames.join(', '),
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Owners')} />
        ),
        cell: ({ row }) => {
          const item = row.original
          const owners =
            item.owner_usernames.length > 0
              ? item.owner_usernames
              : item.owner_user_ids.map((id) => `#${id}`)
          return (
            <div className='min-w-[140px] space-y-1'>
              <div className='flex flex-wrap gap-1'>
                {owners.slice(0, 2).map((owner) => (
                  <StatusBadge
                    key={owner}
                    label={owner}
                    variant='neutral'
                    copyable={false}
                  />
                ))}
                {owners.length > 2 && (
                  <StatusBadge
                    label={`+${owners.length - 2}`}
                    variant='neutral'
                    copyable={false}
                  />
                )}
              </div>
              <div className='text-muted-foreground text-xs'>
                {t('{{count}} members', { count: item.member_count })}
              </div>
            </div>
          )
        },
        meta: { label: t('Owners') },
      },
      {
        id: 'registration_policy',
        accessorFn: (row) => row.subsite.registration_policy ?? 'open',
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Registration')} />
        ),
        cell: ({ row }) => (
          <StatusBadge
            label={t(
              registrationPolicyLabel(row.original.subsite.registration_policy)
            )}
            variant={
              row.original.subsite.registration_policy === 'closed'
                ? 'warning'
                : 'neutral'
            }
            copyable={false}
          />
        ),
        enableSorting: false,
        meta: { label: t('Registration') },
      },
      {
        id: 'validity',
        accessorFn: (row) => row.subsite.starts_at || row.subsite.ends_at || 0,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Validity')} />
        ),
        cell: ({ row }) => (
          <div className='text-muted-foreground max-w-[240px] text-xs leading-5'>
            {formatValidity(row.original.subsite)}
          </div>
        ),
        meta: { label: t('Validity') },
      },
      {
        id: 'today_usage',
        accessorFn: (row) => row.today_quota,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Today')} />
        ),
        cell: ({ row }) => <UsageCell item={row.original} />,
        meta: { label: t('Today') },
      },
      {
        id: 'quota_policy',
        accessorFn: (row) => row.quota_policy?.site_daily_quota ?? 0,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('Quota Policy')} />
        ),
        cell: ({ row }) => <QuotaSummary policy={row.original.quota_policy} />,
        meta: { label: t('Quota Policy') },
      },
      {
        id: 'actions',
        cell: ({ row }) => {
          const item = row.original
          return (
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button
                    variant='ghost'
                    className='data-popup-open:bg-muted flex h-8 w-8 p-0'
                  />
                }
              >
                <MoreHorizontal className='h-4 w-4' />
                <span className='sr-only'>{t('Open menu')}</span>
              </DropdownMenuTrigger>
              <DropdownMenuContent align='end' className='w-[210px]'>
                <DropdownMenuItem
                  onClick={() => options.onEdit(item)}
                  disabled={!item.can_manage}
                >
                  {t('Edit')}
                  <DropdownMenuShortcut>
                    <Pencil size={16} />
                  </DropdownMenuShortcut>
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => options.onActivity(item)}>
                  {t('Activity')}
                  <DropdownMenuShortcut>
                    <ActivityIcon size={16} />
                  </DropdownMenuShortcut>
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => options.onQuota(item)}
                  disabled={!item.can_manage}
                >
                  {t('Quota Policy')}
                  <DropdownMenuShortcut>
                    <Gauge size={16} />
                  </DropdownMenuShortcut>
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => options.onChannels(item)}
                  disabled={!item.can_manage}
                >
                  {t('Channels')}
                  <DropdownMenuShortcut>
                    <Settings2 size={16} />
                  </DropdownMenuShortcut>
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => options.onMembers(item)}
                  disabled={!item.can_manage}
                >
                  {t('Members')}
                  <DropdownMenuShortcut>
                    <Users size={16} />
                  </DropdownMenuShortcut>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  onClick={() =>
                    void navigate({
                      to: '/usage-logs/$section',
                      params: { section: 'common' },
                      search: {
                        subsiteId: String(item.subsite.id),
                        page: 1,
                      },
                    })
                  }
                >
                  {t('View Logs')}
                  <DropdownMenuShortcut>
                    <FileText size={16} />
                  </DropdownMenuShortcut>
                </DropdownMenuItem>
                <DropdownMenuItem
                  render={
                    <a
                      href={`/s/${item.subsite.slug}`}
                      target='_blank'
                      rel='noreferrer'
                    />
                  }
                >
                  {t('Open Subsite')}
                  <DropdownMenuShortcut>
                    <ExternalLink size={16} />
                  </DropdownMenuShortcut>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={options.onRefresh}>
                  {t('Refresh')}
                  <DropdownMenuShortcut>
                    <RefreshCw size={16} />
                  </DropdownMenuShortcut>
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )
        },
        enableSorting: false,
        enableHiding: false,
        meta: { label: t('Actions') },
      },
    ],
    [navigate, options, t]
  )
}

function SubsiteManagementTable(props: {
  onActivity: (item: ManagedSubsite) => void
  onChannels: (item: ManagedSubsite) => void
  onEdit: (item: ManagedSubsite) => void
  onMembers: (item: ManagedSubsite) => void
  onQuota: (item: ManagedSubsite) => void
  onCreate: () => void
}) {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const queryClient = useQueryClient()
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})

  const { pagination, onPaginationChange, ensurePageInRange } =
    useTableUrlState({
      search,
      navigate,
      pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 20 },
      globalFilter: { enabled: false },
    })

  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: [
      'managed-subsites',
      pagination.pageIndex + 1,
      pagination.pageSize,
    ],
    queryFn: async () => {
      const result = await getManagedSubsites({
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
      })
      if (!result.success) {
        toast.error(result.message || t('Failed to load subsites'))
        return { items: [], total: 0 }
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: ['managed-subsites'] })
    void refetch()
  }

  const columns = useManagedSubsitesColumns({
    onActivity: props.onActivity,
    onChannels: props.onChannels,
    onEdit: props.onEdit,
    onMembers: props.onMembers,
    onQuota: props.onQuota,
    onRefresh: handleRefresh,
  })
  const items = data?.items ?? []

  const table = useReactTable({
    data: items,
    columns,
    state: {
      sorting,
      columnVisibility,
      pagination,
    },
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    onPaginationChange,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFacetedRowModel: getFacetedRowModel(),
    getFacetedUniqueValues: getFacetedUniqueValues(),
    manualPagination: true,
    pageCount: Math.ceil((data?.total ?? 0) / pagination.pageSize),
  })

  const pageCount = table.getPageCount()
  useEffect(() => {
    ensurePageInRange(pageCount)
  }, [ensurePageInRange, pageCount])

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Subsites Found')}
      emptyDescription={t(
        isAdmin
          ? 'Create a subsite to give an event, team, or customer a scoped API entry.'
          : 'No manageable subsites are assigned to your account.'
      )}
      emptyAction={
        isAdmin ? (
          <Button onClick={props.onCreate}>
            <Plus />
            {t('Create Subsite')}
          </Button>
        ) : undefined
      }
      toolbarProps={{
        customSearch: null,
        preActions: (
          <Button
            variant='outline'
            onClick={handleRefresh}
            disabled={isFetching}
          >
            <RefreshCw className={cn(isFetching && 'animate-spin')} />
            {t('Refresh')}
          </Button>
        ),
      }}
      getRowClassName={(row) =>
        row.original.subsite.runtime_status !== 'enabled'
          ? 'bg-muted/40 hover:bg-muted/60'
          : undefined
      }
      skeletonKeyPrefix='subsites-management-skeleton'
    />
  )
}

function SubsiteMutateDrawer(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: ManagedSubsite
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isUpdate = Boolean(props.currentRow)
  const isAdmin = useIsAdmin()
  const [isLoadingDetail, setIsLoadingDetail] = useState(false)

  const form = useForm<SubsiteFormValues>({
    resolver: zodResolver(getSubsiteFormSchema(t)),
    defaultValues: subsiteToFormValues(),
  })

  useEffect(() => {
    if (!props.open) return
    let cancelled = false

    if (props.currentRow) {
      setIsLoadingDetail(true)
      getManagedSubsite(props.currentRow.subsite.id)
        .then((result) => {
          if (cancelled) return
          if (result.success && result.data) {
            form.reset(subsiteToFormValues(result.data))
          } else {
            toast.error(result.message || t('Failed to load subsite'))
            form.reset(subsiteToFormValues(props.currentRow))
          }
        })
        .catch(() => {
          if (cancelled) return
          toast.error(t('Failed to load subsite'))
          form.reset(subsiteToFormValues(props.currentRow))
        })
        .finally(() => {
          if (!cancelled) setIsLoadingDetail(false)
        })
      return () => {
        cancelled = true
      }
    }

    form.reset(subsiteToFormValues())
    return () => {
      cancelled = true
    }
  }, [form, props.currentRow, props.open, t])

  const mutation = useMutation({
    mutationFn: async (values: SubsiteFormValues) => {
      const payload = formValuesToPayload(values)
      if (isUpdate && props.currentRow) {
        return updateManagedSubsite(props.currentRow.subsite.id, payload)
      }
      return createManagedSubsite(payload)
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to save subsite'))
        return
      }
      toast.success(isUpdate ? t('Subsite updated') : t('Subsite created'))
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['managed-subsites'] })
    },
    onError: () => {
      toast.error(t('Failed to save subsite'))
    },
  })

  const themeColor = form.watch('theme_color')

  return (
    <Sheet
      open={props.open}
      onOpenChange={(open) => {
        props.onOpenChange(open)
        if (!open) form.reset(subsiteToFormValues())
      }}
    >
      <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isUpdate ? t('Edit Subsite') : t('Create Subsite')}
          </SheetTitle>
          <SheetDescription>
            {t('Configure the subsite entry, access window, and public copy.')}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='subsite-form'
            onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Basic Information')}
                description={t(
                  'Name, slug, and the owner assigned on creation.'
                )}
                icon={<Settings2 className='size-4' />}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='name'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Name')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder={t('Launch Week')} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='slug'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Slug')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder='launch-week'
                          onChange={(event) =>
                            field.onChange(event.target.value.toLowerCase())
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        /s/{field.value || 'slug'}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>

              <FormField
                control={form.control}
                name='title'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Console Title')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={t('Developer Console')} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {!isUpdate && isAdmin && (
                <FormField
                  control={form.control}
                  name='owner_user_id'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Owner User ID')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='1'
                          placeholder={t('Leave empty to use yourself')}
                          value={field.value || ''}
                          onChange={(event) =>
                            field.onChange(Number(event.target.value) || 0)
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t('The selected user becomes the subsite owner.')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Branding')}
                description={t(
                  'Logo, favicon, and the accent shown on public pages.'
                )}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='logo_url'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Logo URL')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder='https://...' />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='favicon_url'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Favicon URL')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder='https://...' />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <FormField
                control={form.control}
                name='theme_color'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Theme Color')}</FormLabel>
                    <FormControl>
                      <div className='grid grid-cols-[3rem_1fr] gap-2'>
                        <Input
                          type='color'
                          className='h-8 p-1'
                          value={themeColor || '#2563eb'}
                          onChange={(event) =>
                            field.onChange(event.target.value)
                          }
                        />
                        <Input {...field} placeholder='#2563eb' />
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Access')}
                description={t(
                  'Publish state, close message, and valid period.'
                )}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='status'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Status')}</FormLabel>
                      <Select
                        items={SUBSITE_STATUS_OPTIONS.map((value) => ({
                          value,
                          label: t(runtimeStatusMeta(value).label),
                        }))}
                        value={field.value}
                        onValueChange={(value) =>
                          value && field.onChange(value)
                        }
                      >
                        <FormControl>
                          <SelectTrigger className='w-full'>
                            <SelectValue placeholder={t('Select status')} />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {SUBSITE_STATUS_OPTIONS.map((value) => (
                              <SelectItem key={value} value={value}>
                                {t(runtimeStatusMeta(value).label)}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='registration_policy'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Registration')}</FormLabel>
                      <Select
                        items={REGISTRATION_POLICY_OPTIONS.map((value) => ({
                          value,
                          label: t(registrationPolicyLabel(value)),
                        }))}
                        value={field.value}
                        onValueChange={(value) =>
                          value && field.onChange(value)
                        }
                      >
                        <FormControl>
                          <SelectTrigger className='w-full'>
                            <SelectValue
                              placeholder={t('Select registration policy')}
                            />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {REGISTRATION_POLICY_OPTIONS.map((value) => (
                              <SelectItem key={value} value={value}>
                                {t(registrationPolicyLabel(value))}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <FormField
                control={form.control}
                name='disabled_reason'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Disabled Reason')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        placeholder={t('Shown when the subsite is closed')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='starts_at'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Starts At')}</FormLabel>
                      <FormControl>
                        <Input {...field} type='datetime-local' />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='ends_at'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Ends At')}</FormLabel>
                      <FormControl>
                        <Input {...field} type='datetime-local' />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Registration Guardrails')}
                description={t(
                  'Invite code and domain allowlist for new members.'
                )}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='invite_code'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Invite Code')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder={t('Optional')} />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='email_domain_whitelist'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Email Domains')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder='example.com, team.dev' />
                      </FormControl>
                      <FormDescription>
                        {t('Comma-separated domains. Empty allows any domain.')}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Announcement')}
                description={t('Message shown in the subsite user console.')}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='announcement_icon'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Icon')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder='megaphone' />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='announcement_title'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Title')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder={t('Announcement title')}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <FormField
                control={form.control}
                name='announcement_body'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Body')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        placeholder={t('Announcement body')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='announcement_url'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Announcement URL')}</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder='https://...' />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='contact_url'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Contact URL')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder='mailto:support@example.com'
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button type='button' variant='outline' />}>
            {t('Cancel')}
          </SheetClose>
          <Button
            type='submit'
            form='subsite-form'
            disabled={mutation.isPending || isLoadingDetail}
          >
            {mutation.isPending ? t('Saving...') : t('Save')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

function memberToFormValues(member?: ManagedSubsiteMember): MemberFormValues {
  return {
    user_id: member?.user_id ?? 0,
    role: member?.role ?? 'member',
    status: member?.status ?? 'active',
  }
}

function modelDisplayNamesToLines(values?: Record<string, string>) {
  return Object.entries(values ?? {})
    .filter(([, displayName]) => displayName.trim())
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([modelName, displayName]) => `${modelName}=${displayName}`)
    .join('\n')
}

function parseModelDisplayNames(
  value: string | undefined,
  models: string
): Record<string, string> | undefined {
  const allowedModels = new Set(
    models
      .split(',')
      .map((modelName) => modelName.trim())
      .filter(Boolean)
  )
  const displayNames: Record<string, string> = {}
  for (const line of (value ?? '').split('\n')) {
    const separatorIndex = line.indexOf('=')
    if (separatorIndex <= 0) continue
    const modelName = line.slice(0, separatorIndex).trim()
    const displayName = line.slice(separatorIndex + 1).trim()
    if (!modelName || !displayName || !allowedModels.has(modelName)) continue
    displayNames[modelName] = displayName
  }
  return Object.keys(displayNames).length > 0 ? displayNames : undefined
}

function channelToFormValues(
  channel?: ManagedSubsiteChannel
): ChannelFormValues {
  return {
    name: channel?.name ?? '',
    type: channel?.type ?? 1,
    key: '',
    base_url: channel?.base_url ?? '',
    models: channel?.models ?? '',
    model_display_names: modelDisplayNamesToLines(channel?.model_display_names),
    group: channel?.group ?? 'default',
    status: channel?.status ?? CHANNEL_STATUS.ENABLED,
    priority: channel?.priority ?? 0,
    weight: channel?.weight ?? 0,
    remark: channel?.remark ?? '',
  }
}

function channelFormValuesToPayload(
  values: ChannelFormValues
): ManagedSubsiteChannelPayload {
  return {
    name: values.name.trim(),
    type: values.type,
    key: values.key?.trim() || undefined,
    base_url: values.base_url?.trim() || undefined,
    models: values.models.trim(),
    model_display_names: parseModelDisplayNames(
      values.model_display_names,
      values.models
    ),
    group: values.group?.trim() || 'default',
    status: values.status,
    priority: values.priority,
    weight: values.weight,
    remark: values.remark?.trim() || undefined,
  }
}

function channelTypeLabel(type: number) {
  return CHANNEL_TYPES[type as keyof typeof CHANNEL_TYPES] ?? `#${type}`
}

function channelStatusMeta(status: number): {
  label: string
  variant: 'success' | 'warning' | 'neutral'
} {
  switch (status) {
    case CHANNEL_STATUS.ENABLED:
      return { label: 'Enabled', variant: 'success' }
    case CHANNEL_STATUS.MANUAL_DISABLED:
      return { label: 'Disabled', variant: 'warning' }
    default:
      return { label: 'Unknown', variant: 'neutral' }
  }
}

function ChannelsDrawer(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: ManagedSubsite
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const subsiteId = props.currentRow?.subsite.id ?? 0
  const canManage = Boolean(props.currentRow?.can_manage)
  const [editingChannel, setEditingChannel] = useState<
    ManagedSubsiteChannel | undefined
  >()
  const [channelToDelete, setChannelToDelete] = useState<
    ManagedSubsiteChannel | undefined
  >()

  const form = useForm<ChannelFormValues>({
    resolver: zodResolver(getChannelFormSchema(t)),
    defaultValues: channelToFormValues(),
  })

  const {
    data: channels = [],
    isLoading,
    isFetching,
  } = useQuery({
    queryKey: ['managed-subsite-channels', subsiteId],
    enabled: props.open && subsiteId > 0,
    queryFn: async () => {
      const result = await getManagedSubsiteChannels(subsiteId)
      if (!result.success) {
        toast.error(result.message || t('Failed to load channels'))
        return []
      }
      return result.data ?? []
    },
  })

  useEffect(() => {
    if (!props.open) {
      setEditingChannel(undefined)
      setChannelToDelete(undefined)
      form.reset(channelToFormValues())
    }
  }, [form, props.open])

  const invalidateChannels = () => {
    queryClient.invalidateQueries({
      queryKey: ['managed-subsite-channels', subsiteId],
    })
  }

  const upsertMutation = useMutation({
    mutationFn: async (values: ChannelFormValues) => {
      if (!subsiteId) throw new Error('missing subsite')
      const payload = channelFormValuesToPayload(values)
      if (editingChannel) {
        return updateManagedSubsiteChannel(
          subsiteId,
          editingChannel.id,
          payload
        )
      }
      return createManagedSubsiteChannel(subsiteId, payload)
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to save channel'))
        return
      }
      toast.success(
        editingChannel ? t('Channel updated') : t('Channel created')
      )
      setEditingChannel(undefined)
      form.reset(channelToFormValues())
      invalidateChannels()
    },
    onError: () => {
      toast.error(t('Failed to save channel'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (channel: ManagedSubsiteChannel) => {
      if (!subsiteId) throw new Error('missing subsite')
      return deleteManagedSubsiteChannel(subsiteId, channel.id)
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to remove channel'))
        return
      }
      toast.success(t('Channel removed'))
      setChannelToDelete(undefined)
      invalidateChannels()
    },
    onError: () => {
      toast.error(t('Failed to remove channel'))
    },
  })

  const testChannelMutation = useMutation({
    mutationFn: async (channel: ManagedSubsiteChannel) => {
      if (!subsiteId) throw new Error('missing subsite')
      return testManagedSubsiteChannel(subsiteId, channel.id)
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Channel test failed'))
        return
      }
      toast.success(
        t('Channel test passed in {{time}}s', {
          time: (result.time ?? 0).toFixed(2),
        })
      )
      invalidateChannels()
    },
    onError: () => {
      toast.error(t('Channel test failed'))
    },
  })

  const balanceMutation = useMutation({
    mutationFn: async (channel: ManagedSubsiteChannel) => {
      if (!subsiteId) throw new Error('missing subsite')
      return updateManagedSubsiteChannelBalance(subsiteId, channel.id)
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to query balance'))
        return
      }
      toast.success(
        t('Balance updated: {{balance}}', {
          balance: formatCurrencyFromUSD(result.data?.balance ?? 0),
        })
      )
      invalidateChannels()
    },
    onError: () => {
      toast.error(t('Failed to query balance'))
    },
  })

  const syncModelsMutation = useMutation({
    mutationFn: async (channel: ManagedSubsiteChannel) => {
      if (!subsiteId) throw new Error('missing subsite')
      return syncManagedSubsiteChannelModels(subsiteId, channel.id)
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to sync models'))
        return
      }
      toast.success(t('Models synced'))
      if (result.data && editingChannel?.id === result.data.id) {
        form.reset(channelToFormValues(result.data))
        setEditingChannel(result.data)
      }
      invalidateChannels()
    },
    onError: () => {
      toast.error(t('Failed to sync models'))
    },
  })

  const resetChannelForm = () => {
    setEditingChannel(undefined)
    form.reset(channelToFormValues())
  }

  return (
    <>
      <Sheet
        open={props.open}
        onOpenChange={(open) => {
          props.onOpenChange(open)
          if (!open) resetChannelForm()
        }}
      >
        <SheetContent className={sideDrawerContentClassName('sm:max-w-4xl')}>
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle>{t('Channels')}</SheetTitle>
            <SheetDescription>
              {props.currentRow
                ? `${props.currentRow.subsite.name} / ${props.currentRow.subsite.slug}`
                : t('Manage subsite channels and model exposure.')}
            </SheetDescription>
          </SheetHeader>

          <div className={sideDrawerFormClassName('gap-5')}>
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={editingChannel ? t('Edit Channel') : t('Add Channel')}
                description={t(
                  'Models listed here are exposed only through this subsite.'
                )}
                icon={<Settings2 className='size-4' />}
              />
              <Form {...form}>
                <form
                  id='subsite-channel-form'
                  className='grid gap-4 lg:grid-cols-3'
                  onSubmit={form.handleSubmit((values) =>
                    upsertMutation.mutate(values)
                  )}
                >
                  <FormField
                    control={form.control}
                    name='name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Name')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            disabled={!canManage}
                            placeholder={t('OpenAI event pool')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='type'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Type')}</FormLabel>
                        <Select
                          items={CHANNEL_TYPE_OPTIONS.map((option) => ({
                            value: String(option.value),
                            label: t(option.label),
                          }))}
                          value={String(field.value)}
                          onValueChange={(value) =>
                            value && field.onChange(Number(value))
                          }
                          disabled={!canManage}
                        >
                          <FormControl>
                            <SelectTrigger className='w-full'>
                              <SelectValue placeholder={t('Select type')} />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {CHANNEL_TYPE_OPTIONS.map((option) => (
                                <SelectItem
                                  key={option.value}
                                  value={String(option.value)}
                                >
                                  {t(option.label)}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='status'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Status')}</FormLabel>
                        <Select
                          items={CHANNEL_STATUS_OPTIONS.map((value) => ({
                            value: String(value),
                            label: t(channelStatusMeta(value).label),
                          }))}
                          value={String(field.value)}
                          onValueChange={(value) =>
                            value && field.onChange(Number(value))
                          }
                          disabled={!canManage}
                        >
                          <FormControl>
                            <SelectTrigger className='w-full'>
                              <SelectValue placeholder={t('Select status')} />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {CHANNEL_STATUS_OPTIONS.map((value) => (
                                <SelectItem key={value} value={String(value)}>
                                  {t(channelStatusMeta(value).label)}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='key'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Upstream Key')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            type='password'
                            autoComplete='off'
                            disabled={!canManage}
                            placeholder={
                              editingChannel
                                ? t('Leave blank to keep current key')
                                : 'sk-...'
                            }
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='base_url'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Base URL')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            disabled={!canManage}
                            placeholder='https://api.openai.com'
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='group'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Group')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            disabled={!canManage}
                            placeholder='default'
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='models'
                    render={({ field }) => (
                      <FormItem className='lg:col-span-3'>
                        <FormLabel>{t('Models')}</FormLabel>
                        <FormControl>
                          <Textarea
                            {...field}
                            disabled={!canManage}
                            placeholder='gpt-4o,gpt-4o-mini'
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'Comma-separated model IDs exposed by this subsite.'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='model_display_names'
                    render={({ field }) => (
                      <FormItem className='lg:col-span-3'>
                        <FormLabel>{t('Display Names')}</FormLabel>
                        <FormControl>
                          <Textarea
                            {...field}
                            disabled={!canManage}
                            placeholder='gpt-4o=GPT-4o Premium'
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='priority'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Priority')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            type='number'
                            step='1'
                            disabled={!canManage}
                            onChange={(event) =>
                              field.onChange(Number(event.target.value) || 0)
                            }
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='weight'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Weight')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            type='number'
                            min='0'
                            step='1'
                            disabled={!canManage}
                            onChange={(event) =>
                              field.onChange(Number(event.target.value) || 0)
                            }
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='remark'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Remark')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            disabled={!canManage}
                            placeholder={t('Optional')}
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <div className='flex flex-wrap items-end gap-2 lg:col-span-3'>
                    {editingChannel && (
                      <Button
                        type='button'
                        variant='outline'
                        onClick={resetChannelForm}
                        disabled={upsertMutation.isPending}
                      >
                        {t('Cancel')}
                      </Button>
                    )}
                    <Button
                      type='submit'
                      disabled={
                        !canManage || !subsiteId || upsertMutation.isPending
                      }
                    >
                      <Settings2 />
                      {upsertMutation.isPending
                        ? t('Saving...')
                        : editingChannel
                          ? t('Update')
                          : t('Add')}
                    </Button>
                  </div>
                </form>
              </Form>
            </SideDrawerSection>

            <SideDrawerSection className='gap-3'>
              <SideDrawerSectionHeader
                title={t('Channel List')}
                description={t('{{count}} channels configured.', {
                  count: channels.length,
                })}
              />
              <div className='rounded-lg border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('Channel')}</TableHead>
                      <TableHead>{t('Models')}</TableHead>
                      <TableHead>{t('Routing')}</TableHead>
                      <TableHead>{t('Usage')}</TableHead>
                      <TableHead className='w-[184px] text-right'>
                        {t('Actions')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {isLoading ? (
                      <TableRow>
                        <TableCell
                          colSpan={5}
                          className='text-muted-foreground h-24 text-center'
                        >
                          {t('Loading channels...')}
                        </TableCell>
                      </TableRow>
                    ) : channels.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={5}
                          className='text-muted-foreground h-24 text-center'
                        >
                          {t('No channels configured yet.')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      channels.map((channel) => {
                        const statusMeta = channelStatusMeta(channel.status)
                        const models = channel.models
                          .split(',')
                          .map((modelName) => modelName.trim())
                          .filter(Boolean)
                        const displayNames = channel.model_display_names ?? {}
                        return (
                          <TableRow
                            key={channel.id}
                            className={
                              channel.status !== CHANNEL_STATUS.ENABLED
                                ? 'bg-muted/40'
                                : undefined
                            }
                          >
                            <TableCell>
                              <div className='flex min-w-[180px] flex-col gap-1'>
                                <div className='flex items-center gap-2'>
                                  <LongText className='max-w-[150px] font-medium'>
                                    {channel.name}
                                  </LongText>
                                  <TableId
                                    value={channel.id}
                                    className='w-[58px]'
                                  />
                                </div>
                                <div className='flex flex-wrap gap-1'>
                                  <StatusBadge
                                    label={t(channelTypeLabel(channel.type))}
                                    variant='neutral'
                                    copyable={false}
                                  />
                                  <StatusBadge
                                    label={t(statusMeta.label)}
                                    variant={statusMeta.variant}
                                    copyable={false}
                                  />
                                  {channel.has_key && (
                                    <StatusBadge
                                      label={t('Key set')}
                                      variant='info'
                                      copyable={false}
                                    />
                                  )}
                                </div>
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className='flex max-w-[260px] flex-wrap gap-1'>
                                {models.slice(0, 3).map((modelName) => (
                                  <StatusBadge
                                    key={modelName}
                                    label={displayNames[modelName] || modelName}
                                    copyText={modelName}
                                    variant='neutral'
                                    copyable
                                  />
                                ))}
                                {models.length > 3 && (
                                  <StatusBadge
                                    label={`+${models.length - 3}`}
                                    variant='neutral'
                                    copyable={false}
                                  />
                                )}
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className='space-y-1 text-xs'>
                                <div>
                                  <span className='text-muted-foreground'>
                                    {t('Group')}:{' '}
                                  </span>
                                  <span className='font-medium'>
                                    {channel.group || 'default'}
                                  </span>
                                </div>
                                <div className='text-muted-foreground tabular-nums'>
                                  P {channel.priority} / W {channel.weight}
                                </div>
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className='space-y-1 text-xs'>
                                <div className='font-medium tabular-nums'>
                                  {formatQuota(channel.used_quota)}
                                </div>
                                <div className='text-muted-foreground tabular-nums'>
                                  {channel.response_time > 0
                                    ? `${channel.response_time}ms`
                                    : '-'}
                                </div>
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className='flex justify-end gap-1'>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='icon-sm'
                                  disabled={
                                    !canManage || testChannelMutation.isPending
                                  }
                                  title={t('Test channel')}
                                  onClick={() =>
                                    testChannelMutation.mutate(channel)
                                  }
                                >
                                  <PlayCircle className='size-4' />
                                  <span className='sr-only'>
                                    {t('Test channel')}
                                  </span>
                                </Button>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='icon-sm'
                                  disabled={
                                    !canManage || balanceMutation.isPending
                                  }
                                  title={t('Query balance')}
                                  onClick={() =>
                                    balanceMutation.mutate(channel)
                                  }
                                >
                                  <CircleDollarSign className='size-4' />
                                  <span className='sr-only'>
                                    {t('Query balance')}
                                  </span>
                                </Button>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='icon-sm'
                                  disabled={
                                    !canManage || syncModelsMutation.isPending
                                  }
                                  title={t('Sync models')}
                                  onClick={() =>
                                    syncModelsMutation.mutate(channel)
                                  }
                                >
                                  <ListChecks className='size-4' />
                                  <span className='sr-only'>
                                    {t('Sync models')}
                                  </span>
                                </Button>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='icon-sm'
                                  disabled={!canManage}
                                  title={t('Edit channel')}
                                  onClick={() => {
                                    setEditingChannel(channel)
                                    form.reset(channelToFormValues(channel))
                                  }}
                                >
                                  <Pencil className='size-4' />
                                  <span className='sr-only'>
                                    {t('Edit channel')}
                                  </span>
                                </Button>
                                <Button
                                  type='button'
                                  variant='destructive'
                                  size='icon-sm'
                                  disabled={!canManage}
                                  title={t('Remove channel')}
                                  onClick={() => setChannelToDelete(channel)}
                                >
                                  <Trash2 className='size-4' />
                                  <span className='sr-only'>
                                    {t('Remove channel')}
                                  </span>
                                </Button>
                              </div>
                            </TableCell>
                          </TableRow>
                        )
                      })
                    )}
                  </TableBody>
                </Table>
              </div>
            </SideDrawerSection>
          </div>

          <SheetFooter className={sideDrawerFooterClassName()}>
            <Button
              type='button'
              variant='outline'
              onClick={() => invalidateChannels()}
              disabled={!subsiteId || isFetching}
            >
              <RefreshCw className={cn(isFetching && 'animate-spin')} />
              {t('Refresh')}
            </Button>
            <SheetClose render={<Button type='button' variant='outline' />}>
              {t('Close')}
            </SheetClose>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <ConfirmDialog
        open={Boolean(channelToDelete)}
        onOpenChange={(open) => {
          if (!open) setChannelToDelete(undefined)
        }}
        title={t('Remove channel')}
        desc={
          channelToDelete
            ? t('Remove {{name}} and its model routing from this subsite?', {
                name: channelToDelete.name,
              })
            : ''
        }
        confirmText={t('Remove')}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (channelToDelete) deleteMutation.mutate(channelToDelete)
        }}
      />
    </>
  )
}

function MembersDrawer(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: ManagedSubsite
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const subsiteId = props.currentRow?.subsite.id ?? 0
  const canManage = Boolean(props.currentRow?.can_manage)
  const [editingMember, setEditingMember] = useState<
    ManagedSubsiteMember | undefined
  >()
  const [memberToDelete, setMemberToDelete] = useState<
    ManagedSubsiteMember | undefined
  >()

  const form = useForm<MemberFormValues>({
    resolver: zodResolver(getMemberFormSchema(t)),
    defaultValues: memberToFormValues(),
  })

  const {
    data: members = [],
    isLoading,
    isFetching,
  } = useQuery({
    queryKey: ['managed-subsite-members', subsiteId],
    enabled: props.open && subsiteId > 0,
    queryFn: async () => {
      const result = await getManagedSubsiteMembers(subsiteId)
      if (!result.success) {
        toast.error(result.message || t('Failed to load members'))
        return []
      }
      return result.data ?? []
    },
  })

  useEffect(() => {
    if (!props.open) {
      setEditingMember(undefined)
      setMemberToDelete(undefined)
      form.reset(memberToFormValues())
    }
  }, [form, props.open])

  const activeOwnerCount = members.filter(
    (member) => member.role === 'owner' && member.status === 'active'
  ).length
  const nextRole = form.watch('role')
  const nextStatus = form.watch('status')
  const editingLastActiveOwner =
    editingMember?.role === 'owner' &&
    editingMember.status === 'active' &&
    activeOwnerCount <= 1
  const wouldLoseLastActiveOwner =
    editingLastActiveOwner && (nextRole !== 'owner' || nextStatus !== 'active')

  const invalidateMembers = () => {
    queryClient.invalidateQueries({
      queryKey: ['managed-subsite-members', subsiteId],
    })
    queryClient.invalidateQueries({ queryKey: ['managed-subsites'] })
  }

  const upsertMutation = useMutation({
    mutationFn: async (values: MemberFormValues) => {
      if (!subsiteId) throw new Error('missing subsite')
      const payload: ManagedSubsiteMemberUpsertPayload = {
        user_id: values.user_id,
        role: values.role,
        status: values.status,
      }
      return upsertManagedSubsiteMember(subsiteId, payload)
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to save member'))
        return
      }
      toast.success(editingMember ? t('Member updated') : t('Member added'))
      setEditingMember(undefined)
      form.reset(memberToFormValues())
      invalidateMembers()
    },
    onError: () => {
      toast.error(t('Failed to save member'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (member: ManagedSubsiteMember) => {
      if (!subsiteId) throw new Error('missing subsite')
      return deleteManagedSubsiteMember(subsiteId, member.user_id)
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to remove member'))
        return
      }
      toast.success(t('Member removed'))
      setMemberToDelete(undefined)
      invalidateMembers()
    },
    onError: () => {
      toast.error(t('Failed to remove member'))
    },
  })

  const resetMemberForm = () => {
    setEditingMember(undefined)
    form.reset(memberToFormValues())
  }

  return (
    <>
      <Sheet
        open={props.open}
        onOpenChange={(open) => {
          props.onOpenChange(open)
          if (!open) resetMemberForm()
        }}
      >
        <SheetContent className={sideDrawerContentClassName('sm:max-w-3xl')}>
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle>{t('Members')}</SheetTitle>
            <SheetDescription>
              {props.currentRow
                ? `${props.currentRow.subsite.name} / ${props.currentRow.subsite.slug}`
                : t('Manage subsite members and access roles.')}
            </SheetDescription>
          </SheetHeader>

          <div className={sideDrawerFormClassName('gap-5')}>
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={editingMember ? t('Edit Member') : t('Add Member')}
                description={t(
                  'Owners and admins can manage the subsite; disabled members cannot access it.'
                )}
                icon={<UserPlus className='size-4' />}
              />

              <Form {...form}>
                <form
                  id='subsite-member-form'
                  className='grid gap-4 lg:grid-cols-[1fr_10rem_10rem_auto]'
                  onSubmit={form.handleSubmit((values) =>
                    upsertMutation.mutate(values)
                  )}
                >
                  <FormField
                    control={form.control}
                    name='user_id'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('User ID')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            type='number'
                            min='1'
                            disabled={Boolean(editingMember) || !canManage}
                            placeholder='1001'
                            value={field.value || ''}
                            onChange={(event) =>
                              field.onChange(Number(event.target.value) || 0)
                            }
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='role'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Role')}</FormLabel>
                        <Select
                          items={MEMBER_ROLE_OPTIONS.map((value) => ({
                            value,
                            label: t(memberRoleLabel(value)),
                          }))}
                          value={field.value}
                          onValueChange={(value) =>
                            value && field.onChange(value as SubsiteMemberRole)
                          }
                          disabled={!canManage}
                        >
                          <FormControl>
                            <SelectTrigger className='w-full'>
                              <SelectValue placeholder={t('Select role')} />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {MEMBER_ROLE_OPTIONS.map((value) => (
                                <SelectItem key={value} value={value}>
                                  {t(memberRoleLabel(value))}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='status'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Status')}</FormLabel>
                        <Select
                          items={MEMBER_STATUS_OPTIONS.map((value) => ({
                            value,
                            label: t(memberStatusMeta(value).label),
                          }))}
                          value={field.value}
                          onValueChange={(value) =>
                            value &&
                            field.onChange(value as SubsiteMemberStatus)
                          }
                          disabled={!canManage}
                        >
                          <FormControl>
                            <SelectTrigger className='w-full'>
                              <SelectValue placeholder={t('Select status')} />
                            </SelectTrigger>
                          </FormControl>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {MEMBER_STATUS_OPTIONS.map((value) => (
                                <SelectItem key={value} value={value}>
                                  {t(memberStatusMeta(value).label)}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <div className='flex items-end gap-2'>
                    {editingMember && (
                      <Button
                        type='button'
                        variant='outline'
                        onClick={resetMemberForm}
                        disabled={upsertMutation.isPending}
                      >
                        {t('Cancel')}
                      </Button>
                    )}
                    <Button
                      type='submit'
                      disabled={
                        !canManage ||
                        !subsiteId ||
                        wouldLoseLastActiveOwner ||
                        upsertMutation.isPending
                      }
                    >
                      <UserPlus />
                      {upsertMutation.isPending
                        ? t('Saving...')
                        : editingMember
                          ? t('Update')
                          : t('Add')}
                    </Button>
                  </div>
                </form>
              </Form>

              {wouldLoseLastActiveOwner && (
                <p className='text-warning text-xs leading-5'>
                  {t('At least one active owner must remain on the subsite.')}
                </p>
              )}
            </SideDrawerSection>

            <SideDrawerSection className='gap-3'>
              <SideDrawerSectionHeader
                title={t('Member List')}
                description={t('{{count}} members configured.', {
                  count: members.length,
                })}
                icon={<Users className='size-4' />}
              />

              <div className='rounded-lg border'>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('User')}</TableHead>
                      <TableHead>{t('Role')}</TableHead>
                      <TableHead>{t('Access')}</TableHead>
                      <TableHead>{t('Updated')}</TableHead>
                      <TableHead className='w-[88px] text-right'>
                        {t('Actions')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {isLoading ? (
                      <TableRow>
                        <TableCell
                          colSpan={5}
                          className='text-muted-foreground h-24 text-center'
                        >
                          {t('Loading members...')}
                        </TableCell>
                      </TableRow>
                    ) : members.length === 0 ? (
                      <TableRow>
                        <TableCell
                          colSpan={5}
                          className='text-muted-foreground h-24 text-center'
                        >
                          {t('No members assigned yet.')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      members.map((member) => {
                        const displayName =
                          member.display_name ||
                          member.username ||
                          `#${member.user_id}`
                        const statusMeta = memberStatusMeta(member.status)
                        const removeDisabled =
                          !canManage ||
                          (member.role === 'owner' &&
                            member.status === 'active' &&
                            activeOwnerCount <= 1)
                        return (
                          <TableRow
                            key={member.id || member.user_id}
                            className={
                              !member.can_access ? 'bg-muted/40' : undefined
                            }
                          >
                            <TableCell>
                              <div className='flex min-w-[190px] flex-col gap-1'>
                                <div className='flex items-center gap-2'>
                                  <LongText className='max-w-[150px] font-medium'>
                                    {displayName}
                                  </LongText>
                                  <TableId
                                    value={member.user_id}
                                    className='w-[60px]'
                                  />
                                </div>
                                <div className='text-muted-foreground flex max-w-[240px] gap-1 text-xs'>
                                  <span className='truncate'>
                                    {member.username || '-'}
                                  </span>
                                  {member.email && (
                                    <span className='truncate'>
                                      / {member.email}
                                    </span>
                                  )}
                                </div>
                              </div>
                            </TableCell>
                            <TableCell>
                              <StatusBadge
                                label={t(memberRoleLabel(member.role))}
                                variant={memberRoleVariant(member.role)}
                                copyable={false}
                              />
                            </TableCell>
                            <TableCell>
                              <div className='flex flex-col gap-1'>
                                <StatusBadge
                                  label={t(statusMeta.label)}
                                  variant={statusMeta.variant}
                                  copyable={false}
                                />
                                {!member.can_access && (
                                  <span className='text-muted-foreground text-xs'>
                                    {t('Access blocked')}
                                  </span>
                                )}
                              </div>
                            </TableCell>
                            <TableCell className='text-muted-foreground text-xs'>
                              {member.updated_at
                                ? formatTimestampToDate(member.updated_at)
                                : '-'}
                            </TableCell>
                            <TableCell>
                              <div className='flex justify-end gap-1'>
                                <Button
                                  type='button'
                                  variant='ghost'
                                  size='icon-sm'
                                  disabled={!canManage}
                                  title={t('Edit member')}
                                  onClick={() => {
                                    setEditingMember(member)
                                    form.reset(memberToFormValues(member))
                                  }}
                                >
                                  <Pencil className='size-4' />
                                  <span className='sr-only'>
                                    {t('Edit member')}
                                  </span>
                                </Button>
                                <Button
                                  type='button'
                                  variant='destructive'
                                  size='icon-sm'
                                  disabled={removeDisabled}
                                  title={
                                    removeDisabled
                                      ? t(
                                          'At least one active owner must remain.'
                                        )
                                      : t('Remove member')
                                  }
                                  onClick={() => setMemberToDelete(member)}
                                >
                                  <Trash2 className='size-4' />
                                  <span className='sr-only'>
                                    {t('Remove member')}
                                  </span>
                                </Button>
                              </div>
                            </TableCell>
                          </TableRow>
                        )
                      })
                    )}
                  </TableBody>
                </Table>
              </div>
            </SideDrawerSection>
          </div>

          <SheetFooter className={sideDrawerFooterClassName()}>
            <Button
              type='button'
              variant='outline'
              onClick={() => invalidateMembers()}
              disabled={!subsiteId || isFetching}
            >
              <RefreshCw className={cn(isFetching && 'animate-spin')} />
              {t('Refresh')}
            </Button>
            <SheetClose render={<Button type='button' variant='outline' />}>
              {t('Close')}
            </SheetClose>
          </SheetFooter>
        </SheetContent>
      </Sheet>

      <ConfirmDialog
        open={Boolean(memberToDelete)}
        onOpenChange={(open) => {
          if (!open) setMemberToDelete(undefined)
        }}
        title={t('Remove member')}
        desc={
          memberToDelete
            ? t('Remove {{name}} from this subsite?', {
                name:
                  memberToDelete.display_name ||
                  memberToDelete.username ||
                  `#${memberToDelete.user_id}`,
              })
            : ''
        }
        confirmText={t('Remove')}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (memberToDelete) deleteMutation.mutate(memberToDelete)
        }}
      />
    </>
  )
}

function ActivityDrawer(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: ManagedSubsite
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const subsiteId = props.currentRow?.subsite.id ?? 0

  const {
    data: activity,
    isLoading,
    isFetching,
    refetch,
  } = useQuery({
    queryKey: ['managed-subsite-activity', subsiteId],
    enabled: props.open && subsiteId > 0,
    queryFn: async () => {
      const result = await getManagedSubsiteActivity(subsiteId)
      if (!result.success) {
        toast.error(result.message || t('Failed to load activity'))
        return undefined
      }
      return result.data
    },
  })

  const stats = activity?.stats_24h
  const recentLogs = activity?.recent_logs ?? []
  const openUsageLogs = (type?: '5') => {
    void navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        subsiteId: String(subsiteId),
        page: 1,
        ...(type ? { type: [type] } : {}),
      },
    })
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-4xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Activity')}</SheetTitle>
          <SheetDescription>
            {props.currentRow
              ? `${props.currentRow.subsite.name} / ${props.currentRow.subsite.slug}`
              : t('Review subsite usage, errors, and recent requests.')}
          </SheetDescription>
        </SheetHeader>

        <div className={sideDrawerFormClassName('gap-5')}>
          <SideDrawerSection>
            <SideDrawerSectionHeader
              title={t('24h Summary')}
              description={t(
                'All consume and error requests for this subsite.'
              )}
              icon={<ActivityIcon className='size-4' />}
            />
            {isLoading ? (
              <div className='text-muted-foreground rounded-lg border p-4 text-sm'>
                {t('Loading activity...')}
              </div>
            ) : (
              <>
                <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-5'>
                  <ActivityMetricTile
                    label={t('Calls')}
                    value={formatNumber(stats?.calls ?? 0)}
                    description={t('Total requests')}
                  />
                  <ActivityMetricTile
                    label={t('Errors')}
                    value={formatNumber(activity?.error_calls_24h ?? 0)}
                    description={t('Failed requests')}
                    variant={
                      activity?.error_calls_24h && activity.error_calls_24h > 0
                        ? 'danger'
                        : 'neutral'
                    }
                  />
                  <ActivityMetricTile
                    label={t('Tokens')}
                    value={formatNumber(stats?.total_tokens ?? 0)}
                    description={t('Input and output')}
                  />
                  <ActivityMetricTile
                    label={t('Quota')}
                    value={formatQuota(stats?.quota ?? 0)}
                    description={t('Consumed')}
                  />
                  <ActivityMetricTile
                    label={t('Last Request')}
                    value={
                      stats?.last_request_at
                        ? formatTimestampToDate(stats.last_request_at)
                        : '-'
                    }
                    description={t('Most recent log')}
                  />
                </div>
                <div className='flex flex-wrap gap-2'>
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => openUsageLogs()}
                    disabled={!subsiteId}
                  >
                    <FileText />
                    {t('View All Logs')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    onClick={() => openUsageLogs('5')}
                    disabled={!subsiteId}
                  >
                    <ActivityIcon />
                    {t('View Error Logs')}
                  </Button>
                </div>
              </>
            )}
          </SideDrawerSection>

          <SideDrawerSection className='gap-3'>
            <SideDrawerSectionHeader
              title={t('Recent Requests')}
              description={t(
                'Latest consume and error logs across all members.'
              )}
            />
            <ActivityRecentLogsTable logs={recentLogs} />
          </SideDrawerSection>
        </div>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <Button
            type='button'
            variant='outline'
            onClick={() => void refetch()}
            disabled={!subsiteId || isFetching}
          >
            <RefreshCw className={cn(isFetching && 'animate-spin')} />
            {t('Refresh')}
          </Button>
          <SheetClose render={<Button type='button' variant='outline' />}>
            {t('Close')}
          </SheetClose>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

function QuotaPolicyDrawer(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: ManagedSubsite
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { meta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = meta.kind === 'tokens'

  const form = useForm<QuotaPolicyFormValues>({
    resolver: zodResolver(getQuotaPolicyFormSchema(t)),
    defaultValues: quotaPolicyToFormValues(),
  })

  useEffect(() => {
    if (!props.open) return
    form.reset(quotaPolicyToFormValues(props.currentRow?.quota_policy))
  }, [form, props.currentRow, props.open])

  const mutation = useMutation({
    mutationFn: async (values: QuotaPolicyFormValues) => {
      if (!props.currentRow) throw new Error('missing subsite')
      return updateManagedSubsiteQuotaPolicy(
        props.currentRow.subsite.id,
        quotaFormValuesToPayload(values)
      )
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(result.message || t('Failed to save quota policy'))
        return
      }
      toast.success(t('Quota policy updated'))
      props.onOpenChange(false)
      queryClient.invalidateQueries({ queryKey: ['managed-subsites'] })
    },
    onError: () => {
      toast.error(t('Failed to save quota policy'))
    },
  })

  const quotaFields: Array<{
    name: keyof Pick<
      QuotaPolicyFormValues,
      | 'site_daily_quota'
      | 'site_window_quota'
      | 'user_daily_quota'
      | 'user_window_quota'
    >
    label: string
    description: string
  }> = [
    {
      name: 'site_daily_quota',
      label: 'Site Daily Quota',
      description: 'Maximum total quota consumed by this subsite per day.',
    },
    {
      name: 'site_window_quota',
      label: 'Site Window Quota',
      description:
        'Maximum total quota consumed by this subsite in the rolling window.',
    },
    {
      name: 'user_daily_quota',
      label: 'User Daily Quota',
      description: 'Maximum quota a single subsite user can consume per day.',
    },
    {
      name: 'user_window_quota',
      label: 'User Window Quota',
      description:
        'Maximum quota a single user can consume in the rolling window.',
    },
  ]

  const requestFields: Array<{
    name: keyof Pick<
      QuotaPolicyFormValues,
      | 'site_daily_request_limit'
      | 'site_window_request_limit'
      | 'user_daily_request_limit'
      | 'user_window_request_limit'
    >
    label: string
  }> = [
    { name: 'site_daily_request_limit', label: 'Site Daily Requests' },
    { name: 'site_window_request_limit', label: 'Site Window Requests' },
    { name: 'user_daily_request_limit', label: 'User Daily Requests' },
    { name: 'user_window_request_limit', label: 'User Window Requests' },
  ]

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Quota Policy')}</SheetTitle>
          <SheetDescription>
            {props.currentRow
              ? `${props.currentRow.subsite.name} / ${props.currentRow.subsite.slug}`
              : t('Set site and user quota windows.')}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='subsite-quota-form'
            onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Quota Limits')}
                description={
                  tokensOnly
                    ? t('Enter token quota amounts. Use 0 for no limit.')
                    : t(
                        'Enter quota amounts in {{currency}}. Use 0 for no limit.',
                        {
                          currency: currencyLabel,
                        }
                      )
                }
                icon={<Gauge className='size-4' />}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                {quotaFields.map((fieldDef) => (
                  <FormField
                    key={fieldDef.name}
                    control={form.control}
                    name={fieldDef.name}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t(fieldDef.label)}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            type='number'
                            min='0'
                            step={tokensOnly ? 1 : 0.01}
                            onChange={(event) =>
                              field.onChange(Number(event.target.value) || 0)
                            }
                          />
                        </FormControl>
                        <FormDescription>
                          {t(fieldDef.description)}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                ))}
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Request Limits')}
                description={t('Use 0 to allow unlimited requests.')}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                {requestFields.map((fieldDef) => (
                  <FormField
                    key={fieldDef.name}
                    control={form.control}
                    name={fieldDef.name}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t(fieldDef.label)}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            type='number'
                            min='0'
                            step='1'
                            onChange={(event) =>
                              field.onChange(Number(event.target.value) || 0)
                            }
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                ))}
              </div>
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Rolling Windows')}
                description={t(
                  'Window length in seconds. Use 0 to disable rolling windows.'
                )}
              />
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='site_window_seconds'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Site Window Seconds')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='0'
                          step='1'
                          onChange={(event) =>
                            field.onChange(Number(event.target.value) || 0)
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='user_window_seconds'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('User Window Seconds')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='0'
                          step='1'
                          onChange={(event) =>
                            field.onChange(Number(event.target.value) || 0)
                          }
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            </SideDrawerSection>
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button type='button' variant='outline' />}>
            {t('Cancel')}
          </SheetClose>
          <Button
            type='submit'
            form='subsite-quota-form'
            disabled={mutation.isPending || !props.currentRow}
          >
            {mutation.isPending ? t('Saving...') : t('Save')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

export function SubsitesManagement() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const [activityOpen, setActivityOpen] = useState(false)
  const [channelsOpen, setChannelsOpen] = useState(false)
  const [formOpen, setFormOpen] = useState(false)
  const [membersOpen, setMembersOpen] = useState(false)
  const [quotaOpen, setQuotaOpen] = useState(false)
  const [currentRow, setCurrentRow] = useState<ManagedSubsite | undefined>()

  const handleCreate = () => {
    setCurrentRow(undefined)
    setFormOpen(true)
  }

  const handleActivity = (item: ManagedSubsite) => {
    setCurrentRow(item)
    setActivityOpen(true)
  }

  const handleChannels = (item: ManagedSubsite) => {
    setCurrentRow(item)
    setChannelsOpen(true)
  }

  const handleEdit = (item: ManagedSubsite) => {
    setCurrentRow(item)
    setFormOpen(true)
  }

  const handleMembers = (item: ManagedSubsite) => {
    setCurrentRow(item)
    setMembersOpen(true)
  }

  const handleQuota = (item: ManagedSubsite) => {
    setCurrentRow(item)
    setQuotaOpen(true)
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Subsites')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {isAdmin && (
            <Button onClick={handleCreate}>
              <Plus />
              {t('Create Subsite')}
            </Button>
          )}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <SubsiteManagementTable
            onActivity={handleActivity}
            onChannels={handleChannels}
            onCreate={handleCreate}
            onEdit={handleEdit}
            onMembers={handleMembers}
            onQuota={handleQuota}
          />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <SubsiteMutateDrawer
        open={formOpen}
        onOpenChange={setFormOpen}
        currentRow={currentRow}
      />
      <ActivityDrawer
        open={activityOpen}
        onOpenChange={setActivityOpen}
        currentRow={currentRow}
      />
      <ChannelsDrawer
        open={channelsOpen}
        onOpenChange={setChannelsOpen}
        currentRow={currentRow}
      />
      <MembersDrawer
        open={membersOpen}
        onOpenChange={setMembersOpen}
        currentRow={currentRow}
      />
      <QuotaPolicyDrawer
        open={quotaOpen}
        onOpenChange={setQuotaOpen}
        currentRow={currentRow}
      />
    </>
  )
}
