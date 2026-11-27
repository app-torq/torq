import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Permission, SharedWith, Vehicle } from '../../lib/types'
import { renderWithApp } from '../../test/render'
import { SharedWithCard } from './SharedWithCard'

function vehicle(permissions: Permission[]): Vehicle {
  return {
    id: 4, vehicle_type: 'car', make: 'Volkswagen', model: 'Golf', chassis_code: '', model_year: null, engine_code: '',
    vin: null, license_plate: null, hsn: null, tsn: null, displacement_cc: null, power_kw: null, power_w: null, fuel_type: null,
    current_odometer_km: 0, odometer_unit: 'km', has_odometer: true, hour_meters: [], displacement_unit: null, power_unit: null,
    avatar_url: null, archived: false, reminder_counts: { due_soon: 0, overdue: 0 }, created_at: 'x', updated_at: 'x', permissions,
  }
}
const jan: SharedWith = { user_id: 7, username: 'jan', display_name: 'Jan', level: 'view', created_at: '2026-11-27T10:00:00Z' }

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

afterEach(() => { vi.restoreAllMocks() })

describe('SharedWithCard', () => {
  it('lists shares and shares by exact username without suggesting names', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation((_, init) =>
      Promise.resolve(init?.method === 'PUT' ? json({ share: jan }) : json({ items: [jan] })))
    renderWithApp(<SharedWithCard vehicle={vehicle(['vehicles.edit', 'vehicle_access.share'])} />)
    expect(await screen.findByText('Jan')).toBeInTheDocument()
    const field = screen.getByLabelText('Username')
    expect(field).toHaveAttribute('autocomplete', 'off')
    expect(field).not.toHaveAttribute('list')
    await userEvent.type(field, ' kai ')
    await userEvent.selectOptions(screen.getByLabelText('Access'), 'edit')
    await userEvent.click(screen.getByRole('button', { name: 'Share' }))
    const put = fetchSpy.mock.calls.find(([, init]) => init?.method === 'PUT')
    expect(put?.[0]).toBe('/api/v1/vehicles/4/shared-with')
    expect(put?.[1]?.body).toBe(JSON.stringify({ username: 'kai', level: 'edit' }))
  })

  it('shows the one refusal message', async () => {
    vi.spyOn(globalThis, 'fetch').mockImplementation((_, init) =>
      Promise.resolve(init?.method === 'PUT' ? json({ error: { code: 'sharing.refused' } }, 422) : json({ items: [] })))
    renderWithApp(<SharedWithCard vehicle={vehicle(['vehicle_access.share'])} />)
    expect(await screen.findByText('Not shared with anyone yet.')).toBeInTheDocument()
    await userEvent.type(screen.getByLabelText('Username'), 'admin')
    await userEvent.click(screen.getByRole('button', { name: 'Share' }))
    expect(await screen.findByText('This vehicle can’t be shared with this user. Check the username.')).toBeInTheDocument()
  })

  it('lets a user without the permission only take shares back', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockImplementation((_, init) =>
      Promise.resolve(init?.method === 'DELETE' ? new Response(null, { status: 204 }) : json({ items: [jan] })))
    renderWithApp(<SharedWithCard vehicle={vehicle(['vehicles.edit'])} />)
    expect(await screen.findByText(/no longer allows sharing/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Username')).toBeNull()
    expect(screen.getByLabelText('Access for Jan')).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: 'Take back from Jan' }))
    const del = fetchSpy.mock.calls.find(([, init]) => init?.method === 'DELETE')
    expect(del?.[0]).toBe('/api/v1/vehicles/4/shared-with/7')
  })

  it('stays hidden without the permission and without shares', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(json({ items: [] }))
    const { container } = renderWithApp(<SharedWithCard vehicle={vehicle([])} />)
    await vi.waitFor(() => { expect(fetchSpy).toHaveBeenCalled() })
    expect(container).toBeEmptyDOMElement()
  })
})
