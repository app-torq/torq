import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithApp } from '../../test/render'
import { LoginPage } from './LoginPage'

function json(status: number, body: unknown) {
  return Promise.resolve(new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }))
}

afterEach(() => { vi.restoreAllMocks() })

describe('LoginPage', () => {
  it('asks for the code after the password and sends it with the challenge', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation((url) => {
      const u = typeof url === 'string' ? url : ''
      if (u.endsWith('/auth/login')) return json(401, { error: { code: 'auth.mfa_required', params: { challenge: 'ch1', methods: ['totp', 'recovery_code'] } } })
      if (u.endsWith('/auth/mfa')) return json(401, { error: { code: 'auth.mfa_invalid', params: { attempts_left: 4 } } })
      return json(200, { passkeys: false, branding: { name: '', logo_url: null } })
    })
    renderWithApp(<LoginPage />)
    await userEvent.type(screen.getByLabelText('Username'), 'anna')
    await userEvent.type(screen.getByLabelText('Password'), 'workshop-manual-1')
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByRole('heading', { name: 'Two-factor sign-in' })).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('Code'), '123456')
    await userEvent.click(screen.getByRole('button', { name: 'Verify' }))
    const mfa = fetchSpy.mock.calls.find(([u]) => u === '/api/v1/auth/mfa')
    expect(JSON.parse(mfa?.[1]?.body as string)).toEqual({ challenge: 'ch1', code: '123456' })
    expect(await screen.findByText("That code didn't work.")).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Use a recovery code' }))
    expect(screen.getByLabelText('Recovery code')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Use a passkey' })).toBeNull()
  })
})
