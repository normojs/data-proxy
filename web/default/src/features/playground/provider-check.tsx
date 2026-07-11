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
import { Link } from '@tanstack/react-router'
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
  Play,
  RotateCcw,
  Server,
  ShieldCheck,
  Terminal,
  XCircle,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import { CopyButton } from '@/components/copy-button'
import { PasswordInput } from '@/components/password-input'
import { checkPlaygroundProvider } from './api'
import type { ProviderCheckResult } from './types'

const DEFAULT_PROMPT = 'Reply exactly with OK.'
const DEFAULT_TIMEOUT_SECONDS = 20

type ProviderCheckForm = {
  baseUrl: string
  key: string
  model: string
  prompt: string
}

const DEFAULT_FORM: ProviderCheckForm = {
  baseUrl: '',
  key: '',
  model: '',
  prompt: DEFAULT_PROMPT,
}

function normalizeProviderEndpoint(baseUrl: string) {
  const raw = baseUrl.trim()
  if (!raw) return ''
  const withScheme = raw.includes('://') ? raw : `https://${raw}`

  try {
    const url = new URL(withScheme)
    const cleanPath = url.pathname.replace(/\/+$/, '')
    if (!cleanPath) {
      url.pathname = '/v1/chat/completions'
    } else if (cleanPath.endsWith('/chat/completions')) {
      url.pathname = cleanPath
    } else if (cleanPath.endsWith('/v1')) {
      url.pathname = `${cleanPath}/chat/completions`
    } else {
      url.pathname = `${cleanPath}/v1/chat/completions`
    }
    url.search = ''
    url.hash = ''
    return url.toString()
  } catch {
    return withScheme
  }
}

function providerCheckPayload(model: string, prompt: string) {
  return {
    model: model.trim() || '<MODEL>',
    messages: [
      {
        role: 'user',
        content: prompt.trim() || DEFAULT_PROMPT,
      },
    ],
    temperature: 0,
    max_tokens: 8,
    stream: false,
  }
}

function normalizeBearerHeader(key: string) {
  const trimmed = key.trim()
  if (!trimmed) return 'Bearer <YOUR_API_KEY>'
  return trimmed.toLowerCase().startsWith('bearer ')
    ? trimmed
    : `Bearer ${trimmed}`
}

function shellQuote(value: string) {
  return `'${value.replaceAll("'", "'\"'\"'")}'`
}

function buildCurl(form: ProviderCheckForm) {
  const endpoint =
    normalizeProviderEndpoint(form.baseUrl) || '<BASE_URL>/v1/chat/completions'
  const body = JSON.stringify(
    providerCheckPayload(form.model, form.prompt),
    null,
    2
  )

  return [
    `curl ${shellQuote(endpoint)} \\`,
    `  -H ${shellQuote(`Authorization: ${normalizeBearerHeader(form.key)}`)} \\`,
    `  -H ${shellQuote('Content-Type: application/json')} \\`,
    `  -d ${shellQuote(body)}`,
  ].join('\n')
}

function errorMessage(error: unknown) {
  if (error instanceof Error && error.message) return error.message
  return 'Provider check failed'
}

function DetailRow({
  label,
  value,
  valueClassName,
}: {
  label: string
  value?: string | number
  valueClassName?: string
}) {
  if (value === undefined || value === '') return null
  return (
    <div className='grid grid-cols-[7rem_minmax(0,1fr)] gap-3 py-2 text-sm'>
      <dt className='text-muted-foreground'>{label}</dt>
      <dd className={cn('min-w-0 font-medium break-words', valueClassName)}>
        {value}
      </dd>
    </div>
  )
}

function ResultPanel({
  result,
  error,
}: {
  result: ProviderCheckResult | null
  error: string
}) {
  const { t } = useTranslation()

  if (error) {
    return (
      <Alert variant='destructive'>
        <AlertCircle className='size-4' />
        <AlertTitle>{t('Check failed')}</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  if (!result) {
    return (
      <div className='text-muted-foreground flex min-h-32 items-center justify-center rounded-lg border border-dashed text-sm'>
        {t('No provider check has run yet.')}
      </div>
    )
  }

  return (
    <div className='space-y-4'>
      <Alert
        className={cn(
          result.ok
            ? 'border-success/30 bg-success/5 text-success'
            : 'border-destructive/30 bg-destructive/5 text-destructive'
        )}
      >
        {result.ok ? (
          <CheckCircle2 className='size-4' />
        ) : (
          <XCircle className='size-4' />
        )}
        <AlertTitle>
          {result.ok
            ? t('Provider is available')
            : t('Provider is not available')}
        </AlertTitle>
        <AlertDescription className='text-current/80'>
          {result.ok
            ? t('The model returned a successful chat completion response.')
            : result.error_message || t('The provider returned an error.')}
        </AlertDescription>
      </Alert>

      <dl className='divide-y rounded-lg border px-3'>
        <DetailRow label={t('Endpoint')} value={result.endpoint} />
        <DetailRow label={t('HTTP Status')} value={result.status} />
        <DetailRow label={t('Duration')} value={`${result.duration_ms} ms`} />
        <DetailRow label={t('Model')} value={result.response_model} />
        <DetailRow label={t('Response ID')} value={result.response_id} />
        <DetailRow
          label={t('Error Code')}
          value={result.error_code}
          valueClassName='text-destructive'
        />
      </dl>

      {result.output_preview && (
        <div className='space-y-2'>
          <Label>{t('Output')}</Label>
          <pre className='bg-muted/50 max-h-40 overflow-auto rounded-lg border p-3 text-sm whitespace-pre-wrap'>
            {result.output_preview}
          </pre>
        </div>
      )}

      {result.response_preview && (
        <div className='space-y-2'>
          <div className='flex items-center justify-between gap-2'>
            <Label>{t('Raw response')}</Label>
            {result.response_truncated && (
              <Badge variant='outline'>{t('Truncated')}</Badge>
            )}
          </div>
          <pre className='bg-muted/50 max-h-64 overflow-auto rounded-lg border p-3 font-mono text-xs whitespace-pre-wrap'>
            {result.response_preview}
          </pre>
        </div>
      )}
    </div>
  )
}

export function ProviderCheck() {
  const { t } = useTranslation()
  const [form, setForm] = useState<ProviderCheckForm>(DEFAULT_FORM)
  const [result, setResult] = useState<ProviderCheckResult | null>(null)
  const [error, setError] = useState('')
  const [isChecking, setIsChecking] = useState(false)

  const endpoint = useMemo(
    () => normalizeProviderEndpoint(form.baseUrl),
    [form.baseUrl]
  )
  const curl = useMemo(() => buildCurl(form), [form])
  const canCheck =
    !!form.baseUrl.trim() && !!form.key.trim() && !!form.model.trim()

  const updateField = (field: keyof ProviderCheckForm, value: string) => {
    setForm((current) => ({ ...current, [field]: value }))
  }

  const handleReset = () => {
    setForm(DEFAULT_FORM)
    setResult(null)
    setError('')
  }

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!canCheck || isChecking) return

    setIsChecking(true)
    setError('')
    setResult(null)
    try {
      const data = await checkPlaygroundProvider({
        base_url: form.baseUrl,
        key: form.key,
        model: form.model,
        prompt: form.prompt,
        timeout_seconds: DEFAULT_TIMEOUT_SECONDS,
      })
      setResult(data)
      if (data.ok) {
        toast.success(t('Provider is available'))
      } else {
        toast.error(data.error_message || t('Provider is not available'))
      }
    } catch (caught) {
      const message = errorMessage(caught)
      setError(message)
      toast.error(message)
    } finally {
      setIsChecking(false)
    }
  }

  return (
    <div className='bg-background flex h-full min-h-0 flex-col overflow-auto'>
      <div className='mx-auto flex w-full max-w-6xl flex-col gap-4 p-4 md:p-6'>
        <div className='flex flex-wrap items-start justify-between gap-3'>
          <div className='min-w-0 space-y-2'>
            <div className='flex items-center gap-2'>
              <div className='bg-muted flex size-8 items-center justify-center rounded-lg border'>
                <Server className='size-4' aria-hidden='true' />
              </div>
              <div>
                <h1 className='text-xl leading-tight font-semibold'>
                  {t('Provider Check')}
                </h1>
                <p className='text-muted-foreground text-sm'>
                  {t('OpenAI-compatible endpoint validation')}
                </p>
              </div>
            </div>
            <div className='flex flex-wrap gap-2'>
              <Badge variant='secondary'>
                <ShieldCheck className='size-3' aria-hidden='true' />
                {t('Server-side')}
              </Badge>
              <Badge variant='secondary'>{t('CORS-safe')}</Badge>
              <Badge variant='secondary'>{t('No key storage')}</Badge>
            </div>
          </div>
          <Button variant='outline' render={<Link to='/playground' />}>
            <ArrowLeft className='size-4' aria-hidden='true' />
            {t('Back to Playground')}
          </Button>
        </div>

        <div className='grid gap-4 lg:grid-cols-[minmax(0,0.95fr)_minmax(360px,1.05fr)]'>
          <Card>
            <CardHeader>
              <CardTitle>{t('Connection')}</CardTitle>
              <CardDescription>
                {t('Base URL, key, model, and probe message')}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form className='space-y-4' onSubmit={handleSubmit}>
                <div className='space-y-2'>
                  <Label htmlFor='provider-base-url'>{t('Base URL')}</Label>
                  <Input
                    id='provider-base-url'
                    autoComplete='url'
                    placeholder='https://api.openai.com/v1'
                    value={form.baseUrl}
                    onChange={(event) =>
                      updateField('baseUrl', event.target.value)
                    }
                  />
                  {endpoint && (
                    <p className='text-muted-foreground text-xs break-all'>
                      {endpoint}
                    </p>
                  )}
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='provider-key'>{t('Key')}</Label>
                  <PasswordInput
                    id='provider-key'
                    autoComplete='off'
                    placeholder='sk-...'
                    value={form.key}
                    onChange={(event) => updateField('key', event.target.value)}
                  />
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='provider-model'>{t('Model')}</Label>
                  <Input
                    id='provider-model'
                    autoComplete='off'
                    placeholder='gpt-4o-mini'
                    value={form.model}
                    onChange={(event) =>
                      updateField('model', event.target.value)
                    }
                  />
                </div>

                <div className='space-y-2'>
                  <Label htmlFor='provider-prompt'>{t('Probe Message')}</Label>
                  <Textarea
                    id='provider-prompt'
                    rows={3}
                    value={form.prompt}
                    onChange={(event) =>
                      updateField('prompt', event.target.value)
                    }
                  />
                </div>

                <Separator />

                <div className='flex flex-wrap items-center gap-2'>
                  <Button type='submit' disabled={!canCheck || isChecking}>
                    {isChecking ? (
                      <Spinner className='size-4' aria-hidden='true' />
                    ) : (
                      <Play className='size-4' aria-hidden='true' />
                    )}
                    {isChecking ? t('Checking') : t('Run Check')}
                  </Button>
                  <Button
                    type='button'
                    variant='outline'
                    onClick={handleReset}
                    disabled={isChecking}
                  >
                    <RotateCcw className='size-4' aria-hidden='true' />
                    {t('Reset')}
                  </Button>
                </div>
              </form>
            </CardContent>
          </Card>

          <div className='flex min-w-0 flex-col gap-4'>
            <Card>
              <CardHeader>
                <CardTitle className='flex items-center gap-2'>
                  <Terminal className='size-4' aria-hidden='true' />
                  {t('cURL')}
                </CardTitle>
                <CardDescription>
                  {t('Matches the same endpoint and payload used by the check')}
                </CardDescription>
                <CardAction>
                  <CopyButton
                    value={curl}
                    variant='outline'
                    tooltip={t('Copy cURL')}
                    successTooltip={t('Copied')}
                  />
                </CardAction>
              </CardHeader>
              <CardContent>
                <pre className='bg-muted/50 max-h-80 overflow-auto rounded-lg border p-3 font-mono text-xs whitespace-pre-wrap'>
                  {curl}
                </pre>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>{t('Result')}</CardTitle>
                <CardDescription>
                  {t('Status, latency, model response, and upstream body')}
                </CardDescription>
              </CardHeader>
              <CardContent>
                <ResultPanel result={result} error={error} />
              </CardContent>
            </Card>
          </div>
        </div>
      </div>
    </div>
  )
}
