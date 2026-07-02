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
import { expect, test, type Page } from '@playwright/test'

type PublicSubsiteState = {
  slug: string
  name: string
  title: string
  runtimeStatus: 'disabled' | 'expired' | 'enabled'
  accessCode: string
  accessMessage: string
  disabledReason?: string
  startsAt?: number
  endsAt?: number
}

type CommonMockOptions = {
  quotaExceeded?: boolean
}

const now = Math.floor(Date.now() / 1000)

function quotaMetric(limit = 100000): {
  limit: number
  used: number
  remaining: number
  window_start: number
  window_end: number
  next_reset_time: number
  window_seconds: number
} {
  return {
    limit,
    used: 0,
    remaining: limit,
    window_start: now - 3600,
    window_end: now + 3600,
    next_reset_time: now + 3600,
    window_seconds: 3600,
  }
}

const publicSubsites: Record<string, PublicSubsiteState> = {
  'logout-smoke': {
    slug: 'logout-smoke',
    name: 'Logout Smoke Site',
    title: 'Logout Smoke Site',
    runtimeStatus: 'enabled',
    accessCode: '',
    accessMessage: '',
    startsAt: now - 3600,
    endsAt: now + 86400,
  },
  'closed-smoke': {
    slug: 'closed-smoke',
    name: 'Closed Smoke Site',
    title: 'Closed Smoke Site',
    runtimeStatus: 'disabled',
    accessCode: 'subsite_disabled',
    accessMessage: 'Subsite is currently disabled',
    disabledReason: 'Maintenance window in progress.',
    startsAt: now - 3600,
    endsAt: now + 86400,
  },
  'expired-smoke': {
    slug: 'expired-smoke',
    name: 'Expired Smoke Site',
    title: 'Expired Smoke Site',
    runtimeStatus: 'expired',
    accessCode: 'subsite_expired',
    accessMessage: 'Subsite access window has ended',
    startsAt: now - 86400 * 2,
    endsAt: now - 3600,
  },
}

async function installCommonMocks(page: Page, options: CommonMockOptions = {}) {
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname

    if (path === '/api/status') {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            system_name: 'Data Proxy Smoke',
            logo: '',
            footer_html: '',
          },
        }),
      })
      return
    }

    if (path === '/api/user/logout') {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          message: '',
        }),
      })
      return
    }

    const publicMatch = path.match(/^\/api\/subsites\/([^/]+)\/public$/)

    if (publicMatch) {
      const slug = decodeURIComponent(publicMatch[1])
      const subsite = publicSubsites[slug]

      if (!subsite) {
        await route.fulfill({
          status: 404,
          contentType: 'application/json',
          body: JSON.stringify({
            success: false,
            message: 'subsite_not_found',
          }),
        })
        return
      }

      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            id: 100,
            slug: subsite.slug,
            name: subsite.name,
            title: subsite.title,
            logo_url: '',
            favicon_url: '',
            theme_color: '#2563eb',
            status: subsite.runtimeStatus,
            runtime_status: subsite.runtimeStatus,
            disabled_reason: subsite.disabledReason,
            registration_policy: 'open',
            starts_at: subsite.startsAt,
            ends_at: subsite.endsAt,
            access: {
              allowed: subsite.runtimeStatus === 'enabled',
              status: subsite.runtimeStatus,
              code: subsite.accessCode,
              message: subsite.accessMessage,
            },
          },
        }),
      })
      return
    }

    if (path === '/api/subsites/logout-smoke/dashboard') {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            subsite: {
              id: 101,
              slug: 'logout-smoke',
              name: 'Logout Smoke Site',
              title: 'Logout Smoke Site',
              logo_url: '',
              favicon_url: '',
              theme_color: '#2563eb',
              status: 'enabled',
              runtime_status: 'enabled',
              registration_policy: 'open',
              starts_at: now - 3600,
              ends_at: now + 86400,
              access: {
                allowed: true,
                status: 'enabled',
                code: '',
                message: '',
              },
            },
            member: {
              subsite_id: 101,
              user_id: 7001,
              role: 'member',
              status: 'active',
              can_access: true,
              can_manage: false,
              joined_at: now - 1800,
            },
            base_url: 'http://127.0.0.1:4174/s/logout-smoke/v1',
            token: {
              id: 9001,
              name: 'Subsite key',
              masked_key: 'sk-smoke...tail',
              status: 1,
              created_time: now - 1200,
              accessed_time: 0,
              expired_time: -1,
              unlimited_quota: false,
            },
            quota: {
              site_daily_quota: quotaMetric(),
              site_window_quota: quotaMetric(),
              user_daily_quota: quotaMetric(),
              user_window_quota: quotaMetric(),
              site_daily_requests: quotaMetric(100),
              site_window_requests: quotaMetric(100),
              user_daily_requests: quotaMetric(100),
              user_window_requests: quotaMetric(100),
            },
            stats_24h: {
              window_seconds: 86400,
              calls: 0,
              prompt_tokens: 0,
              output_tokens: 0,
              total_tokens: 0,
              quota: 0,
              last_request_at: 0,
            },
            recent_logs: [],
          },
        }),
      })
      return
    }

    if (
      options.quotaExceeded &&
      path === '/api/subsites/quota-smoke/dashboard'
    ) {
      await route.fulfill({
        contentType: 'application/json',
        body: JSON.stringify({
          success: false,
          message:
            'subsite_quota_exceeded: Site daily quota exceeded. Try again after reset.',
          error_code: 'subsite_quota_exceeded',
        }),
      })
      return
    }

    await route.fulfill({
      status: 404,
      contentType: 'application/json',
      body: JSON.stringify({
        success: false,
        message: 'smoke_unmocked_api',
      }),
    })
  })
}

async function expectNoHorizontalOverflow(page: Page) {
  await expect
    .poll(async () =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth + 1
      )
    )
    .toBe(true)
}

test.describe('subsite state pages', () => {
  test.beforeEach(async ({ page }) => {
    await installCommonMocks(page)
  })

  test('closed subsite shows the closed page without horizontal overflow', async ({
    page,
  }) => {
    await page.goto('/s/closed-smoke')

    await expect(
      page.getByRole('heading', { name: 'Subsite is closed' })
    ).toBeVisible()
    await expect(page.getByText('Maintenance window in progress.')).toBeVisible()
    await expect(page.getByText('subsite_disabled')).toHaveCount(0)
    await expectNoHorizontalOverflow(page)
  })

  test('expired subsite shows the ended page without horizontal overflow', async ({
    page,
  }) => {
    await page.goto('/s/expired-smoke')

    await expect(
      page.getByRole('heading', { name: 'Subsite has ended' })
    ).toBeVisible()
    await expect(
      page.getByText('The access window for this subsite has ended.')
    ).toBeVisible()
    await expect(page.getByText('subsite_expired')).toHaveCount(0)
    await expectNoHorizontalOverflow(page)
  })

  test('missing subsite shows the not found page without horizontal overflow', async ({
    page,
  }) => {
    await page.goto('/s/missing-smoke')

    await expect(
      page.getByRole('heading', { name: 'Subsite not found' })
    ).toBeVisible()
    await expect(
      page.getByText('The subsite link is incorrect, removed, or no longer available.')
    ).toBeVisible()
    await expect(page.getByText('subsite_not_found')).toHaveCount(0)
    await expectNoHorizontalOverflow(page)
  })
})

test.describe('subsite quota state', () => {
  test.beforeEach(async ({ page }) => {
    await installCommonMocks(page, { quotaExceeded: true })
    await page.addInitScript(() => {
      window.localStorage.setItem(
        'user',
        JSON.stringify({
          id: 7001,
          username: 'smoke-user',
          display_name: 'Smoke User',
          role: 1,
          status: 1,
          group: 'default',
        })
      )
    })
  })

  test('quota exceeded dashboard state uses clear copy without horizontal overflow', async ({
    page,
  }) => {
    await page.goto('/s/quota-smoke/dashboard')

    await expect(page.getByRole('heading', { name: 'Quota exceeded' })).toBeVisible()
    await expect(page.getByText('Site daily quota exceeded')).toBeVisible()
    await expectNoHorizontalOverflow(page)
  })
})

test.describe('subsite dashboard sign out', () => {
  test.beforeEach(async ({ page }) => {
    await installCommonMocks(page)
    await page.addInitScript(() => {
      if (!window.location.pathname.endsWith('/dashboard')) return

      window.localStorage.setItem(
        'user',
        JSON.stringify({
          id: 7001,
          username: 'smoke-user',
          display_name: 'Smoke User',
          role: 1,
          status: 1,
          group: 'default',
        })
      )
      window.localStorage.setItem('uid', '7001')
    })
  })

  test('sign out from subsite dashboard returns to the subsite entry', async ({
    page,
  }) => {
    let logoutRequested = false
    page.on('request', (request) => {
      if (new URL(request.url()).pathname === '/api/user/logout') {
        logoutRequested = true
      }
    })

    await page.goto('/s/logout-smoke/dashboard')
    await expect(
      page.getByRole('heading', { name: 'Subsite console' })
    ).toBeVisible()

    await page.getByRole('button', { name: 'Sign out' }).click()
    const dialog = page.getByRole('alertdialog')
    await expect(dialog).toBeVisible()
    await dialog.getByRole('button', { name: 'Sign out' }).click()

    await expect(page).toHaveURL(/\/s\/logout-smoke$/)
    await expect
      .poll(() => logoutRequested, { message: 'logout endpoint was called' })
      .toBe(true)
    await expect
      .poll(() =>
        page.evaluate(() => ({
          user: window.localStorage.getItem('user'),
          uid: window.localStorage.getItem('uid'),
        }))
      )
      .toEqual({ user: null, uid: null })
  })
})
