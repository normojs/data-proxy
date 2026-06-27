#!/usr/bin/env node
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..')

function read(relativePath) {
  return readFileSync(resolve(root, relativePath), 'utf8')
}

function assertIncludes(source, needle, label) {
  if (!source.includes(needle)) {
    throw new Error(`Missing ${label}: ${needle}`)
  }
}

function assertNotIncludes(source, needle, label) {
  if (source.includes(needle)) {
    throw new Error(`Unexpected ${label}: ${needle}`)
  }
}

const systemBehavior = read(
  'src/features/system-settings/general/system-behavior-section.tsx'
)
const monitoringSettings = read(
  'src/features/system-settings/integrations/monitoring-settings-section.tsx'
)
const operationsRegistry = read(
  'src/features/system-settings/operations/section-registry.tsx'
)
const channelRowActions = read(
  'src/features/channels/components/data-table-row-actions.tsx'
)

assertNotIncludes(
  systemBehavior,
  "name='RetryTimes'",
  'RetryTimes field in System Behavior section'
)
assertIncludes(
  monitoringSettings,
  "name='RetryTimes'",
  'RetryTimes field in Monitoring section'
)
assertIncludes(
  monitoringSettings,
  'RetryTimes: z.coerce.number().int().min(0).max(10)',
  'RetryTimes validation'
)
assertIncludes(
  operationsRegistry,
  'RetryTimes: settings.RetryTimes',
  'RetryTimes passed to Monitoring section'
)

for (const label of [
  'Channel failover control chain',
  'Retry Times',
  'Transient failure status codes',
  'Transient failure keywords',
  'Auto-disable status codes',
  'Hard failure keywords',
  'Failure threshold',
  'Failure window (minutes)',
  'Cooldown (minutes)',
  'Max cooldown (minutes)',
]) {
  assertIncludes(monitoringSettings, label, `${label} setting label`)
}

assertIncludes(
  monitoringSettings,
  'Number of extra attempts after the first channel fails. Set at least 1 to allow same-model backup failover.',
  'RetryTimes failover description'
)
assertIncludes(
  channelRowActions,
  'Clear temporary circuit',
  'manual temporary circuit recovery action'
)

console.log('channel failover settings smoke passed')
