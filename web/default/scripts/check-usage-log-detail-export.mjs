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

function assertMatch(source, pattern, label) {
  if (!pattern.test(source)) {
    throw new Error(`Missing ${label}: ${pattern}`)
  }
}

const detailsDialog = read(
  'src/features/usage-logs/components/dialogs/details-dialog.tsx'
)
const commonColumns = read(
  'src/features/usage-logs/components/columns/common-logs-columns.tsx'
)
const mobileCard = read(
  'src/features/usage-logs/components/usage-logs-mobile-card.tsx'
)

assertIncludes(
  detailsDialog,
  'function buildLogDetailsExportSvg',
  'SVG export builder'
)
assertIncludes(detailsDialog, "format: 'png'", 'PNG export action')
assertIncludes(detailsDialog, "format: 'svg'", 'SVG export action')
assertIncludes(detailsDialog, "title={t('Download as PNG')}", 'PNG button')
assertIncludes(detailsDialog, "title={t('Download as SVG')}", 'SVG button')
assertIncludes(detailsDialog, "type: 'image/svg+xml;charset=utf-8'", 'SVG blob')
assertIncludes(detailsDialog, "canvas.toBlob", 'PNG canvas conversion')
assertIncludes(detailsDialog, 'function getTotalTokens', 'total token helper')
assertIncludes(
  detailsDialog,
  'function formatCostPerMillionTokens',
  'cost per million token helper'
)
assertIncludes(detailsDialog, "label: t('Total Tokens')", 'total token field')
assertIncludes(
  detailsDialog,
  "label: t('Cost per 1M tokens')",
  'cost per 1M token field'
)
assertMatch(
  detailsDialog,
  /prompt_tokens[\s\S]+completion_tokens/,
  'input/output token fields'
)

assertIncludes(commonColumns, "accessorKey: 'request_id'", 'request id column')
assertIncludes(commonColumns, "t('Filter by Request ID')", 'request id filter')
assertIncludes(commonColumns, "accessorKey: 'quota'", 'usage amount column')
assertIncludes(commonColumns, "accessorKey: 'prompt_tokens'", 'token quantity column')
assertIncludes(commonColumns, "title='Tokens'", 'token quantity column header')
assertIncludes(commonColumns, 'formatTokenVolume(totalTokens)', 'cost column token summary')
assertIncludes(mobileCard, "label={t('Request ID')}", 'mobile request id row')
assertIncludes(mobileCard, "label={t('Tokens')}", 'mobile token row')

console.log('usage log detail export smoke passed')
