import { afterEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { OfflineScreen } from './OfflineScreen'

describe('OfflineScreen', () => {
  afterEach(() => { vi.restoreAllMocks() })

  it('explains the unreachable server in English by default', () => {
    vi.spyOn(navigator, 'languages', 'get').mockReturnValue(['en-US'])
    render(<OfflineScreen onRetry={() => {}} />)
    expect(screen.getByRole('heading', { name: "Can't reach your torq server" })).toBeInTheDocument()
    expect(screen.getByText(new RegExp(window.location.host))).toBeInTheDocument()
  })

  it('speaks German when the browser prefers it', () => {
    vi.spyOn(navigator, 'languages', 'get').mockReturnValue(['de-DE', 'en'])
    render(<OfflineScreen onRetry={() => {}} />)
    expect(screen.getByRole('heading', { name: 'Dein torq-Server ist nicht erreichbar' })).toBeInTheDocument()
  })

  it('retries on demand', () => {
    vi.spyOn(navigator, 'languages', 'get').mockReturnValue(['en'])
    const onRetry = vi.fn()
    render(<OfflineScreen onRetry={onRetry} />)
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(onRetry).toHaveBeenCalledOnce()
  })
})
