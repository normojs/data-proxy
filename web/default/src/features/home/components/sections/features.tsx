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
import type { LucideIcon } from 'lucide-react'
import {
  Activity,
  BarChart3,
  Braces,
  Cable,
  Download,
  FileCode2,
  KeyRound,
  Route,
  ShieldCheck,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { AnimateInView } from '@/components/animate-in-view'

interface FeaturesProps {
  className?: string
}

type FeatureTone = 'emerald' | 'blue' | 'amber' | 'violet'

const toneClassName: Record<FeatureTone, string> = {
  emerald:
    'bg-emerald-500/10 text-emerald-700 ring-emerald-600/15 dark:text-emerald-300',
  blue: 'bg-blue-500/10 text-blue-700 ring-blue-600/15 dark:text-blue-300',
  amber: 'bg-amber-500/10 text-amber-700 ring-amber-600/15 dark:text-amber-300',
  violet:
    'bg-violet-500/10 text-violet-700 ring-violet-600/15 dark:text-violet-300',
}

function FeatureIcon({
  icon: Icon,
  tone,
}: {
  icon: LucideIcon
  tone: FeatureTone
}) {
  return (
    <div
      className={`${toneClassName[tone]} flex size-9 items-center justify-center rounded-lg ring-1`}
    >
      <Icon className='size-4' strokeWidth={1.8} />
    </div>
  )
}

export function Features(_props: FeaturesProps) {
  const { t } = useTranslation()

  const primaryFeatures = [
    {
      title: t('Provider routing that stays inspectable'),
      desc: t(
        'Route OpenAI-compatible, Claude, Gemini, and custom providers through one control surface while keeping latency, cost, and policy state visible.'
      ),
      icon: Route,
      tone: 'emerald' as const,
      visual: ['OpenAI', 'Claude', 'Gemini', 'DeepSeek', 'Qwen', 'Custom'],
    },
    {
      title: t('MCP and Bridge execution with guardrails'),
      desc: t(
        'Proxy remote MCP servers and local Bridge clients with heartbeats, write controls, target allowlists, and auditable tool calls.'
      ),
      icon: Cable,
      tone: 'blue' as const,
      visual: [t('Proxy health'), t('Bridge heartbeat'), t('Tool policy')],
    },
    {
      title: t('Keys, quota, and billing in one ledger'),
      desc: t(
        'Issue API keys, assign groups, track quota movement, reconcile billing events, and expose exchange-rate aware currency display.'
      ),
      icon: KeyRound,
      tone: 'amber' as const,
      visual: [t('API keys'), t('Quota'), t('Billing events')],
    },
    {
      title: t('OpenAPI imports beyond text-only tools'),
      desc: t(
        'Reuse schemas, store binary OpenAPI responses as managed objects, and issue authorized download links for generated artifacts.'
      ),
      icon: Download,
      tone: 'violet' as const,
      visual: [t('Schema reuse'), t('Binary objects'), t('Download audit')],
    },
  ]

  const secondaryFeatures = [
    {
      icon: Braces,
      title: t('Unified API surface'),
      desc: t(
        'Compatible routes for chat, responses, embeddings, images, and provider-native protocols.'
      ),
    },
    {
      icon: BarChart3,
      title: t('Operational trends'),
      desc: t(
        'Storage, Bridge sessions, proxy errors, and billing anomalies are visible from the dashboard.'
      ),
    },
    {
      icon: ShieldCheck,
      title: t('Review queue'),
      desc: t(
        'Health checks, stale clients, and high-error tools become concrete operator work items.'
      ),
    },
    {
      icon: FileCode2,
      title: t('Import discipline'),
      desc: t(
        'Preview schema reuse, skipped operations, and diff summaries before enabling tools.'
      ),
    },
  ]

  return (
    <section className='relative z-10 px-6 py-20 md:py-24'>
      <div className='mx-auto max-w-6xl'>
        <AnimateInView className='mb-10 max-w-2xl'>
          <p className='text-muted-foreground mb-3 text-sm font-medium'>
            {t('Gateway capabilities')}
          </p>
          <h2 className='max-w-2xl text-2xl leading-tight font-semibold tracking-tight text-balance md:text-3xl'>
            {t('A precise operating layer for model and tool traffic')}
          </h2>
          <p className='text-muted-foreground mt-4 text-sm leading-6 text-pretty'>
            {t(
              'Data Proxy is designed for operators who need the gateway to be powerful without becoming opaque.'
            )}
          </p>
        </AnimateInView>

        <div className='border-border bg-border grid overflow-hidden rounded-xl border md:grid-cols-2'>
          {primaryFeatures.map((feature, index) => (
            <AnimateInView
              key={feature.title}
              delay={index * 80}
              animation='fade-up'
              className='bg-background min-h-[248px] p-6 md:p-7'
            >
              <div className='flex items-start gap-4'>
                <FeatureIcon icon={feature.icon} tone={feature.tone} />
                <div className='min-w-0'>
                  <h3 className='text-base font-semibold tracking-tight'>
                    {feature.title}
                  </h3>
                  <p className='text-muted-foreground mt-2 max-w-xl text-sm leading-6'>
                    {feature.desc}
                  </p>
                </div>
              </div>
              <div className='mt-6 flex flex-wrap gap-2'>
                {feature.visual.map((item) => (
                  <span
                    key={item}
                    className='border-border bg-muted/30 text-muted-foreground rounded-md border px-2.5 py-1 text-xs'
                  >
                    {item}
                  </span>
                ))}
              </div>
            </AnimateInView>
          ))}
        </div>

        <div className='mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
          {secondaryFeatures.map((feature, index) => {
            const Icon = feature.icon
            return (
              <AnimateInView
                key={feature.title}
                delay={index * 70}
                animation='fade-up'
                className='border-border/80 bg-background rounded-xl border p-5'
              >
                <div className='text-muted-foreground mb-4 flex items-center gap-2'>
                  <Icon className='size-4' strokeWidth={1.8} />
                </div>
                <h3 className='text-sm font-semibold'>{feature.title}</h3>
                <p className='text-muted-foreground mt-2 text-xs leading-5'>
                  {feature.desc}
                </p>
              </AnimateInView>
            )
          })}
        </div>

        <AnimateInView className='border-border/80 bg-muted/20 mt-8 flex flex-col gap-4 rounded-xl border p-5 sm:flex-row sm:items-center sm:justify-between'>
          <div>
            <p className='text-sm font-semibold'>
              {t('Designed for daily operations')}
            </p>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t(
                'Every capability is paired with status, audit context, and an operator action.'
              )}
            </p>
          </div>
          <div className='text-muted-foreground flex items-center gap-2 text-xs'>
            <Activity className='size-4 text-emerald-600' />
            {t('Observable by default')}
          </div>
        </AnimateInView>
      </div>
    </section>
  )
}
