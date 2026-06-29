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
const port = Number(process.env.SUBSITE_SMOKE_PORT || 4174)
const baseURL = `http://127.0.0.1:${port}`
const browserChannel = process.env.SUBSITE_SMOKE_BROWSER_CHANNEL
const chromiumUse = {
  browserName: 'chromium',
  ...(browserChannel ? { channel: browserChannel } : {}),
}

export default {
  testDir: './tests',
  outputDir: './output/playwright',
  timeout: 30_000,
  workers: 1,
  expect: {
    timeout: 5_000,
  },
  use: {
    baseURL,
    trace: 'off',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'desktop-chromium',
      use: { ...chromiumUse, viewport: { width: 1366, height: 900 } },
    },
    {
      name: 'mobile-chromium',
      use: {
        ...chromiumUse,
        viewport: { width: 393, height: 852 },
        isMobile: true,
        hasTouch: true,
        deviceScaleFactor: 2.75,
      },
    },
  ],
  webServer: {
    command: `./node_modules/.bin/rsbuild dev --host 127.0.0.1 --port ${port}`,
    url: baseURL,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
}
