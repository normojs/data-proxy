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
  'async function exportDialogSnapshot',
  'dialog snapshot export helper'
)
assertIncludes(
  detailsDialog,
  'function createExportableDialogNode',
  'exportable dialog clone builder'
)
assertMatch(
  detailsDialog,
  /exportDialogSnapshot\([\s\S]+,\s*'png'\s*\)/,
  'PNG export action'
)
assertMatch(
  detailsDialog,
  /exportDialogSnapshot\([\s\S]+,\s*'svg'\s*\)/,
  'SVG export action'
)
assertIncludes(detailsDialog, "title={t('Download as PNG')}", 'PNG button')
assertIncludes(detailsDialog, "title={t('Download as SVG')}", 'SVG button')
assertIncludes(detailsDialog, 'data-export-exclude', 'export controls exclusion')
assertIncludes(detailsDialog, 'showCloseButton={false}', 'custom dialog header controls')
assertIncludes(detailsDialog, "type: 'image/svg+xml;charset=utf-8'", 'SVG blob')
assertIncludes(detailsDialog, "canvas.toBlob", 'PNG canvas conversion')
assertIncludes(detailsDialog, 'function getTotalTokens', 'total token helper')
assertIncludes(
  detailsDialog,
  'function formatCostPerTokenVolume',
  'token unit cost helper'
)
assertIncludes(detailsDialog, "label: t('Total Tokens')", 'total token field')
assertIncludes(
  detailsDialog,
  "label: t('Token Unit Cost')",
  'token unit cost field'
)
assertIncludes(detailsDialog, '100M:', '100M token unit cost value')
assertIncludes(detailsDialog, "t('User Information')", 'user info tab')
assertIncludes(detailsDialog, "t('Admin Information')", 'admin info tab')
assertIncludes(
  detailsDialog,
  'const isUserView = !props.isAdmin || activeTab ===',
  'normal user-only view gate'
)
assertIncludes(
  detailsDialog,
  'const isAdminView = props.isAdmin && activeTab ===',
  'admin-only view gate'
)
assertMatch(
  detailsDialog,
  /prompt_tokens[\s\S]+completion_tokens/,
  'input/output token fields'
)

assertIncludes(commonColumns, "accessorKey: 'request_id'", 'request id column')
assertIncludes(
  read('src/features/usage-logs/components/columns/request-id-cell.tsx'),
  "t('Filter by Request ID')",
  'request id filter'
)
assertIncludes(commonColumns, "accessorKey: 'quota'", 'usage amount column')
assertIncludes(commonColumns, "accessorKey: 'prompt_tokens'", 'token quantity column')
assertIncludes(commonColumns, "title='Tokens'", 'token quantity column header')
assertIncludes(commonColumns, 'formatTokenVolume(totalTokens)', 'cost column token summary')
assertIncludes(mobileCard, "label={t('Request ID')}", 'mobile request id row')
assertIncludes(mobileCard, "label={t('Tokens')}", 'mobile token row')

console.log('usage log detail export smoke passed')
