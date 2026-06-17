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

import {
  AlertTriangle,
  CheckCircle2,
  CircleHelp,
  Info,
  MinusCircle,
  Radio,
  ShieldAlert,
  XCircle,
  type LucideIcon,
} from 'lucide-react'
import type {
  ServiceSignalState,
  ServiceStatusKind,
  ServiceStatusSummary,
} from './types'

export interface ServiceStatusMeta {
  icon: LucideIcon
  labelKey: string
  shortLabelKey: string
  textClassName: string
  dotClassName: string
  bgClassName: string
}

const STATUS_META: Record<ServiceStatusKind, ServiceStatusMeta> = {
  normal: {
    icon: CheckCircle2,
    labelKey: 'Normal',
    shortLabelKey: 'Normal',
    textClassName: 'text-success',
    dotClassName: 'bg-success',
    bgClassName: 'bg-success/10 text-success',
  },
  degraded: {
    icon: AlertTriangle,
    labelKey: 'Degraded',
    shortLabelKey: 'Degraded',
    textClassName: 'text-warning',
    dotClassName: 'bg-warning',
    bgClassName: 'bg-warning/10 text-warning',
  },
  outage: {
    icon: XCircle,
    labelKey: 'Outage',
    shortLabelKey: 'Outage',
    textClassName: 'text-destructive',
    dotClassName: 'bg-destructive',
    bgClassName: 'bg-destructive/10 text-destructive',
  },
  no_traffic: {
    icon: MinusCircle,
    labelKey: 'No traffic',
    shortLabelKey: 'No traffic',
    textClassName: 'text-muted-foreground',
    dotClassName: 'bg-muted-foreground',
    bgClassName: 'bg-muted text-muted-foreground',
  },
  only_probe: {
    icon: Radio,
    labelKey: 'Probe only',
    shortLabelKey: 'Probe only',
    textClassName: 'text-info',
    dotClassName: 'bg-info',
    bgClassName: 'bg-info/10 text-info',
  },
  low_confidence: {
    icon: CircleHelp,
    labelKey: 'Low confidence',
    shortLabelKey: 'Low confidence',
    textClassName: 'text-warning',
    dotClassName: 'bg-warning',
    bgClassName: 'bg-warning/10 text-warning',
  },
  unknown: {
    icon: Info,
    labelKey: 'Unknown',
    shortLabelKey: 'Unknown',
    textClassName: 'text-muted-foreground',
    dotClassName: 'bg-muted-foreground',
    bgClassName: 'bg-muted text-muted-foreground',
  },
}

export function getStatusMeta(status: ServiceStatusKind): ServiceStatusMeta {
  return STATUS_META[status] ?? STATUS_META.unknown
}

export function getOverallStatus(
  summary?: ServiceStatusSummary
): ServiceStatusKind {
  if (!summary || summary.total_channels <= 0) return 'unknown'
  if (summary.outage > 0) return 'outage'
  if (summary.degraded > 0) return 'degraded'
  if (summary.low_confidence > 0) return 'low_confidence'
  if (summary.normal > 0) return 'normal'
  if (summary.no_traffic > 0) return 'no_traffic'
  if (summary.only_probe > 0) return 'only_probe'
  return 'unknown'
}

export function getSignalLabelKey(state: ServiceSignalState): string {
  switch (state) {
    case 'observed':
      return 'Observed'
    case 'not_observed':
      return 'Not observed'
    case 'not_configured':
      return 'Not connected'
    default:
      return 'Unknown'
  }
}

export function getSignalIcon(state: ServiceSignalState): LucideIcon {
  if (state === 'observed') return CheckCircle2
  if (state === 'not_configured') return MinusCircle
  return ShieldAlert
}

export function getSignalClassName(state: ServiceSignalState): string {
  if (state === 'observed') return 'text-success'
  if (state === 'not_configured') return 'text-muted-foreground'
  return 'text-warning'
}
