import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { vehiclePermissions } from '../../lib/permissions'
import type { Share, Vehicle } from '../../lib/types'
import { renderWithApp, testSession } from '../../test/render'
import { VehicleQrCard } from './VehicleQrCard'

const vehicle: Vehicle = {
  id: 4, vehicle_type: 'car', make: 'Volkswagen', model: 'Golf', chassis_code: '', model_year: null, engine_code: '',
  vin: null, license_plate: null, hsn: null, tsn: null, displacement_cc: null, power_kw: null, power_w: null, fuel_type: null,
  current_odometer_km: 0, odometer_unit: 'km', has_odometer: true, hour_meters: [], displacement_unit: null, power_unit: null,
  avatar_url: null, archived: false, reminder_counts: { due_soon: 0, overdue: 0 }, created_at: 'x', updated_at: 'x', permissions: vehiclePermissions,
}
const share: Share = {
  token: 'abc', path: '/s/abc', include_costs: false, include_license_plate: true, include_vin: false,
  created_at: '2026-11-01T10:00:00Z', last_opened_at: null,
}

function respond(body: unknown) {
  return vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } }))
}

function renderCard(session = testSession) {
  return renderWithApp(<MemoryRouter><VehicleQrCard vehicle={vehicle} /></MemoryRouter>, session)
}

afterEach(() => { vi.restoreAllMocks() })

describe('VehicleQrCard', () => {
  it('offers to turn the public link on when there is none', async () => {
    respond({ share: null })
    renderCard()
    expect(await screen.findByRole('button', { name: 'Turn on' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: 'Public link' })).toBeNull()
  })

  it('shows the address, options and the code choice when the link is on', async () => {
    respond({ share })
    renderCard()
    expect(await screen.findByText(`${window.location.origin}/s/abc`)).toBeInTheDocument()
    expect(screen.getByRole('checkbox', { name: 'License plate' })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Costs' })).not.toBeChecked()
    expect(screen.getByRole('link', { name: 'Print label' })).toHaveAttribute('href', '/vehicles/4/qr/print')
    await userEvent.click(screen.getByRole('button', { name: 'Public link' }))
    expect(screen.getByRole('link', { name: 'Print label' })).toHaveAttribute('href', '/vehicles/4/qr/print?code=public')
  })

  it('saves an option right away', async () => {
    const fetchSpy = respond({ share })
    renderCard()
    await userEvent.click(await screen.findByRole('checkbox', { name: 'Costs' }))
    const put = fetchSpy.mock.calls.find(([, init]) => init?.method === 'PUT')
    expect(put?.[0]).toBe('/api/v1/vehicles/4/share')
    expect(put?.[1]?.body).toBe(JSON.stringify({ include_costs: true }))
  })

  it('hides the public link when the server has it turned off', () => {
    const fetchSpy = respond({ share })
    renderCard({ ...testSession, instance: { ...testSession.instance, public_links: false } })
    expect(screen.queryByText('Public link')).toBeNull()
    expect(fetchSpy).not.toHaveBeenCalled()
  })

  it('shows the link but no controls without the permission to manage it', async () => {
    respond({ share })
    renderWithApp(<MemoryRouter><VehicleQrCard vehicle={{ ...vehicle, permissions: [] }} /></MemoryRouter>)
    expect(await screen.findByText(`${window.location.origin}/s/abc`)).toBeInTheDocument()
    expect(screen.queryByRole('checkbox', { name: 'Costs' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Renew link' })).toBeNull()
  })

  it('offers nothing to turn on without the permission', async () => {
    const fetchSpy = respond({ share: null })
    renderWithApp(<MemoryRouter><VehicleQrCard vehicle={{ ...vehicle, permissions: [] }} /></MemoryRouter>)
    await vi.waitFor(() => { expect(fetchSpy).toHaveBeenCalled() })
    expect(screen.queryByRole('button', { name: 'Turn on' })).toBeNull()
  })

  it('cannot turn the link on in the demo', async () => {
    respond({ share: null })
    renderCard({ ...testSession, instance: { ...testSession.instance, demo_mode: true } })
    expect(await screen.findByRole('button', { name: 'Turn on' })).toBeDisabled()
    expect(screen.getByText('Public links cannot be changed in the demo.')).toBeInTheDocument()
  })
})
