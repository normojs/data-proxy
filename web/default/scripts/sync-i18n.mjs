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
import fs from 'node:fs/promises'
import path from 'node:path'

// This script is executed from the web/ package root (see package.json script).
const SOURCE_DIR = path.resolve('src')
const LOCALES_DIR = path.resolve('src/i18n/locales')
const STATIC_KEYS_FILE = path.resolve('src/i18n/static-keys.ts')
const SOURCE_LOCALE = 'en'
const FALLBACK_COMPARE_LOCALE = 'en' // used for "still English" detection only
const SOURCE_EXTENSIONS = new Set(['.js', '.jsx', '.ts', '.tsx'])
const SOURCE_SCAN_IGNORED_DIRS = new Set(['.git', 'build', 'dist', 'node_modules'])
const OBFUSCATED_KEYS = [
  {
    runtime: ['footer', 'new' + 'api', 'projectAttributionSuffix'].join('.'),
    serialized: 'footer.new\\u0061pi.projectAttributionSuffix',
  },
]

const BRAND_AND_LITERAL_KEYS = new Set([
  'AI Proxy',
  'AIGC2D',
  'Alipay',
  'Anthropic',
  'API Key ID',
  'API URL',
  'API2GPT',
  'AccessKey / SecretAccessKey',
  'AZURE_OPENAI_ENDPOINT *',
  'Baidu V2',
  'Chat Completions',
  'ChatGPT',
  'Claude',
  'Client ID',
  'Client Secret',
  'Cloudflare',
  'Cohere',
  'Bridge',
  'Bridge WebSocket',
  'Data Proxy',
  'Data Proxy &lt;noreply@example.com&gt;',
  'DeepSeek',
  'Discord',
  'DoubaoVideo',
  'FastGPT',
  'Favicon URL',
  'Gemini',
  'Gemini Image 4K',
  'GitHub',
  'H 站 OAuth Client Secret',
  'Jimeng',
  'JustSong',
  'LingYiWanWu',
  'LinuxDO',
  'Midjourney',
  'MidjourneyPlus',
  'Midjourney-Proxy',
  'MiniMax',
  'MiniMax reasoning_split',
  'Mistral',
  'MokaAI',
  'Moonshot',
  'New API',
  'New API &lt;noreply@example.com&gt;',
  'NewAPI',
  'OAuth Client Secret',
  'OhMyGPT',
  'Ollama',
  'One API',
  'OpenAI',
  'OpenAI reasoning_effort',
  'OpenAIMax',
  'OpenAPI',
  'OpenAPI URL',
  'OpenRouter',
  'OpenRouter reasoning.effort',
  'Pancake',
  'Passkey',
  'Perplexity',
  'QuantumNous',
  'QuantumNous/new-api',
  'Qwen enable_thinking',
  'Qidian Browser',
  'QidianBrowser Bridge',
  'Quota:',
  'Replicate',
  'Responses',
  'SDK & OpenAPI',
  'SiliconFlow',
  'Setup Token',
  'Stripe',
  'Streamable HTTP',
  'Submodel',
  'SunoAPI',
  'Telegram',
  'Tencent',
  'TTFT P50',
  'TTFT P95',
  'TTFT P99',
  'Uptime Kuma',
  'Uptime Kuma URL',
  'Vertex AI',
  'VolcEngine',
  'Waffo Pancake Dashboard',
  'Waffo Pancake MoR',
  'WeChat',
  'WeChat Pay',
  'Webhook',
  'Webhook URL',
  'Webhook URL:',
  'Well-Known URL',
  'Windows',
  'Worker URL',
  'Xinference',
  'Xunfei',
  'Zhipu V4',
  '"default": "us-central1", "claude-3-5-sonnet-20240620": "europe-west1"',
  'edit_this',
  'footer.columns.related.links.midjourney',
  'footer.columns.related.links.newApiKeyTool',
  'my-status',
  'new-api-key-tool',
  'price_xxx',
  'whsec_xxx',
])

function isPlainObject(v) {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

function stableStringify(obj) {
  let text = JSON.stringify(obj, null, 2)
  for (const key of OBFUSCATED_KEYS) {
    text = text.replaceAll(`"${key.runtime}":`, `"${key.serialized}":`)
  }
  return text + '\n'
}

function duplicateTranslationKeys(raw) {
  const duplicates = []
  const translationIndex = raw.indexOf('"translation"')
  if (translationIndex < 0) return duplicates

  const start = raw.indexOf('{', translationIndex)
  if (start < 0) return duplicates

  let depth = 0
  let inString = false
  let escaped = false
  let end = -1
  for (let i = start; i < raw.length; i++) {
    const ch = raw[i]
    if (inString) {
      if (escaped) {
        escaped = false
      } else if (ch === '\\') {
        escaped = true
      } else if (ch === '"') {
        inString = false
      }
      continue
    }
    if (ch === '"') {
      inString = true
    } else if (ch === '{') {
      depth += 1
    } else if (ch === '}') {
      depth -= 1
      if (depth === 0) {
        end = i
        break
      }
    }
  }
  if (end < 0) return duplicates

  const block = raw.slice(start + 1, end)
  const entryRegex = /^\s*"((?:\\.|[^"])*)"\s*:\s*(?:"((?:\\.|[^"])*)"|[^,\n}]*)/gm
  const seen = new Map()
  let match
  while ((match = entryRegex.exec(block))) {
    const key = JSON.parse(`"${match[1]}"`)
    const value = match[2] === undefined ? '' : JSON.parse(`"${match[2]}"`)
    const line = raw.slice(0, start + 1 + match.index).split('\n').length
    if (seen.has(key)) {
      const first = seen.get(key)
      duplicates.push({
        key,
        firstLine: first.line,
        duplicateLine: line,
        conflict: first.value !== value,
      })
    } else {
      seen.set(key, { line, value })
    }
  }

  return duplicates
}

function countLeafKeys(obj) {
  if (Array.isArray(obj)) return obj.length
  if (!isPlainObject(obj)) return 0
  let count = 0
  for (const k of Object.keys(obj)) {
    const v = obj[k]
    if (isPlainObject(v) || Array.isArray(v)) count += countLeafKeys(v)
    else count += 1
  }
  return count
}

function cloneJSON(obj) {
  return JSON.parse(JSON.stringify(obj))
}

async function listSourceFiles(dir) {
  const files = []
  let entries = []
  try {
    entries = await fs.readdir(dir, { withFileTypes: true })
  } catch {
    return files
  }

  for (const entry of entries) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      if (!SOURCE_SCAN_IGNORED_DIRS.has(entry.name)) {
        files.push(...(await listSourceFiles(full)))
      }
      continue
    }
    if (entry.isFile() && SOURCE_EXTENSIONS.has(path.extname(entry.name))) {
      files.push(full)
    }
  }

  return files.sort((a, b) => a.localeCompare(b))
}

function decodeSourceStringLiteral(quote, raw) {
  try {
    return Function(`"use strict"; return ${quote}${raw}${quote}`)()
  } catch {
    return raw
  }
}

function addRuntimeKey(keys, key) {
  if (typeof key !== 'string') return
  if (key.length === 0) return
  keys.add(key)
}

function collectTCallKeys(raw, keys) {
  const tCallRegex = /\b(?:t|i18n\.t)\(\s*(['"`])((?:\\.|(?!\1)[\s\S])*?)\1/g
  let match
  while ((match = tCallRegex.exec(raw))) {
    const [, quote, rawKey] = match
    if (quote === '`' && rawKey.includes('${')) continue
    addRuntimeKey(keys, decodeSourceStringLiteral(quote, rawKey))
  }
}

function collectStaticKeys(raw, keys) {
  const staticKeysMatch = raw.match(/STATIC_I18N_KEYS\s*=\s*\[([\s\S]*?)\]\s+as\s+const/)
  if (!staticKeysMatch) return

  const stringRegex = /(['"`])((?:\\.|(?!\1)[\s\S])*?)\1/g
  let match
  while ((match = stringRegex.exec(staticKeysMatch[1]))) {
    const [, quote, rawKey] = match
    if (quote === '`' && rawKey.includes('${')) continue
    addRuntimeKey(keys, decodeSourceStringLiteral(quote, rawKey))
  }
}

async function collectRuntimeI18nKeys() {
  const keys = new Set()
  for (const file of await listSourceFiles(SOURCE_DIR)) {
    const raw = await fs.readFile(file, 'utf8')
    collectTCallKeys(raw, keys)
  }

  const staticKeysRaw = await fs.readFile(STATIC_KEYS_FILE, 'utf8').catch(() => '')
  collectStaticKeys(staticKeysRaw, keys)

  return keys
}

function buildBaseJson(parsedByLocale, sourceLocale, runtimeI18nKeys) {
  const sourceJson = parsedByLocale[sourceLocale]
  if (!sourceJson) return null

  const baseJson = cloneJSON(sourceJson)
  const baseTrans = isPlainObject(baseJson.translation) ? baseJson.translation : {}
  baseJson.translation = baseTrans

  const sourceMissingRuntimeKeys = []
  for (const key of [...runtimeI18nKeys].sort((a, b) => a.localeCompare(b))) {
    if (!Object.prototype.hasOwnProperty.call(baseTrans, key)) {
      sourceMissingRuntimeKeys.push(key)
      baseTrans[key] = key
    }
  }

  const sourceKeys = new Set(Object.keys(baseTrans))
  const extraKeys = new Set()
  for (const locale of Object.keys(parsedByLocale).sort((a, b) => a.localeCompare(b))) {
    const trans = parsedByLocale[locale]?.translation
    if (!isPlainObject(trans)) continue
    for (const key of Object.keys(trans)) {
      if (!sourceKeys.has(key)) extraKeys.add(key)
    }
  }

  for (const key of [...extraKeys].sort((a, b) => a.localeCompare(b))) {
    baseTrans[key] = key
  }

  return {
    baseJson,
    sourceMissingRuntimeKeys,
  }
}

function reorderLikeBase(base, target, fill, extras, missing, currentPath = []) {
  // If base is an object, we keep base's key order and recurse.
  if (isPlainObject(base)) {
    const out = {}
    const t = isPlainObject(target) ? target : {}
    const f = isPlainObject(fill) ? fill : {}

    for (const key of Object.keys(base)) {
      const nextPath = [...currentPath, key]
      if (Object.prototype.hasOwnProperty.call(t, key)) {
        out[key] = reorderLikeBase(base[key], t[key], f[key], extras, missing, nextPath)
      } else {
        missing.push(nextPath.join('.'))
        out[key] = reorderLikeBase(base[key], undefined, f[key], extras, missing, nextPath)
      }
    }

    for (const key of Object.keys(t)) {
      if (!Object.prototype.hasOwnProperty.call(base, key)) {
        const nextPath = [...currentPath, key].join('.')
        extras[nextPath] = t[key]
      }
    }

    return out
  }

  // For arrays: prefer target if it's also an array; otherwise use base.
  if (Array.isArray(base)) {
    if (Array.isArray(target)) return target
    if (Array.isArray(fill)) return fill
    return base
  }

  // For primitives: prefer target if defined, else base.
  return target === undefined ? (fill ?? base) : target
}

function isLikelyUntranslated({ locale, baseValue, value }) {
  if (typeof value !== 'string' || typeof baseValue !== 'string') return false
  if (value !== baseValue) return false

  // Skip short tokens / acronyms / ids
  const s = baseValue.trim()
  if (BRAND_AND_LITERAL_KEYS.has(s)) return false
  if (
    /^https?:\/\//.test(s) ||
    /^\/[\w/-]+/.test(s) ||
    /^[\w.-]+@[\w.-]+(?:\s*,\s*[\w.-]+@[\w.-]+)*$/.test(s) ||
    /^[\w.-]+(?:,\s*[\w.-]+)+$/.test(s) ||
    /^smtp\./i.test(s) ||
    /^socks5:/i.test(s) ||
    /^org-/.test(s) ||
    /^gpt-/i.test(s) ||
    /^checkout\./.test(s) ||
    /^footer\./.test(s) ||
    /^[A-Z0-9_ *./:-]+$/.test(s) ||
    s.startsWith('{') ||
    s.startsWith('[') ||
    s.includes('&#10;')
  ) {
    return false
  }
  if (s.length < 6) return false
  if (!/[A-Za-z]{3,}/.test(s)) return false

  // For locales with non-latin scripts, equality with EN is a strong signal.
  if (locale === 'ja' || locale === 'zh') return true
  if (locale === 'ru') return true

  // For fr/vi: still useful but noisier; keep it conservative.
  if (locale === 'fr' || locale === 'vi') return /\b(the|and|or|to|with|please)\b/i.test(s)

  return false
}

async function main() {
  const entries = await fs.readdir(LOCALES_DIR, { withFileTypes: true })
  const localeFiles = entries
    .filter((e) => e.isFile() && e.name.endsWith('.json'))
    .map((e) => e.name)
    .sort((a, b) => a.localeCompare(b))

  const parsedByLocale = {}
  const duplicateKeysByLocale = {}
  const runtimeI18nKeys = await collectRuntimeI18nKeys()
  for (const filename of localeFiles) {
    const locale = filename.replace(/\.json$/i, '')
    const raw = await fs.readFile(path.join(LOCALES_DIR, filename), 'utf8')
    parsedByLocale[locale] = JSON.parse(raw)
    duplicateKeysByLocale[locale] = duplicateTranslationKeys(raw)
  }

  const fallbackBaseLocale = Object.keys(parsedByLocale)
    .map((locale) => {
      const json = parsedByLocale[locale]
      const trans = json?.translation ?? {}
      return { locale, score: countLeafKeys(trans) }
    })
    .sort((a, b) => b.score - a.score || a.locale.localeCompare(b.locale))[0]?.locale
  const baseLocale = parsedByLocale[SOURCE_LOCALE] ? SOURCE_LOCALE : fallbackBaseLocale

  if (!baseLocale) throw new Error('No locale files found.')

  const baseFile = `${baseLocale}.json`
  const baseBuild = buildBaseJson(parsedByLocale, baseLocale, runtimeI18nKeys)
  if (!baseBuild) throw new Error(`Base locale ${baseLocale} is not available.`)
  const { baseJson, sourceMissingRuntimeKeys } = baseBuild

  const compareJson = parsedByLocale[FALLBACK_COMPARE_LOCALE] ?? baseJson

  const report = {
    base: baseFile,
    sourceLocale: baseLocale,
    baseStrategy: 'source_locale_with_source_scan_and_locale_key_union',
    runtimeKeyCount: runtimeI18nKeys.size,
    sourceMissingRuntimeKeyCount: sourceMissingRuntimeKeys.length,
    locales: {},
  }

  const extrasDir = path.join(LOCALES_DIR, '_extras')
  const reportsDir = path.join(LOCALES_DIR, '_reports')
  await fs.mkdir(extrasDir, { recursive: true })
  await fs.mkdir(reportsDir, { recursive: true })

  for (const filename of localeFiles) {
    const locale = filename.replace(/\.json$/i, '')
    const full = path.join(LOCALES_DIR, filename)
    const json = parsedByLocale[locale]

    const extras = {}
    const missing = []
    const fixed = reorderLikeBase(baseJson, json, compareJson, extras, missing)
    const duplicateKeys = duplicateKeysByLocale[locale] ?? []

    // Untranslated scan (translation namespace only)
    const untranslated = {}
    const compareTrans = compareJson?.translation ?? {}
    const trans = fixed?.translation ?? {}
    if (
      isPlainObject(compareTrans) &&
      isPlainObject(trans) &&
      locale !== FALLBACK_COMPARE_LOCALE &&
      locale !== baseLocale
    ) {
      for (const k of Object.keys(compareTrans)) {
        const baseValue = compareTrans[k]
        const value = trans[k]
        if (isLikelyUntranslated({ locale, baseValue, value })) {
          untranslated[k] = value
        }
      }
    }

    report.locales[locale] = {
      file: filename,
      missingCount: missing.length,
      extrasCount: Object.keys(extras).length,
      untranslatedCount: Object.keys(untranslated).length,
      duplicateKeyCount: duplicateKeys.length,
      duplicateConflictCount: duplicateKeys.filter((item) => item.conflict).length,
    }

    if (Object.keys(extras).length > 0) {
      await fs.writeFile(path.join(extrasDir, `${locale}.extras.json`), stableStringify(extras), 'utf8')
    } else {
      await fs.rm(path.join(extrasDir, `${locale}.extras.json`), { force: true })
    }
    if (Object.keys(untranslated).length > 0) {
      await fs.writeFile(
        path.join(reportsDir, `${locale}.untranslated.json`),
        stableStringify(untranslated),
        'utf8',
      )
    } else {
      await fs.rm(path.join(reportsDir, `${locale}.untranslated.json`), { force: true })
    }
    if (duplicateKeys.length > 0) {
      await fs.writeFile(
        path.join(reportsDir, `${locale}.duplicates.json`),
        stableStringify(duplicateKeys),
        'utf8',
      )
    } else {
      await fs.rm(path.join(reportsDir, `${locale}.duplicates.json`), { force: true })
    }

    // Rewrite locale file in base order (even for en to normalize formatting)
    await fs.writeFile(full, stableStringify(fixed), 'utf8')
  }

  await fs.writeFile(path.join(reportsDir, '_sync-report.json'), stableStringify(report), 'utf8')

  console.log(`i18n sync done. Report: ${path.join(reportsDir, '_sync-report.json')}`)
}

main().catch((err) => {

  console.error(err)
  process.exitCode = 1
})
