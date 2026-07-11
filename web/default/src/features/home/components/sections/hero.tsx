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
import {
  Activity,
  ArrowRight,
  BookOpen,
  Braces,
  Cable,
  CircleDollarSign,
  ExternalLink,
  KeyRound,
  ShieldCheck,
  type LucideIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useStatus } from '@/hooks/use-status'
import { Button } from '@/components/ui/button'
import { HeroTerminalDemo } from '../hero-terminal-demo'

interface HeroProps {
  className?: string
  isAuthenticated?: boolean
}

function CapabilityPill({
  icon: Icon,
  label,
}: {
  icon: LucideIcon
  label: string
}) {
  return (
    <div className='border-border/70 bg-background/70 text-muted-foreground flex items-center gap-2 rounded-lg border px-3 py-2 text-xs'>
      <Icon className='text-foreground size-3.5' strokeWidth={1.8} />
      <span>{label}</span>
    </div>
  )
}

function RoutePreview() {
  const { t } = useTranslation()
  const rows = [
    {
      name: 'OpenAI',
      route: '/v1/chat/completions',
      state: t('Healthy'),
      value: '142 ms',
    },
    {
      name: 'Claude',
      route: '/v1/messages',
      state: t('Guarded'),
      value: '29 tok',
    },
    {
      name: 'MCP',
      route: 'bridge://workspace/tools',
      state: t('Audited'),
      value: '$0.0008',
    },
  ]

  return (
    <div className='border-border/70 bg-muted/20 overflow-hidden rounded-xl border'>
      <div className='border-border/70 flex items-center justify-between border-b px-4 py-3'>
        <div>
          <p className='text-sm font-semibold'>{t('Live routing ledger')}</p>
          <p className='text-muted-foreground text-xs'>
            {t('Protocol, cost, and policy signals in one place')}
          </p>
        </div>
        <Activity className='size-4 text-emerald-600' />
      </div>
      <div className='divide-border/70 divide-y'>
        {rows.map((row) => (
          <div
            key={row.name}
            className='grid grid-cols-[72px_1fr_auto] items-center gap-3 px-4 py-3 text-xs'
          >
            <span className='font-medium'>{row.name}</span>
            <span className='text-muted-foreground truncate font-mono'>
              {row.route}
            </span>
            <div className='flex items-center gap-2'>
              <span className='text-muted-foreground'>{row.value}</span>
              <span className='rounded-md border border-emerald-600/20 bg-emerald-600/10 px-1.5 py-0.5 text-[11px] font-medium text-emerald-700 dark:text-emerald-400'>
                {row.state}
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

export function Hero(props: HeroProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'
  const systemName = status?.system_name || 'Data Proxy'
  const logo = status?.logo || '/logo.png'

  const renderDocsButton = () => {
    const isExternal = docsUrl.startsWith('http')
    if (isExternal) {
      return (
        <Button
          variant='outline'
          className='h-10 rounded-lg px-4'
          render={
            <a href={docsUrl} target='_blank' rel='noopener noreferrer' />
          }
        >
          <BookOpen className='size-4' />
          {t('Docs')}
          <ExternalLink className='size-3.5' />
        </Button>
      )
    }
    return (
      <Button
        variant='outline'
        className='h-10 rounded-lg px-4'
        render={<Link to={docsUrl} />}
      >
        <BookOpen className='size-4' />
        {t('Docs')}
      </Button>
    )
  }

  return (
    <section className='relative overflow-hidden border-b px-6 pt-20 pb-12 md:pt-28 md:pb-16'>
      <div
        aria-hidden
        className='absolute inset-0 -z-10 bg-[linear-gradient(to_right,var(--border)_1px,transparent_1px),linear-gradient(to_bottom,var(--border)_1px,transparent_1px)] [mask-image:linear-gradient(to_bottom,black,transparent_80%)] bg-[size:3rem_3rem] opacity-[0.12]'
      />
      <div className='mx-auto grid max-w-6xl gap-10 lg:grid-cols-[0.92fr_1.08fr] lg:items-center'>
        <div className='min-w-0'>
          <div
            className='landing-animate-fade-up border-border bg-background mb-5 inline-flex items-center gap-2 rounded-full border px-2.5 py-1.5 opacity-0'
            style={{ animationDelay: '0ms' }}
          >
            <img
              src={logo}
              alt={systemName}
              className='size-5 rounded-md object-cover'
            />
            <span className='text-xs font-medium'>{systemName}</span>
            <span className='bg-muted text-muted-foreground rounded-full px-2 py-0.5 text-[11px]'>
              {t('Operations control plane')}
            </span>
          </div>

          <h1
            className='landing-animate-fade-up max-w-3xl text-4xl leading-[1.08] font-semibold tracking-tight text-balance opacity-0 md:text-5xl'
            style={{ animationDelay: '60ms' }}
          >
            {t(
              'Run model, MCP, and Bridge traffic through one governed gateway'
            )}
          </h1>
          <p
            className='landing-animate-fade-up text-muted-foreground mt-5 max-w-2xl text-base leading-7 text-pretty opacity-0'
            style={{ animationDelay: '120ms' }}
          >
            {t(
              'Data Proxy gives operators a unified API surface for provider routing, local tools, quotas, billing, and audit trails without hiding the operational state.'
            )}
          </p>

          <div
            className='landing-animate-fade-up mt-7 flex flex-wrap items-center gap-3 opacity-0'
            style={{ animationDelay: '180ms' }}
          >
            {props.isAuthenticated ? (
              <Button
                className='h-10 rounded-lg px-4'
                render={<Link to='/dashboard' />}
              >
                {t('Go to Dashboard')}
                <ArrowRight className='size-4' />
              </Button>
            ) : (
              <>
                <Button
                  className='h-10 rounded-lg px-4'
                  render={<Link to='/sign-up' />}
                >
                  {t('Get Started')}
                  <ArrowRight className='size-4' />
                </Button>
                <Button
                  variant='outline'
                  className='h-10 rounded-lg px-4'
                  render={<Link to='/pricing' />}
                >
                  {t('View Pricing')}
                </Button>
              </>
            )}
            {renderDocsButton()}
          </div>

          <div
            className='landing-animate-fade-up mt-8 grid max-w-2xl grid-cols-2 gap-2 opacity-0 sm:grid-cols-4'
            style={{ animationDelay: '240ms' }}
          >
            <CapabilityPill icon={Braces} label={t('OpenAI compatible')} />
            <CapabilityPill icon={Cable} label={t('MCP proxy')} />
            <CapabilityPill icon={KeyRound} label={t('Key governance')} />
            <CapabilityPill
              icon={CircleDollarSign}
              label={t('Quota billing')}
            />
          </div>
        </div>

        <div
          className='landing-animate-fade-up grid min-w-0 gap-4 opacity-0'
          style={{ animationDelay: '300ms' }}
        >
          <div className='border-border bg-background overflow-hidden rounded-xl border'>
            <div className='border-border flex items-center justify-between border-b px-4 py-3'>
              <div className='flex items-center gap-2'>
                <ShieldCheck className='size-4 text-emerald-600' />
                <span className='text-sm font-semibold'>
                  {t('Gateway execution preview')}
                </span>
              </div>
              <span className='text-muted-foreground text-xs'>
                {t('SSE ready')}
              </span>
            </div>
            <div className='p-3 sm:p-4'>
              <HeroTerminalDemo />
            </div>
          </div>
          <RoutePreview />
        </div>
      </div>
    </section>
  )
}
