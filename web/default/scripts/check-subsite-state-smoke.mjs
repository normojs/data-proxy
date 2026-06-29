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
import { spawnSync } from 'node:child_process'
import { existsSync } from 'node:fs'

const playwrightBin =
  process.platform === 'win32'
    ? './node_modules/.bin/playwright.cmd'
    : './node_modules/.bin/playwright'

const args = [
  'test',
  'tests/subsite-state.smoke.spec.ts',
  '--config',
  'playwright.config.ts',
]

const result = spawnSync(playwrightBin, args, {
  stdio: 'inherit',
  env: {
    ...process.env,
    ...(process.env.SUBSITE_SMOKE_BROWSER_CHANNEL ||
    !existsSync('/Applications/Google Chrome.app')
      ? {}
      : { SUBSITE_SMOKE_BROWSER_CHANNEL: 'chrome' }),
  },
})

if (result.error) {
  console.error(result.error)
  process.exit(1)
}

process.exit(result.status ?? 1)
