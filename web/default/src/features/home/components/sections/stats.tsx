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
import { Activity, CircleDollarSign, Network, ShieldCheck } from 'lucide-react'
import { useTranslation } from 'react-i18next'

interface StatsProps {
  className?: string
}

export function Stats(_props: StatsProps) {
  const { t } = useTranslation()

  const items = [
    {
      icon: Network,
      label: t('Protocol surface'),
      value: t('Models, MCP, Bridge'),
      desc: t('One gateway for provider APIs and tool execution paths.'),
    },
    {
      icon: ShieldCheck,
      label: t('Runtime policy'),
      value: t('Keys, groups, limits'),
      desc: t('Access and quota rules stay attached to every request.'),
    },
    {
      icon: CircleDollarSign,
      label: t('Cost state'),
      value: t('Ledger and rates'),
      desc: t('Billing, quota movement, and exchange rates share one view.'),
    },
    {
      icon: Activity,
      label: t('Operator review'),
      value: t('Health and audit'),
      desc: t('Failures become reviewable records instead of hidden logs.'),
    },
  ]

  return (
    <div className='border-border/60 bg-muted/10 relative z-10 border-y'>
      <div className='mx-auto max-w-6xl px-6 py-6'>
        <div className='bg-border grid gap-px overflow-hidden rounded-xl border sm:grid-cols-2 lg:grid-cols-4'>
          {items.map((item) => {
            const Icon = item.icon
            return (
              <div key={item.label} className='bg-background p-4'>
                <div className='text-muted-foreground flex items-center gap-2 text-xs'>
                  <Icon className='size-4' strokeWidth={1.8} />
                  <span>{item.label}</span>
                </div>
                <p className='mt-3 text-sm font-semibold'>{item.value}</p>
                <p className='text-muted-foreground mt-1 text-xs leading-5'>
                  {item.desc}
                </p>
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}
