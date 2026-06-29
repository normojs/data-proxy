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
  runtimeStatus: 'disabled' | 'expired'
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

const publicSubsites: Record<string, PublicSubsiteState> = {
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
              allowed: false,
              status: subsite.runtimeStatus,
              code: subsite.accessCode,
              message: subsite.accessMessage,
            },
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
