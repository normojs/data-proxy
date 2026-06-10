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
import { getRouteApi } from '@tanstack/react-router'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'

const route = getRouteApi('/_authenticated/mcp/$section')

type TimeRangeFilterProps = {
  endKey: 'callsEndTime' | 'billingEndTime' | 'auditEndTime'
  pageKey: 'callsPage' | 'billingPage' | 'auditPage'
  startKey: 'callsStartTime' | 'billingStartTime' | 'auditStartTime'
}

function toDate(value: unknown): Date | undefined {
  if (typeof value !== 'number') return undefined
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : date
}

export function timestampMsToSeconds(value: unknown): number | undefined {
  if (typeof value !== 'number' || !Number.isFinite(value)) return undefined
  return Math.floor(value / 1000)
}

export function TimeRangeFilter(props: TimeRangeFilterProps) {
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const start = toDate(search[props.startKey])
  const end = toDate(search[props.endKey])

  return (
    <CompactDateTimeRangePicker
      start={start}
      end={end}
      onChange={(range) => {
        void navigate({
          search: (prev) => ({
            ...prev,
            [props.pageKey]: undefined,
            [props.startKey]: range.start?.getTime(),
            [props.endKey]: range.end?.getTime(),
          }),
        })
      }}
      className='w-full sm:w-[280px] lg:w-[320px]'
    />
  )
}
