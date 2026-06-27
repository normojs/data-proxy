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
import { useMemo, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { AlertTriangle, Route } from 'lucide-react'
import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

const numericString = z.string().refine((value) => {
  const trimmed = value.trim()
  if (!trimmed) return true
  return !Number.isNaN(Number(trimmed)) && Number(trimmed) >= 0
}, 'Enter a non-negative number or leave empty')

const monitoringSchema = z
  .object({
    ChannelDisableThreshold: numericString,
    QuotaRemindThreshold: numericString,
    AutomaticDisableChannelEnabled: z.boolean(),
    AutomaticEnableChannelEnabled: z.boolean(),
    AutomaticDisableKeywords: z.string(),
    AutomaticDisableStatusCodes: z.string(),
    ChannelHealthTransientKeywords: z.string(),
    ChannelHealthTransientStatusCodes: z.string(),
    ChannelHealthFailureThreshold: z.coerce
      .number()
      .int()
      .min(1, 'Threshold must be at least 1'),
    ChannelHealthFailureWindowMinutes: z.coerce
      .number()
      .int()
      .min(1, 'Window must be at least 1 minute'),
    ChannelHealthCooldownMinutes: z.coerce
      .number()
      .int()
      .min(1, 'Cooldown must be at least 1 minute'),
    ChannelHealthMaxCooldownMinutes: z.coerce
      .number()
      .int()
      .min(1, 'Max cooldown must be at least 1 minute'),
    AutomaticRetryStatusCodes: z.string(),
    monitor_setting: z.object({
      auto_test_channel_enabled: z.boolean(),
      auto_test_channel_minutes: z.coerce
        .number()
        .int()
        .min(1, 'Interval must be at least 1 minute'),
    }),
  })
  .superRefine((values, ctx) => {
    const disableParsed = parseHttpStatusCodeRules(
      values.AutomaticDisableStatusCodes
    )
    if (!disableParsed.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['AutomaticDisableStatusCodes'],
        message: `Invalid status code rules: ${disableParsed.invalidTokens.join(
          ', '
        )}`,
      })
    }

    const retryParsed = parseHttpStatusCodeRules(
      values.AutomaticRetryStatusCodes
    )
    if (!retryParsed.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['AutomaticRetryStatusCodes'],
        message: `Invalid status code rules: ${retryParsed.invalidTokens.join(
          ', '
        )}`,
      })
    }

    const transientParsed = parseHttpStatusCodeRules(
      values.ChannelHealthTransientStatusCodes
    )
    if (!transientParsed.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['ChannelHealthTransientStatusCodes'],
        message: `Invalid status code rules: ${transientParsed.invalidTokens.join(
          ', '
        )}`,
      })
    }

    if (
      values.ChannelHealthMaxCooldownMinutes <
      values.ChannelHealthCooldownMinutes
    ) {
      ctx.addIssue({
        code: 'custom',
        path: ['ChannelHealthMaxCooldownMinutes'],
        message: 'Max cooldown must be greater than or equal to cooldown',
      })
    }
  })

type MonitoringFormValues = z.output<typeof monitoringSchema>
type MonitoringFormInput = z.input<typeof monitoringSchema>

type MonitoringSettingsSectionProps = {
  defaultValues: {
    ChannelDisableThreshold: string
    QuotaRemindThreshold: string
    AutomaticDisableChannelEnabled: boolean
    AutomaticEnableChannelEnabled: boolean
    AutomaticDisableKeywords: string
    AutomaticDisableStatusCodes: string
    ChannelHealthTransientKeywords: string
    ChannelHealthTransientStatusCodes: string
    ChannelHealthFailureThreshold: number
    ChannelHealthFailureWindowMinutes: number
    ChannelHealthCooldownMinutes: number
    ChannelHealthMaxCooldownMinutes: number
    AutomaticRetryStatusCodes: string
    'monitor_setting.auto_test_channel_enabled': boolean
    'monitor_setting.auto_test_channel_minutes': number
  }
}

function normalizeLineEndings(value: string) {
  return value.replace(/\r\n/g, '\n')
}

type NormalizedMonitoringValues = {
  ChannelDisableThreshold: string
  QuotaRemindThreshold: string
  AutomaticDisableChannelEnabled: boolean
  AutomaticEnableChannelEnabled: boolean
  AutomaticDisableKeywords: string
  AutomaticDisableStatusCodes: string
  ChannelHealthTransientKeywords: string
  ChannelHealthTransientStatusCodes: string
  ChannelHealthFailureThreshold: number
  ChannelHealthFailureWindowMinutes: number
  ChannelHealthCooldownMinutes: number
  ChannelHealthMaxCooldownMinutes: number
  AutomaticRetryStatusCodes: string
  'monitor_setting.auto_test_channel_enabled': boolean
  'monitor_setting.auto_test_channel_minutes': number
}

const buildFormDefaults = (
  defaults: MonitoringSettingsSectionProps['defaultValues']
): MonitoringFormInput => ({
  ChannelDisableThreshold: defaults.ChannelDisableThreshold ?? '',
  QuotaRemindThreshold: defaults.QuotaRemindThreshold ?? '',
  AutomaticDisableChannelEnabled: defaults.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: defaults.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    defaults.AutomaticDisableKeywords ?? ''
  ),
  AutomaticDisableStatusCodes: defaults.AutomaticDisableStatusCodes ?? '',
  ChannelHealthTransientKeywords: normalizeLineEndings(
    defaults.ChannelHealthTransientKeywords ?? ''
  ),
  ChannelHealthTransientStatusCodes:
    defaults.ChannelHealthTransientStatusCodes ?? '',
  ChannelHealthFailureThreshold: defaults.ChannelHealthFailureThreshold ?? 3,
  ChannelHealthFailureWindowMinutes:
    defaults.ChannelHealthFailureWindowMinutes ?? 5,
  ChannelHealthCooldownMinutes: defaults.ChannelHealthCooldownMinutes ?? 2,
  ChannelHealthMaxCooldownMinutes: defaults.ChannelHealthMaxCooldownMinutes ?? 10,
  AutomaticRetryStatusCodes: defaults.AutomaticRetryStatusCodes ?? '',
  monitor_setting: {
    auto_test_channel_enabled:
      defaults['monitor_setting.auto_test_channel_enabled'],
    auto_test_channel_minutes:
      defaults['monitor_setting.auto_test_channel_minutes'],
  },
})

const normalizeDefaults = (
  defaults: MonitoringSettingsSectionProps['defaultValues']
): NormalizedMonitoringValues => ({
  ChannelDisableThreshold: (defaults.ChannelDisableThreshold ?? '').trim(),
  QuotaRemindThreshold: (defaults.QuotaRemindThreshold ?? '').trim(),
  AutomaticDisableChannelEnabled: defaults.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: defaults.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    defaults.AutomaticDisableKeywords ?? ''
  ),
  AutomaticDisableStatusCodes: parseHttpStatusCodeRules(
    defaults.AutomaticDisableStatusCodes ?? ''
  ).normalized,
  ChannelHealthTransientKeywords: normalizeLineEndings(
    defaults.ChannelHealthTransientKeywords ?? ''
  ),
  ChannelHealthTransientStatusCodes: parseHttpStatusCodeRules(
    defaults.ChannelHealthTransientStatusCodes ?? ''
  ).normalized,
  ChannelHealthFailureThreshold: defaults.ChannelHealthFailureThreshold ?? 3,
  ChannelHealthFailureWindowMinutes:
    defaults.ChannelHealthFailureWindowMinutes ?? 5,
  ChannelHealthCooldownMinutes: defaults.ChannelHealthCooldownMinutes ?? 2,
  ChannelHealthMaxCooldownMinutes: defaults.ChannelHealthMaxCooldownMinutes ?? 10,
  AutomaticRetryStatusCodes: parseHttpStatusCodeRules(
    defaults.AutomaticRetryStatusCodes ?? ''
  ).normalized,
  'monitor_setting.auto_test_channel_enabled':
    defaults['monitor_setting.auto_test_channel_enabled'],
  'monitor_setting.auto_test_channel_minutes':
    defaults['monitor_setting.auto_test_channel_minutes'],
})

const normalizeFormValues = (
  values: MonitoringFormValues
): NormalizedMonitoringValues => ({
  ChannelDisableThreshold: values.ChannelDisableThreshold.trim(),
  QuotaRemindThreshold: values.QuotaRemindThreshold.trim(),
  AutomaticDisableChannelEnabled: values.AutomaticDisableChannelEnabled,
  AutomaticEnableChannelEnabled: values.AutomaticEnableChannelEnabled,
  AutomaticDisableKeywords: normalizeLineEndings(
    values.AutomaticDisableKeywords
  ),
  AutomaticDisableStatusCodes: parseHttpStatusCodeRules(
    values.AutomaticDisableStatusCodes
  ).normalized,
  ChannelHealthTransientKeywords: normalizeLineEndings(
    values.ChannelHealthTransientKeywords
  ),
  ChannelHealthTransientStatusCodes: parseHttpStatusCodeRules(
    values.ChannelHealthTransientStatusCodes
  ).normalized,
  ChannelHealthFailureThreshold: values.ChannelHealthFailureThreshold,
  ChannelHealthFailureWindowMinutes: values.ChannelHealthFailureWindowMinutes,
  ChannelHealthCooldownMinutes: values.ChannelHealthCooldownMinutes,
  ChannelHealthMaxCooldownMinutes: values.ChannelHealthMaxCooldownMinutes,
  AutomaticRetryStatusCodes: parseHttpStatusCodeRules(
    values.AutomaticRetryStatusCodes
  ).normalized,
  'monitor_setting.auto_test_channel_enabled':
    values.monitor_setting.auto_test_channel_enabled,
  'monitor_setting.auto_test_channel_minutes':
    values.monitor_setting.auto_test_channel_minutes,
})

export function MonitoringSettingsSection({
  defaultValues,
}: MonitoringSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<NormalizedMonitoringValues>(
    normalizeDefaults(defaultValues)
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<MonitoringFormInput, unknown, MonitoringFormValues>({
    resolver: zodResolver(monitoringSchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  const autoDisableStatusCodes = form.watch('AutomaticDisableStatusCodes')
  const autoRetryStatusCodes = form.watch('AutomaticRetryStatusCodes')
  const transientStatusCodes = form.watch('ChannelHealthTransientStatusCodes')
  const autoDisableParsed = useMemo(
    () => parseHttpStatusCodeRules(autoDisableStatusCodes),
    [autoDisableStatusCodes]
  )
  const autoRetryParsed = useMemo(
    () => parseHttpStatusCodeRules(autoRetryStatusCodes),
    [autoRetryStatusCodes]
  )
  const transientParsed = useMemo(
    () => parseHttpStatusCodeRules(transientStatusCodes),
    [transientStatusCodes]
  )

  const onSubmit = async (values: MonitoringFormValues) => {
    const normalized = normalizeFormValues(values)
    const updates = (
      Object.keys(normalized) as Array<keyof NormalizedMonitoringValues>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of updates) {
      const value = normalized[key]
      await updateOption.mutateAsync({
        key,
        value,
      })
    }

    baselineRef.current = normalized
  }

  return (
    <SettingsSection title={t('Monitoring & Alerts')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save monitoring rules'
          />

          <Alert className='bg-muted/40'>
            <Route className='text-muted-foreground' />
            <AlertTitle>{t('Channel failover control chain')}</AlertTitle>
            <AlertDescription className='space-y-2 text-pretty'>
              <p>
                {t(
                  'A same-model backup channel is only tried when Retry Times is at least 1 and the upstream error is retryable.'
                )}
              </p>
              <div className='flex flex-wrap gap-2'>
                <Badge variant='outline'>
                  {t('Temporary faults keep the channel enabled')}
                </Badge>
                <Badge variant='outline'>
                  {t('Hard faults can disable the channel')}
                </Badge>
                <Badge variant='outline'>
                  {t('Request trace records the routing decision')}
                </Badge>
              </div>
            </AlertDescription>
          </Alert>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='monitor_setting.auto_test_channel_enabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Scheduled channel tests')}</FormLabel>
                    <FormDescription>
                      {t('Automatically probe all channels in the background')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='monitor_setting.auto_test_channel_minutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Test interval (minutes)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('How frequently the system tests all channels')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='ChannelDisableThreshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Disable threshold (seconds)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Automatically disable channels exceeding this response time'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaRemindThreshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Quota reminder (tokens)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      step={1}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Send email alerts when a user falls below this quota')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='AutomaticDisableChannelEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Disable on failure')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Enables hard auto-disable and temporary circuit tracking for failed channels.'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <FormField
              control={form.control}
              name='AutomaticEnableChannelEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Re-enable on success')}</FormLabel>
                    <FormDescription>
                      {t('Bring channels back online after successful checks')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='AutomaticDisableKeywords'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  <span className='inline-flex items-center gap-1.5'>
                    <AlertTriangle className='text-muted-foreground size-3.5' />
                    {t('Hard failure keywords')}
                  </span>
                </FormLabel>
                <FormControl>
                  <Textarea
                    rows={6}
                    placeholder={t('one keyword per line')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'If an upstream error contains any of these keywords, it is treated as a hard fault and can auto-disable the channel.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='ChannelHealthTransientStatusCodes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Transient failure status codes')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('e.g. 408, 429, 500-599')}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Temporary status codes are retried, counted toward circuit breaking, and do not hard-disable the channel.'
                    )}{' '}
                    {transientParsed.ok &&
                      transientParsed.normalized &&
                      transientParsed.normalized !== field.value.trim() && (
                        <span className='text-muted-foreground'>
                          {t('Normalized:')} {transientParsed.normalized}
                        </span>
                      )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='ChannelHealthTransientKeywords'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Transient failure keywords')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={6}
                      placeholder={t('one keyword per line')}
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Temporary keywords are retried, counted toward circuit breaking, and do not hard-disable the channel.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-4'>
            <FormField
              control={form.control}
              name='ChannelHealthFailureThreshold'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Failure threshold')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Consecutive temporary failures before circuit breaking')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='ChannelHealthFailureWindowMinutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Failure window (minutes)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Failures counted within this rolling window')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='ChannelHealthCooldownMinutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Cooldown (minutes)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Initial skip time after the threshold is hit')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='ChannelHealthMaxCooldownMinutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Max cooldown (minutes)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Upper bound for progressive cooldown backoff')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='AutomaticDisableStatusCodes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Auto-disable status codes')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('e.g. 401, 403, 429, 500-599')}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Hard status codes can auto-disable the channel; keep 5xx and 429 in temporary faults unless the provider uses them for permanent account errors.'
                    )}{' '}
                    {autoDisableParsed.ok &&
                      autoDisableParsed.normalized &&
                      autoDisableParsed.normalized !== field.value.trim() && (
                        <span className='text-muted-foreground'>
                          {t('Normalized:')} {autoDisableParsed.normalized}
                        </span>
                      )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='AutomaticRetryStatusCodes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Auto-retry status codes')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('e.g. 401, 403, 429, 500-599')}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'These status codes are retryable for the current request. With Retry Times at least 1, Data Proxy can select a same-model backup channel.'
                    )}{' '}
                    {autoRetryParsed.ok &&
                      autoRetryParsed.normalized &&
                      autoRetryParsed.normalized !== field.value.trim() && (
                        <span className='text-muted-foreground'>
                          {t('Normalized:')} {autoRetryParsed.normalized}
                        </span>
                      )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
