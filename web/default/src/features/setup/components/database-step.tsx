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
import {
  Database,
  HardDrive,
  Loader2,
  RotateCcw,
  Save,
  Server,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'
import { StatusBadge } from '@/components/status-badge'
import {
  saveSetupRuntimeConfig,
  waitForSetupRuntimeConfigApplied,
} from '../api'
import type { SetupDatabaseType, SetupStatus } from '../types'

interface DatabaseStepProps {
  status?: SetupStatus
  onConfigSaved?: (status?: SetupStatus) => Promise<void> | void
}

const DATABASE_META: Record<
  string,
  {
    label: string
    descriptionKey: string
    variant: 'info' | 'success' | 'warning'
  }
> = {
  sqlite: {
    label: 'SQLite',
    descriptionKey:
      'SQLite stores all data in a single file. Make sure that file is persisted when running in containers.',
    variant: 'warning',
  },
  mysql: {
    label: 'MySQL',
    descriptionKey:
      'MySQL is a production-ready relational database. Keep your credentials secure.',
    variant: 'success',
  },
  postgres: {
    label: 'PostgreSQL',
    descriptionKey:
      'PostgreSQL offers advanced reliability and data integrity for production workloads.',
    variant: 'success',
  },
}

function resolveDatabaseMeta(type?: string) {
  if (!type) return null
  const normalized = type.toLowerCase()
  return (
    DATABASE_META[normalized] ?? {
      label: type,
      descriptionKey: 'Custom database driver detected.',
      variant: 'info' as const,
    }
  )
}

const SOURCE_LABEL_KEYS: Record<string, string> = {
  env: 'Environment override',
  'runtime-config': 'Setup runtime config',
  'sqlite-default': 'Temporary SQLite fallback',
}

function normalizeDatabaseType(type?: string): SetupDatabaseType {
  if (type === 'mysql' || type === 'postgres') return type
  return 'sqlite'
}

function databasePlaceholder(type: SetupDatabaseType) {
  if (type === 'postgres') {
    return 'postgresql://user:password@host.docker.internal:5432/data_proxy'
  }
  if (type === 'mysql') {
    return 'user:password@tcp(host.docker.internal:3306)/data_proxy?charset=utf8mb4&parseTime=true&loc=Local'
  }
  return ''
}

const POSTGRES_BUNDLED_DSN =
  'postgresql://root:123456@postgres:5432/data_proxy?sslmode=disable'
const POSTGRES_HOST_DSN =
  'postgresql://user:password@host.docker.internal:5432/data_proxy'
const MYSQL_HOST_DSN =
  'user:password@tcp(host.docker.internal:3306)/data_proxy?charset=utf8mb4&parseTime=true&loc=Local'
const REDIS_BUNDLED_DSN = 'redis://:123456@redis:6379/0'
const REDIS_HOST_DSN = 'redis://:password@host.docker.internal:6379/0'

export function DatabaseStep({ status, onConfigSaved }: DatabaseStepProps) {
  const { t } = useTranslation()
  const meta = resolveDatabaseMeta(status?.database_type)
  const [databaseType, setDatabaseType] = useState<SetupDatabaseType>(() =>
    normalizeDatabaseType(status?.database_type)
  )
  const [sqlDsn, setSqlDsn] = useState('')
  const [sqlitePath, setSqlitePath] = useState('')
  const [redisEnabled, setRedisEnabled] = useState(
    Boolean(status?.redis_configured || status?.redis_enabled)
  )
  const [redisConnString, setRedisConnString] = useState('')
  const [saving, setSaving] = useState(false)
  const [restarting, setRestarting] = useState(false)
  const electronApi =
    typeof window !== 'undefined'
      ? ((window as unknown as Record<string, unknown>)?.electron as
          | Record<string, unknown>
          | undefined)
      : undefined
  const isElectron = Boolean(electronApi?.isElectron)
  const electronDataDir = electronApi?.dataDir as string | undefined
  const databaseSource = status?.database_source ?? 'sqlite-default'
  const redisSource = status?.redis_source || 'not-configured'
  const restartRequired = Boolean(status?.runtime_config_restart_required)
  const databaseSourceLabel = t(
    SOURCE_LABEL_KEYS[databaseSource] ?? databaseSource
  )
  const redisSourceLabel = status?.redis_configured
    ? t(SOURCE_LABEL_KEYS[redisSource] ?? redisSource)
    : t('Not configured')

  const canSave = useMemo(() => {
    if (saving || restarting) return false
    if (databaseType !== 'sqlite' && sqlDsn.trim().length === 0) return false
    if (redisEnabled && redisConnString.trim().length === 0) return false
    return true
  }, [databaseType, redisConnString, redisEnabled, restarting, saving, sqlDsn])

  const handleSaveRuntimeConfig = async () => {
    setSaving(true)
    try {
      const response = await saveSetupRuntimeConfig({
        database_type: databaseType,
        sql_dsn: databaseType === 'sqlite' ? undefined : sqlDsn.trim(),
        sqlite_path:
          databaseType === 'sqlite' && sqlitePath.trim()
            ? sqlitePath.trim()
            : undefined,
        redis_enabled: redisEnabled,
        redis_conn_string: redisEnabled ? redisConnString.trim() : undefined,
      })

      if (!response.success) {
        toast.error(response.message || t('Failed to save runtime config'))
        return
      }

      if (response.data?.restart_scheduled) {
        toast.success(
          response.message ||
            t('Runtime config saved. Data Proxy is restarting automatically.')
        )
        setRestarting(true)
        try {
          const setupResponse = await waitForSetupRuntimeConfigApplied()
          toast.success(t('Data Proxy restarted. You can continue setup.'))
          await onConfigSaved?.(setupResponse.data)
        } catch {
          toast.error(
            t('Data Proxy is still restarting. Refresh this page in a moment.')
          )
          await onConfigSaved?.()
        } finally {
          setRestarting(false)
        }
        return
      }

      toast.success(
        response.message ||
          t('Runtime config saved. Restart Data Proxy to apply it.')
      )
      await onConfigSaved?.()
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className='space-y-4'>
      <div className='bg-card flex flex-col gap-4 rounded-lg border p-4 sm:flex-row sm:items-center sm:justify-between'>
        <div className='space-y-1'>
          <p className='text-muted-foreground text-sm font-medium'>
            {t('Detected database')}
          </p>
          <p className='text-foreground text-base font-semibold'>
            {meta?.label ?? t('Unknown')}
          </p>
          <p className='text-muted-foreground text-sm'>
            {t(
              meta?.descriptionKey ??
                'The setup wizard will use this database during initialization.'
            )}
          </p>
          <p className='text-muted-foreground text-xs'>
            {t('Source:')} {databaseSourceLabel}
          </p>
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <StatusBadge
            label={meta?.label ?? t('Unknown')}
            variant={meta?.variant ?? 'info'}
            className='cursor-default'
            copyable={false}
            icon={Database}
          />
          <StatusBadge
            label={
              status?.redis_enabled
                ? t('Redis enabled')
                : status?.redis_configured
                  ? t('Redis configured')
                  : t('Redis optional')
            }
            variant={status?.redis_enabled ? 'success' : 'neutral'}
            className='cursor-default'
            copyable={false}
            icon={Server}
          />
        </div>
      </div>

      <div className='bg-card space-y-4 rounded-lg border p-4'>
        <div className='space-y-1'>
          <h3 className='text-sm font-semibold'>
            {t('Workspace dependency configuration')}
          </h3>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Configure the database and optional Redis service here before creating the first administrator account. Data Proxy will test the connection, save a local runtime config, and use it after restart.'
            )}
          </p>
        </div>

        <div className='grid gap-4 md:grid-cols-[180px_1fr]'>
          <div className='space-y-2'>
            <Label htmlFor='setup-database-type'>{t('Database type')}</Label>
            <NativeSelect
              id='setup-database-type'
              className='w-full'
              value={databaseType}
              onChange={(event) =>
                setDatabaseType(event.target.value as SetupDatabaseType)
              }
            >
              <NativeSelectOption value='sqlite'>SQLite</NativeSelectOption>
              <NativeSelectOption value='mysql'>MySQL</NativeSelectOption>
              <NativeSelectOption value='postgres'>
                PostgreSQL
              </NativeSelectOption>
            </NativeSelect>
          </div>

          {databaseType === 'sqlite' ? (
            <div className='space-y-2'>
              <Label htmlFor='setup-sqlite-path'>{t('SQLite file path')}</Label>
              <Input
                id='setup-sqlite-path'
                value={sqlitePath}
                onChange={(event) => setSqlitePath(event.target.value)}
                placeholder='/data/data-proxy.db'
              />
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Leave empty to use the built-in default path. For containers, make sure the data directory is persisted.'
                )}
              </p>
            </div>
          ) : (
            <div className='space-y-2'>
              <Label htmlFor='setup-sql-dsn'>
                {databaseType === 'postgres'
                  ? t('PostgreSQL connection string')
                  : t('MySQL connection string')}
              </Label>
              <Input
                id='setup-sql-dsn'
                value={sqlDsn}
                onChange={(event) => setSqlDsn(event.target.value)}
                placeholder={databasePlaceholder(databaseType)}
              />
              <div className='flex flex-wrap gap-2'>
                {databaseType === 'postgres' && (
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() => setSqlDsn(POSTGRES_BUNDLED_DSN)}
                  >
                    <Server className='size-3.5' />
                    {t('Use bundled PostgreSQL')}
                  </Button>
                )}
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() =>
                    setSqlDsn(
                      databaseType === 'postgres'
                        ? POSTGRES_HOST_DSN
                        : MYSQL_HOST_DSN
                    )
                  }
                >
                  <Server className='size-3.5' />
                  {databaseType === 'postgres'
                    ? t('Use local PostgreSQL')
                    : t('Use local MySQL')}
                </Button>
              </div>
              <ul className='text-muted-foreground list-disc space-y-1 pl-4 text-xs'>
                {databaseType === 'postgres' && (
                  <li>
                    {t(
                      'Bundled PostgreSQL runs only when the local-deps Compose profile is enabled. Use postgres as the host; it does not publish port 5432 to this Mac, so it will not conflict with a local PostgreSQL service.'
                    )}
                  </li>
                )}
                <li>
                  {databaseType === 'postgres'
                    ? t(
                        'Local PostgreSQL means an existing database outside this Compose stack. Use host.docker.internal only when Data Proxy runs in Docker and PostgreSQL runs on this Mac; use 127.0.0.1 when Data Proxy also runs directly on this Mac; use a network IP or domain for another machine.'
                      )
                    : t(
                        'Local MySQL means an existing database outside this Compose stack. Use host.docker.internal only when Data Proxy runs in Docker and MySQL runs on this Mac; use 127.0.0.1 when Data Proxy also runs directly on this Mac; use a network IP or domain for another machine.'
                      )}
                </li>
              </ul>
            </div>
          )}
        </div>

        <div className='rounded-lg border p-3'>
          <div className='flex items-start justify-between gap-3'>
            <div className='space-y-1'>
              <Label htmlFor='setup-redis-enabled'>{t('Redis cache')}</Label>
              <p className='text-muted-foreground text-xs'>
                {t('Current Redis source:')} {redisSourceLabel}
              </p>
            </div>
            <Switch
              id='setup-redis-enabled'
              checked={redisEnabled}
              onCheckedChange={setRedisEnabled}
            />
          </div>
          {redisEnabled && (
            <div className='mt-3 space-y-2'>
              <Label htmlFor='setup-redis-dsn'>
                {t('Redis connection string')}
              </Label>
              <Input
                id='setup-redis-dsn'
                value={redisConnString}
                onChange={(event) => setRedisConnString(event.target.value)}
                placeholder={REDIS_HOST_DSN}
              />
              <div className='flex flex-wrap gap-2'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => setRedisConnString(REDIS_BUNDLED_DSN)}
                >
                  <Server className='size-3.5' />
                  {t('Use bundled Redis')}
                </Button>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => setRedisConnString(REDIS_HOST_DSN)}
                >
                  <Server className='size-3.5' />
                  {t('Use local Redis')}
                </Button>
              </div>
              <ul className='text-muted-foreground list-disc space-y-1 pl-4 text-xs'>
                <li>
                  {t(
                    'Bundled Redis runs only when the local-deps Compose profile is enabled. Use redis as the host; it does not publish port 6379 to this Mac, so it will not conflict with a local Redis service.'
                  )}
                </li>
                <li>
                  {t(
                    'Local Redis means an existing Redis service outside this Compose stack. Use host.docker.internal only when Data Proxy runs in Docker and Redis runs on this Mac; use 127.0.0.1 when Data Proxy also runs directly on this Mac; use a network IP or domain for another machine.'
                  )}
                </li>
              </ul>
            </div>
          )}
        </div>

        <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Secrets are saved on the server only and are never returned to this page after saving.'
            )}
          </p>
          <Button
            type='button'
            onClick={handleSaveRuntimeConfig}
            disabled={!canSave}
          >
            {restarting ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <Save className='size-4' />
            )}
            {restarting
              ? t('Restarting…')
              : saving
                ? t('Testing…')
                : t('Test and save')}
          </Button>
        </div>
      </div>

      {restarting ? (
        <Alert className='border-sky-200 bg-sky-50 dark:border-sky-900/60 dark:bg-sky-950/40'>
          <AlertTitle className='flex items-center gap-2'>
            <Loader2 className='size-4 animate-spin text-sky-500' />
            {t('Restarting Data Proxy')}
          </AlertTitle>
          <AlertDescription>
            {t(
              'Data Proxy is restarting to apply the saved database and Redis settings. This page will continue automatically when the service is back.'
            )}
          </AlertDescription>
        </Alert>
      ) : restartRequired ? (
        <Alert className='border-sky-200 bg-sky-50 dark:border-sky-900/60 dark:bg-sky-950/40'>
          <AlertTitle className='flex items-center gap-2'>
            <RotateCcw className='size-4 text-sky-500' />
            {t('Restart required')}
          </AlertTitle>
          <AlertDescription>
            {t(
              'Runtime config has been saved. Restart Data Proxy so the server can initialize the selected database and Redis before you create the first administrator account.'
            )}
          </AlertDescription>
        </Alert>
      ) : null}

      {status?.database_type === 'sqlite' && (
        <Alert className='border-amber-200 bg-amber-50 dark:border-amber-900/60 dark:bg-amber-950/40'>
          <AlertTitle className='flex items-center gap-2'>
            <HardDrive className='size-4 text-amber-500' />
            {t('Persist your data file')}
          </AlertTitle>
          <AlertDescription>
            <p>
              {t(
                'When running in containers or ephemeral environments, ensure the SQLite file is mapped to persistent storage to avoid data loss on restart.'
              )}
            </p>
            {isElectron && electronDataDir && (
              <p className='mt-3 rounded-md bg-amber-100/70 px-3 py-2 font-mono text-xs text-amber-800 dark:bg-amber-900/30 dark:text-amber-200'>
                {t('Data directory:')} {electronDataDir}
              </p>
            )}
            {isElectron && !electronDataDir && (
              <p className='text-muted-foreground mt-3 text-xs'>
                {t(
                  'Data is stored locally on this device. Use system backups to keep a safe copy.'
                )}
              </p>
            )}
          </AlertDescription>
        </Alert>
      )}

      {status?.database_type === 'mysql' && (
        <Alert className='border-emerald-200 bg-emerald-50 dark:border-emerald-900/60 dark:bg-emerald-950/40'>
          <AlertTitle className='flex items-center gap-2'>
            <Server className='size-4 text-emerald-500' />
            {t('MySQL detected')}
          </AlertTitle>
          <AlertDescription>
            {t(
              'MySQL is production ready. Ensure automated backups and a dedicated user with the minimal required privileges are configured.'
            )}
          </AlertDescription>
        </Alert>
      )}

      {status?.database_type === 'postgres' && (
        <Alert className='border-sky-200 bg-sky-50 dark:border-sky-900/60 dark:bg-sky-950/40'>
          <AlertTitle className='flex items-center gap-2'>
            <Server className='size-4 text-sky-500' />
            {t('PostgreSQL detected')}
          </AlertTitle>
          <AlertDescription>
            {t(
              'PostgreSQL offers strong reliability guarantees. Double check your maintenance window and retention policies before going live.'
            )}
          </AlertDescription>
        </Alert>
      )}
    </div>
  )
}
