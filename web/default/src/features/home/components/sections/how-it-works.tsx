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
import { ClipboardCheck, Gauge, GitBranch, KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AnimateInView } from '@/components/animate-in-view'

export function HowItWorks() {
  const { t } = useTranslation()

  const steps = [
    {
      num: '01',
      title: t('Authenticate the caller'),
      desc: t(
        'API keys, groups, quotas, and policy limits are resolved before traffic reaches a provider or tool.'
      ),
      icon: KeyRound,
    },
    {
      num: '02',
      title: t('Route with context'),
      desc: t(
        'Requests are mapped to channels, MCP servers, Bridge clients, or OpenAPI tools with observable health signals.'
      ),
      icon: GitBranch,
    },
    {
      num: '03',
      title: t('Settle usage'),
      desc: t(
        'Token usage, binary objects, download links, and billing events are recorded against the same operational ledger.'
      ),
      icon: Gauge,
    },
    {
      num: '04',
      title: t('Review exceptions'),
      desc: t(
        'Failures, stale clients, high-error tools, and reconciliation anomalies are surfaced for administrators.'
      ),
      icon: ClipboardCheck,
    },
  ]

  return (
    <section className='border-border/60 relative z-10 border-t px-6 py-20 md:py-24'>
      <div className='mx-auto max-w-6xl'>
        <AnimateInView className='mb-10 flex flex-col gap-4 md:flex-row md:items-end md:justify-between'>
          <div className='max-w-2xl'>
            <p className='text-muted-foreground mb-3 text-sm font-medium'>
              {t('Request lifecycle')}
            </p>
            <h2 className='text-2xl font-semibold tracking-tight text-balance md:text-3xl'>
              {t('From API key to audit trail, every step stays accountable')}
            </h2>
          </div>
          <p className='text-muted-foreground max-w-md text-sm leading-6'>
            {t(
              'The gateway is not a black box. It keeps request identity, route choice, cost, and operator follow-up connected.'
            )}
          </p>
        </AnimateInView>

        <div className='grid gap-3 md:grid-cols-4'>
          {steps.map((step, index) => {
            const Icon = step.icon
            return (
              <AnimateInView
                key={step.num}
                delay={index * 80}
                animation='fade-up'
                className='border-border/80 bg-background rounded-xl border p-5'
              >
                <div className='mb-5 flex items-center justify-between'>
                  <span className='text-muted-foreground font-mono text-xs'>
                    {step.num}
                  </span>
                  <Icon className='text-muted-foreground size-4' />
                </div>
                <h3 className='text-sm font-semibold'>{step.title}</h3>
                <p className='text-muted-foreground mt-2 text-xs leading-5'>
                  {step.desc}
                </p>
              </AnimateInView>
            )
          })}
        </div>
      </div>
    </section>
  )
}
