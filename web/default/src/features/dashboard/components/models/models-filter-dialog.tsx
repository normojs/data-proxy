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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Filter, RotateCcw, Calendar, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getUserModels } from '@/lib/api'
import { getRollingDateRange, type TimeGranularity } from '@/lib/time'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { DateTimePicker } from '@/components/datetime-picker'
import { FilterComboboxInput } from '@/components/filter-combobox-input'
import { getChannels } from '@/features/channels/api'
import { parseModelsList } from '@/features/channels/lib/channel-utils'
import {
  TIME_GRANULARITY_OPTIONS,
  TIME_RANGE_PRESETS,
} from '@/features/dashboard/constants'
import {
  buildDefaultDashboardFilters,
  cleanFilters,
} from '@/features/dashboard/lib'
import type {
  DashboardChartPreferences,
  DashboardFilters,
} from '@/features/dashboard/types'
import { getModels } from '@/features/models/api'

interface ModelsFilterProps {
  preferences: DashboardChartPreferences
  allowUsernameFilter?: boolean
  allowSiteFilters?: boolean
  onFilterChange: (filters: DashboardFilters) => void
  onReset: () => void
}

/**
 * Section divider component for better visual organization
 */
const SectionDivider = ({ label }: { label: string }) => (
  <div className='relative'>
    <div className='absolute inset-0 flex items-center'>
      <span className='w-full border-t' />
    </div>
    <div className='relative flex justify-center text-xs uppercase'>
      <span className='bg-background text-muted-foreground px-2'>{label}</span>
    </div>
  </div>
)

export function ModelsFilter(props: ModelsFilterProps) {
  const { t } = useTranslation()

  const [open, setOpen] = useState(false)
  const [filters, setFilters] = useState<DashboardFilters>(() =>
    buildDefaultDashboardFilters(props.preferences)
  )
  const [selectedRange, setSelectedRange] = useState<number | null>(
    () => props.preferences.defaultTimeRangeDays
  )
  const channelsQuery = useQuery({
    queryKey: ['dashboard-filter-channels', props.allowSiteFilters],
    queryFn: () => getChannels({ page_size: 1000 }),
    enabled: Boolean(props.allowSiteFilters),
    staleTime: 60 * 1000,
  })
  const modelsQuery = useQuery({
    queryKey: ['dashboard-filter-models'],
    queryFn: () => getModels({ page_size: 1000 }),
    enabled: Boolean(props.allowSiteFilters),
    staleTime: 60 * 1000,
  })
  const userModelsQuery = useQuery({
    queryKey: ['dashboard-filter-user-models'],
    queryFn: () => getUserModels(),
    enabled: !props.allowSiteFilters,
    staleTime: 60 * 1000,
  })

  const channelOptions = useMemo(
    () =>
      (channelsQuery.data?.data?.items ?? []).map((channel) => ({
        value: String(channel.id),
        label: channel.name
          ? `${channel.name} (#${channel.id})`
          : `#${channel.id}`,
        description: channel.group || undefined,
      })),
    [channelsQuery.data]
  )

  const modelOptions = useMemo(() => {
    const options = new Map<string, { value: string; label: string }>()
    for (const model of modelsQuery.data?.data?.items ?? []) {
      options.set(model.model_name, {
        value: model.model_name,
        label: model.model_name,
      })
    }
    for (const modelName of userModelsQuery.data?.data ?? []) {
      options.set(modelName, {
        value: modelName,
        label: modelName,
      })
    }
    for (const channel of channelsQuery.data?.data?.items ?? []) {
      for (const modelName of parseModelsList(channel.models)) {
        if (!options.has(modelName)) {
          options.set(modelName, { value: modelName, label: modelName })
        }
      }
    }
    return Array.from(options.values()).sort((a, b) =>
      a.label.localeCompare(b.label)
    )
  }, [channelsQuery.data, modelsQuery.data, userModelsQuery.data])

  const resetFiltersFromPreferences = () => {
    setFilters(buildDefaultDashboardFilters(props.preferences))
    setSelectedRange(props.preferences.defaultTimeRangeDays)
  }

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) resetFiltersFromPreferences()
    setOpen(nextOpen)
  }

  const handleApply = () => {
    props.onFilterChange(
      cleanFilters(
        filters as unknown as Record<string, unknown>
      ) as typeof filters
    )
    setOpen(false)
  }

  const handleReset = () => {
    const days = props.preferences.defaultTimeRangeDays
    const { start, end } = getRollingDateRange(days)
    setFilters({
      ...buildDefaultDashboardFilters(props.preferences),
      start_timestamp: start,
      end_timestamp: end,
    })
    setSelectedRange(days)
    props.onReset()
    setOpen(false)
  }

  const handleChange = (
    field: keyof DashboardFilters,
    value: Date | string | undefined
  ) => {
    setFilters((prev) => ({ ...prev, [field]: value }))
    if (field === 'start_timestamp' || field === 'end_timestamp')
      setSelectedRange(null)
  }

  const handleQuickRange = (days: number) => {
    const { start, end } = getRollingDateRange(days)

    setFilters((prev) => ({
      ...prev,
      start_timestamp: start,
      end_timestamp: end,
    }))
    setSelectedRange(days)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger render={<Button variant='outline' size='sm' />}>
        <Filter className='mr-2 h-4 w-4' />
        {t('Filter')}
      </DialogTrigger>
      <DialogContent className='flex max-h-[calc(100dvh-2rem)] flex-col max-sm:h-dvh max-sm:w-screen max-sm:max-w-none max-sm:rounded-none max-sm:p-4 sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{t('Filter Dashboard Models')}</DialogTitle>
          <DialogDescription>
            {t(
              'Set filters to customize your dashboard statistics and charts.'
            )}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='flex-1 pr-3 sm:pr-4'>
          <div className='grid gap-3 py-3 sm:gap-4 sm:py-4'>
            {/* Quick time range selection */}
            <div className='grid gap-2'>
              <Label className='flex items-center gap-2'>
                <Calendar className='h-4 w-4' />
                {t('Quick Range')}
              </Label>
              <div className='grid grid-cols-2 gap-2 sm:flex'>
                {TIME_RANGE_PRESETS.map((range) => (
                  <Button
                    key={range.days}
                    type='button'
                    size='sm'
                    variant={
                      selectedRange === range.days ? 'default' : 'outline'
                    }
                    onClick={() => handleQuickRange(range.days)}
                    className={cn(
                      'flex-1',
                      selectedRange === range.days &&
                        'ring-ring ring-2 ring-offset-2'
                    )}
                  >
                    {t(range.label)}
                  </Button>
                ))}
              </div>
            </div>

            <SectionDivider label={t('Custom Time Range')} />

            {/* Custom time range */}
            <div className='grid gap-3 sm:gap-4'>
              <div className='grid gap-2'>
                <Label htmlFor='start_timestamp'>{t('Start Time')}</Label>
                <DateTimePicker
                  value={filters.start_timestamp}
                  onChange={(date) =>
                    handleChange('start_timestamp', date || undefined)
                  }
                  placeholder={t('Select start time')}
                />
              </div>

              <div className='grid gap-2'>
                <Label htmlFor='end_timestamp'>{t('End Time')}</Label>
                <DateTimePicker
                  value={filters.end_timestamp}
                  onChange={(date) =>
                    handleChange('end_timestamp', date || undefined)
                  }
                  placeholder={t('Select end time')}
                />
              </div>
            </div>

            <SectionDivider label={t('Data Filters')} />

            <div className='grid gap-3 sm:gap-4'>
              {props.allowSiteFilters && (
                <div className='grid gap-2'>
                  <Label htmlFor='channel_id'>{t('Channel')}</Label>
                  <FilterComboboxInput
                    value={filters.channel_id || ''}
                    onValueChange={(value) => handleChange('channel_id', value)}
                    options={channelOptions}
                    placeholder={t('Filter by channel')}
                    searchPlaceholder={t('Search channels...')}
                    emptyText={t('No channel found.')}
                  />
                </div>
              )}

              <div className='grid gap-2'>
                <Label htmlFor='model_name'>{t('Model Name')}</Label>
                <FilterComboboxInput
                  value={filters.model_name || ''}
                  onValueChange={(value) => handleChange('model_name', value)}
                  options={modelOptions}
                  placeholder={t('Filter by model')}
                  searchPlaceholder={t('Search models...')}
                  emptyText={t('No model found.')}
                />
              </div>
            </div>

            <SectionDivider label={t('Chart Settings')} />

            <div className='grid gap-2'>
              <Label htmlFor='time_granularity'>{t('Time Granularity')}</Label>
              <Select
                items={[
                  ...TIME_GRANULARITY_OPTIONS.map((option) => ({
                    value: option.value,
                    label: t(option.label),
                  })),
                ]}
                value={filters.time_granularity}
                onValueChange={(value) =>
                  handleChange('time_granularity', value as TimeGranularity)
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('Select time granularity')} />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {TIME_GRANULARITY_OPTIONS.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {t(option.label)}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>

            {/* Site-wide fields */}
            {props.allowUsernameFilter && (
              <>
                <SectionDivider label={t('Site-wide Filters')} />

                <div className='grid gap-2'>
                  <Label htmlFor='username'>{t('Username')}</Label>
                  <Input
                    id='username'
                    placeholder={t('Filter by username')}
                    value={filters.username}
                    onChange={(e) => handleChange('username', e.target.value)}
                  />
                </div>
              </>
            )}
          </div>
        </ScrollArea>

        <DialogFooter className='grid grid-cols-2 gap-2 sm:flex'>
          <Button onClick={handleReset} variant='outline' type='button'>
            <RotateCcw className='mr-2 h-4 w-4' />
            {t('Reset')}
          </Button>
          <Button onClick={handleApply} type='submit'>
            <Search className='mr-2 h-4 w-4' />
            {t('Apply Filters')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
