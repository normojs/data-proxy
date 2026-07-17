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
import { Code2, KeyRound, MonitorSmartphone, ShieldCheck } from 'lucide-react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { PublicLayout } from '@/components/layout'

const DEVICE_START_EXAMPLE = `curl -sS -X POST "$BASE_URL/api/connected-apps/<slug>/device/start" \\
  -H 'Content-Type: application/json' \\
  -d '{"device_id":"desktop-1","device_name":"My App","platform":"darwin"}'`

const DEVICE_POLL_EXAMPLE = `curl -sS -X POST "$BASE_URL/api/connected-apps/<slug>/device/poll" \\
  -H 'Content-Type: application/json' \\
  -d '{"device_code":"<device_code>"}'`

const OAUTH_AUTHORIZE_EXAMPLE = `$BASE_URL/oauth/authorize?client_id=<slug>&redirect_uri=<https-callback>&response_type=code&scope=openai.chat%20quota.read&state=<state>&code_challenge=<s256>&code_challenge_method=S256`

const OAUTH_TOKEN_EXAMPLE = `curl -sS -X POST "$BASE_URL/api/oauth/token" \\
  -H 'Content-Type: application/x-www-form-urlencoded' \\
  -d 'grant_type=authorization_code&code=<code>&redirect_uri=<https-callback>&client_id=<slug>&code_verifier=<verifier>'`

export function DevelopersPage() {
  const { t } = useTranslation()

  return (
    <PublicLayout>
      <div className='mx-auto flex max-w-5xl flex-col gap-8 py-6 md:py-10'>
        <section className='space-y-4 border-b pb-8'>
          <div className='bg-muted text-muted-foreground flex size-11 items-center justify-center rounded-lg'>
            <Code2 className='size-5' aria-hidden />
          </div>
          <div className='space-y-2'>
            <h1 className='text-3xl font-semibold tracking-tight'>
              {t('Build with Data Proxy')}
            </h1>
            <p className='text-muted-foreground max-w-3xl text-base'>
              {t(
                'Let your product sign users in with their Data Proxy account and call the API with a scoped key. Use Device Code for desktop/CLI, or browser redirect (OAuth 2.0 + PKCE) for websites.'
              )}
            </p>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Button asChild>
              <Link to='/profile'>{t('Open developer tools')}</Link>
            </Button>
            <Button asChild variant='outline'>
              <Link to='/sign-in' search={{ redirect: '/developers' }}>
                {t('Sign in to apply')}
              </Link>
            </Button>
            <Button asChild variant='ghost'>
              <a href='/docs/user-quickstart.md' target='_blank' rel='noreferrer'>
                {t('API quickstart')}
              </a>
            </Button>
          </div>
        </section>

        <section className='grid gap-4 md:grid-cols-2'>
          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2 text-lg'>
                <MonitorSmartphone className='size-4' />
                {t('Desktop / CLI — Device Code')}
              </CardTitle>
              <CardDescription>
                {t(
                  'Best for native apps. The user opens a verification URL, approves access, and your app polls once for an API key.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-3 text-sm'>
              <ol className='text-muted-foreground list-decimal space-y-1 pl-4'>
                <li>{t('Call device/start to get device_code and user_code')}</li>
                <li>
                  {t('Open verification_uri (or send the user to /connect/device)')}
                </li>
                <li>{t('User signs in and approves scopes')}</li>
                <li>{t('Poll device/poll once to receive sk- api_key')}</li>
              </ol>
              <pre className='bg-muted overflow-x-auto rounded-md p-3 text-xs'>
                {DEVICE_START_EXAMPLE}
              </pre>
              <pre className='bg-muted overflow-x-auto rounded-md p-3 text-xs'>
                {DEVICE_POLL_EXAMPLE}
              </pre>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2 text-lg'>
                <KeyRound className='size-4' />
                {t('Website — OAuth 2.0 + PKCE')}
              </CardTitle>
              <CardDescription>
                {t(
                  'Best for web apps. Redirect the user to Data Proxy, then exchange the authorization code for a scoped sk- API key and OpenID id_token.'
                )}
              </CardDescription>
            </CardHeader>
            <CardContent className='space-y-3 text-sm'>
              <ol className='text-muted-foreground list-decimal space-y-1 pl-4'>
                <li>{t('Redirect to /oauth/authorize with PKCE')}</li>
                <li>{t('User approves on the consent page')}</li>
                <li>{t('Receive ?code= on your redirect_uri')}</li>
                <li>
                  {t('POST /api/oauth/token to get access_token (sk-) and id_token')}
                </li>
              </ol>
              <pre className='bg-muted overflow-x-auto rounded-md p-3 text-xs'>
                {OAUTH_AUTHORIZE_EXAMPLE}
              </pre>
              <pre className='bg-muted overflow-x-auto rounded-md p-3 text-xs'>
                {OAUTH_TOKEN_EXAMPLE}
              </pre>
            </CardContent>
          </Card>
        </section>

        <section className='grid gap-4 md:grid-cols-3'>
          <Card>
            <CardHeader>
              <CardTitle className='text-base'>{t('1. Apply for an app')}</CardTitle>
            </CardHeader>
            <CardContent className='text-muted-foreground text-sm'>
              {t(
                'Submit a Connected App request with slug, scopes, homepage, and callback URL (required for website login). Admins approve in System Settings → Connected Apps.'
              )}
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle className='text-base'>{t('2. Request least privilege')}</CardTitle>
            </CardHeader>
            <CardContent className='text-muted-foreground text-sm'>
              {t(
                'Common scopes: openai.models, openai.chat, quota.read, token.manage. Only request what your product needs.'
              )}
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2 text-base'>
                <ShieldCheck className='size-4' />
                {t('3. Secure the key')}
              </CardTitle>
            </CardHeader>
            <CardContent className='text-muted-foreground text-sm'>
              {t(
                'API keys are shown once. Store them securely. Users can revoke devices and grants from their Profile. Never put keys in public frontend code.'
              )}
            </CardContent>
          </Card>
        </section>

        <section className='rounded-lg border p-5'>
          <h2 className='mb-2 text-lg font-semibold'>{t('Endpoints at a glance')}</h2>
          <div className='text-muted-foreground grid gap-2 font-mono text-xs md:grid-cols-2'>
            <div>POST /api/connected-apps/:slug/device/start</div>
            <div>POST /api/connected-apps/:slug/device/poll</div>
            <div>GET /connect/device?user_code=…</div>
            <div>GET /oauth/authorize</div>
            <div>POST /api/oauth/token</div>
            <div>GET /api/oauth/userinfo</div>
            <div>GET /.well-known/openid-configuration</div>
            <div>GET /oauth/jwks.json</div>
          </div>
        </section>
      </div>
    </PublicLayout>
  )
}
