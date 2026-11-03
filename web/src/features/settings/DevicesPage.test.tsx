import { describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithApp, testSession } from '../../test/render'
import { DevicesPage } from './DevicesPage'

describe('DevicesPage', () => {
  it('shows only a notice in the demo and never asks the server', () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
    renderWithApp(<DevicesPage />, { ...testSession, instance: { ...testSession.instance, demo_mode: true } })
    expect(screen.getByRole('status')).toHaveTextContent('not available in the demo')
    expect(screen.queryByRole('table')).toBeNull()
    expect(fetchSpy).not.toHaveBeenCalled()
    fetchSpy.mockRestore()
  })
})
