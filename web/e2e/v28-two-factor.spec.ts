import { createHmac } from 'node:crypto'
import { expect, test, type Browser, type Page } from '@playwright/test'
import { apiPost, login } from './helpers'

const password = 'workshop-manual-1'

function base32(s: string): Buffer {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'
  let bits = ''
  for (const c of s.replace(/\s/g, '')) bits += alphabet.indexOf(c).toString(2).padStart(5, '0')
  const bytes: number[] = []
  for (let i = 0; i + 8 <= bits.length; i += 8) bytes.push(parseInt(bits.slice(i, i + 8), 2))
  return Buffer.from(bytes)
}

// RFC 6238 code for the 30-second step `offset` steps from now.
function totp(secret: string, offset = 0): string {
  const step = Math.floor(Date.now() / 30000) + offset
  const msg = Buffer.alloc(8)
  msg.writeBigUInt64BE(BigInt(step))
  const h = createHmac('sha1', base32(secret)).update(msg).digest()
  const off = (h[h.length - 1] ?? 0) & 0x0f
  return String((h.readUInt32BE(off) & 0x7fffffff) % 1_000_000).padStart(6, '0')
}

async function signInUI(page: Page, username: string) {
  await page.goto('/')
  await page.getByLabel('Username').fill(username)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
}

async function newUser(browser: Browser, username: string, baseURL?: string) {
  const ctx = await browser.newContext({ locale: 'en-US', ...(baseURL ? { baseURL } : {}) })
  return { ctx, page: await ctx.newPage(), username }
}

// Two-factor sign-in (#45): an authenticator app with recovery codes, the
// requirement gate and passkeys.
test('V28: two-factor sign-in and passkeys', async ({ page, browser }) => {
  await login(page)
  const roles = ((await (await page.request.get('/api/v1/roles')).json()) as { items: { id: number; builtin_key: string | null }[] }).items
  const member = roles.find((r) => r.builtin_key === 'member')?.id
  for (const username of ['otp28', 'gate28', 'pk28']) {
    await apiPost(page.request, '/users', { username, display_name: username, password, role_id: member })
  }

  // Set up an authenticator app in Settings → Sign-in & security.
  const u = await newUser(browser, 'otp28')
  await signInUI(u.page, u.username)
  await expect(u.page.getByRole('button', { name: 'Sign out' })).toBeVisible()
  await u.page.goto('/settings/security')
  await u.page.getByRole('button', { name: 'Set up' }).click()
  const secret = (await u.page.locator('code').first().textContent()) ?? ''
  await u.page.getByLabel('Code from the app').fill(totp(secret))
  await u.page.getByRole('button', { name: 'Turn on' }).click()
  await expect(u.page.getByRole('heading', { name: 'Your recovery codes' })).toBeVisible()
  const recovery = (await u.page.locator('#new-recovery-codes ~ ul li, section[aria-labelledby="new-recovery-codes"] li').allTextContents())
  expect(recovery).toHaveLength(10)
  await u.page.getByRole('button', { name: 'I saved them' }).click()
  await expect(u.page.getByText('10 unused codes left.')).toBeVisible()
  await u.page.getByRole('button', { name: 'Sign out' }).click()

  // Password, then a code from the app (the next interval: the setup code is used up).
  await signInUI(u.page, u.username)
  await expect(u.page.getByRole('heading', { name: 'Two-factor sign-in' })).toBeVisible()
  await u.page.getByLabel('Code').fill('000000')
  await u.page.getByRole('button', { name: 'Verify' }).click()
  await expect(u.page.getByText("That code didn't work.")).toBeVisible()
  await u.page.getByLabel('Code').fill(totp(secret, 1))
  await u.page.getByRole('button', { name: 'Verify' }).click()
  await expect(u.page.getByRole('link', { name: 'Vehicles' }).first()).toBeVisible()
  await u.page.getByRole('button', { name: 'Sign out' }).click()

  // A recovery code instead.
  await signInUI(u.page, u.username)
  await u.page.getByRole('button', { name: 'Use a recovery code' }).click()
  await u.page.getByLabel('Recovery code').fill(recovery[0] ?? '')
  await u.page.getByRole('button', { name: 'Verify' }).click()
  await expect(u.page.getByRole('link', { name: 'Vehicles' }).first()).toBeVisible()
  await u.ctx.close()

  // The administrator requires 2FA for a user: after sign-in only the setup is possible.
  const users = ((await (await page.request.get('/api/v1/users')).json()) as { items: { id: number; username: string }[] }).items
  const gate = users.find((x) => x.username === 'gate28')?.id ?? 0
  const patched = await page.request.patch(`/api/v1/users/${String(gate)}`, { headers: { 'X-Torq-Request': '1' }, data: { mfa_required: true } })
  expect(patched.ok()).toBeTruthy()
  const g = await newUser(browser, 'gate28')
  await signInUI(g.page, g.username)
  await expect(g.page.getByText(/Your account requires two-factor sign-in/)).toBeVisible()
  await g.page.goto('/vehicles')
  await expect(g.page.getByRole('heading', { name: 'Sign-in & security' })).toBeVisible()
  await expect(g.page.getByRole('link', { name: 'Vehicles' })).toHaveCount(0)
  const refused = await g.page.request.get('/api/v1/vehicles')
  expect(refused.status()).toBe(403)
  await g.ctx.close()

  // The users list shows who has 2FA.
  await page.goto('/settings/users')
  await expect(page.getByRole('row', { name: /otp28/ })).toContainText('On')
  await expect(page.getByRole('row', { name: /gate28/ })).toContainText('Required')

  // Passkeys, with Chromium's virtual authenticator, on localhost (passkeys
  // need a domain); skipped where the server has no address configured.
  const info = (await (await page.request.get('/api/v1/instance')).json()) as { passkeys: boolean }
  // The local server sets TORQ_BASE_URL (playwright.config.ts); the container run doesn't.
  expect(info.passkeys || process.env.TORQ_E2E_URL !== undefined).toBe(true)
  if (!info.passkeys) return
  const origin = new URL(page.url()).port
  const p = await newUser(browser, 'pk28', `http://localhost:${origin}`)
  const cdp = await p.ctx.newCDPSession(p.page)
  await cdp.send('WebAuthn.enable')
  await cdp.send('WebAuthn.addVirtualAuthenticator', {
    options: { protocol: 'ctap2', transport: 'internal', hasResidentKey: true, hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true },
  })
  await signInUI(p.page, p.username)
  await expect(p.page.getByRole('button', { name: 'Sign out' })).toBeVisible()
  await p.page.goto('/settings/security')
  await p.page.getByRole('button', { name: 'Add passkey' }).click()
  await p.page.getByRole('dialog').getByLabel('Name').fill('Test laptop')
  await p.page.getByRole('dialog').getByRole('button', { name: 'Continue' }).click()
  await expect(p.page.getByRole('heading', { name: 'Your recovery codes' })).toBeVisible()
  await p.page.getByRole('button', { name: 'I saved them' }).click()
  await expect(p.page.getByText('Test laptop')).toBeVisible()
  await p.page.getByRole('button', { name: 'Sign out' }).click()

  await p.page.goto('/')
  await p.page.getByRole('button', { name: 'Sign in with a passkey' }).click()
  await expect(p.page.getByRole('link', { name: 'Vehicles' }).first()).toBeVisible()
  await p.ctx.close()
})
