import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, screen } from '@testing-library/react'
import { renderWithApp } from '../../test/render'
import { InstallHint } from './InstallHint'
import { captureInstallPrompt } from './install'

const iPhone = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1'
const iPadDesktopMode = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15'
const android = 'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36'

function device({ ua = android, touch = true, standalone = false, maxTouchPoints = 5 } = {}) {
  vi.spyOn(navigator, 'userAgent', 'get').mockReturnValue(ua)
  // jsdom has no maxTouchPoints, so it can't be spied on.
  Object.defineProperty(navigator, 'maxTouchPoints', { value: maxTouchPoints, configurable: true })
  window.matchMedia = vi.fn((q: string) => ({
    matches: q === '(pointer: coarse)' ? touch : q === '(display-mode: standalone)' ? standalone : false,
  })) as unknown as typeof window.matchMedia
}

describe('InstallHint', () => {
  beforeEach(() => { localStorage.clear() })
  afterEach(() => { vi.restoreAllMocks() })

  it('stays hidden on devices without touch', () => {
    device({ touch: false })
    renderWithApp(<InstallHint />)
    expect(screen.queryByText('Use torq like an app')).toBeNull()
  })

  it('stays hidden in the installed app', () => {
    device({ standalone: true })
    renderWithApp(<InstallHint />)
    expect(screen.queryByText('Use torq like an app')).toBeNull()
  })

  it('shows the Safari steps on an iPhone and on an iPad in desktop mode', () => {
    device({ ua: iPhone })
    const first = renderWithApp(<InstallHint />)
    expect(screen.getByText(/Add to Home Screen/)).toBeInTheDocument()
    first.unmount()
    device({ ua: iPadDesktopMode, maxTouchPoints: 5 })
    renderWithApp(<InstallHint />)
    expect(screen.getByText(/Add to Home Screen/)).toBeInTheDocument()
  })

  it('shows the Android steps, and an Install button once the browser offers one', () => {
    device({ ua: android })
    captureInstallPrompt()
    renderWithApp(<InstallHint />)
    expect(screen.getByText(/Install app/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Install' })).toBeNull()

    const prompt = vi.fn(() => Promise.resolve())
    act(() => { window.dispatchEvent(Object.assign(new Event('beforeinstallprompt', { cancelable: true }), { prompt })) })
    fireEvent.click(screen.getByRole('button', { name: 'Install' }))
    expect(prompt).toHaveBeenCalledOnce()
  })

  it('remembers a dismissal on this device', () => {
    device()
    const first = renderWithApp(<InstallHint />)
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))
    expect(screen.queryByText('Use torq like an app')).toBeNull()
    expect(localStorage.getItem('torq.installHint.dismissed')).toBe('1')
    first.unmount()
    renderWithApp(<InstallHint />)
    expect(screen.queryByText('Use torq like an app')).toBeNull()
  })
})
