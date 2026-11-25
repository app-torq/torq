import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { MfaStatus } from '../../lib/types'
import { renderWithApp, testSession } from '../../test/render'
import { SecurityPage } from './SecurityPage'

const none: MfaStatus = { totp: false, recovery_codes_left: 0, passkeys: [], required: false, passkeys_available: false }

function json(status: number, body: unknown) {
  return Promise.resolve(new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }))
}

afterEach(() => { vi.restoreAllMocks() })

describe('SecurityPage', () => {
  it('sets up an authenticator app and shows the recovery codes once', async () => {
    const codes = Array.from({ length: 10 }, (_, i) => `abcde-fgh${String(i)}k`)
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation((url, init) => {
      const u = typeof url === 'string' ? url : ''
      if (u.endsWith('/me/totp') && init?.method === 'POST') return json(200, { secret: 'JBSWY3DPEHPK3PXP', uri: 'otpauth://totp/torq:marek?secret=JBSWY3DPEHPK3PXP' })
      if (u.endsWith('/me/totp/confirm')) return json(200, { recovery_codes: codes })
      return json(200, none)
    })
    renderWithApp(<SecurityPage />)
    expect(await screen.findByText(/Passkeys aren't available on this server/)).toBeInTheDocument()
    expect(screen.queryByText('Recovery codes')).toBeNull()

    await userEvent.click(screen.getByRole('button', { name: 'Set up' }))
    expect(await screen.findByText('JBSW Y3DP EHPK 3PXP')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'QR code for the authenticator app' })).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('Code from the app'), '123456')
    await userEvent.click(screen.getByRole('button', { name: 'Turn on' }))
    expect(await screen.findByText('Your recovery codes')).toBeInTheDocument()
    expect(screen.getByText(codes[0] as string)).toBeInTheDocument()
    const confirm = fetchSpy.mock.calls.find(([u]) => u === '/api/v1/me/totp/confirm')
    expect(JSON.parse(confirm?.[1]?.body as string)).toEqual({ code: '123456' })
    await userEvent.click(screen.getByRole('button', { name: 'I saved them' }))
    expect(screen.queryByText('Your recovery codes')).toBeNull()
  })

  it('explains a required setup and hides the password form', async () => {
    vi.spyOn(globalThis, 'fetch').mockImplementation(() => json(200, { ...none, required: true }))
    renderWithApp(<SecurityPage />, { ...testSession, user: { ...testSession.user, mfa_setup_required: true } })
    expect(await screen.findByText(/Your account requires two-factor sign-in/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Current password')).toBeNull()
  })
})
