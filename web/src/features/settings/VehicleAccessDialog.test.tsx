import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithApp, testSession } from '../../test/render'
import { VehicleAccessDialog } from './VehicleAccessDialog'

const vehicle = (id: number, make: string) => ({ id, make, model: 'X', license_plate: null })
const user = { ...testSession.user, id: 7, role: 'member' as const, display_name: 'Anna', role_name: 'Customer' }

function respond(allVehicles = false) {
  return vi.spyOn(globalThis, 'fetch').mockImplementation((url) => {
    const body = (typeof url === 'string' ? url : '').startsWith('/api/v1/vehicles')
      ? { items: [vehicle(1, 'Alpha'), vehicle(2, 'Bravo'), vehicle(3, 'Charlie')], total: 3 }
      : { all_vehicles: allVehicles, items: [{ vehicle_id: 1, level: 'edit' }] }
    return Promise.resolve(new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } }))
  })
}

afterEach(() => { vi.restoreAllMocks() })

describe('VehicleAccessDialog', () => {
  it('shows the assignments and saves None / View / Edit per vehicle', async () => {
    const fetchSpy = respond()
    renderWithApp(<VehicleAccessDialog user={user} onClose={() => undefined} />)
    const alpha = await screen.findByRole('radiogroup', { name: 'Alpha X' })
    expect(within(alpha).getByRole('radio', { name: 'Edit' })).toHaveAttribute('aria-checked', 'true')
    await userEvent.click(within(screen.getByRole('radiogroup', { name: 'Bravo X' })).getByRole('radio', { name: 'View' }))
    await userEvent.click(within(alpha).getByRole('radio', { name: 'None' }))
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    const put = fetchSpy.mock.calls.find(([, init]) => init?.method === 'PUT')
    expect(put?.[0]).toBe('/api/v1/users/7/vehicle-access')
    expect(JSON.parse(put?.[1]?.body as string)).toEqual({ items: [{ vehicle_id: 2, level: 'view' }] })
  })

  it('says when the role sees every vehicle anyway', async () => {
    respond(true)
    renderWithApp(<VehicleAccessDialog user={user} onClose={() => undefined} />)
    expect(await screen.findByText(/The role Customer sees all vehicles/)).toBeInTheDocument()
  })
})
