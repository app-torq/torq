import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { PermissionInfo, Role } from '../../lib/types'
import { renderWithApp } from '../../test/render'
import { RolesPage } from './RolesPage'

const permissions: PermissionInfo[] = [
  { key: 'service_logs.write', scope: 'vehicle' }, { key: 'service_logs.delete', scope: 'vehicle' },
  { key: 'vehicles.create', scope: 'garage' },
]
const roles: Role[] = [
  { id: 1, name: 'Administrator', custom_name: null, builtin_key: 'admin', description: '', all_vehicles: true, permissions: [], user_count: 1 },
  { id: 2, name: 'Member', custom_name: null, builtin_key: 'member', description: '', all_vehicles: true, permissions: ['service_logs.write'], user_count: 2 },
  { id: 4, name: 'Customer', custom_name: 'Customer', builtin_key: null, description: 'Own cars only', all_vehicles: false, permissions: ['service_logs.write'], user_count: 3 },
]

function respond() {
  return vi.spyOn(globalThis, 'fetch').mockImplementation((_, init) =>
    Promise.resolve(new Response(JSON.stringify(init?.method && init.method !== 'GET' ? { role: roles[2] } : { items: roles, permissions }), { status: 200, headers: { 'Content-Type': 'application/json' } })),
  )
}

afterEach(() => { vi.restoreAllMocks() })

describe('RolesPage', () => {
  it('lists roles; the Administrator role can be neither edited nor deleted', async () => {
    respond()
    renderWithApp(<RolesPage />)
    const admin = (await screen.findByText('Administrator')).closest('tr')
    expect(admin).not.toBeNull()
    expect(within(admin as HTMLElement).queryAllByRole('button')).toHaveLength(0)
    const member = screen.getByText('Member').closest('tr') as HTMLElement
    expect(within(member).getByRole('button', { name: 'Edit Member' })).toBeInTheDocument()
    expect(within(member).queryByRole('button', { name: 'Delete Member' })).toBeNull()
    const customer = screen.getByText('Customer').closest('tr') as HTMLElement
    expect(within(customer).getByText('Assigned vehicles')).toBeInTheDocument()
    expect(within(customer).getByRole('button', { name: 'Delete Customer' })).toBeInTheDocument()
  })

  it('saves the chosen permissions', async () => {
    const fetchSpy = respond()
    renderWithApp(<RolesPage />)
    await userEvent.click(await screen.findByRole('button', { name: 'Edit Customer' }))
    await userEvent.click(screen.getByRole('checkbox', { name: 'Delete service entries' }))
    await userEvent.click(screen.getByRole('checkbox', { name: 'Add vehicles' }))
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    const patch = fetchSpy.mock.calls.find(([, init]) => init?.method === 'PATCH')
    expect(patch?.[0]).toBe('/api/v1/roles/4')
    expect(JSON.parse(patch?.[1]?.body as string)).toMatchObject({
      name: 'Customer', all_vehicles: false, permissions: ['service_logs.write', 'service_logs.delete', 'vehicles.create'],
    })
  })

  it('asks where the users go when deleting a role that has some', async () => {
    const fetchSpy = respond()
    renderWithApp(<RolesPage />)
    await userEvent.click(await screen.findByRole('button', { name: 'Delete Customer' }))
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText('3 users have this role. Choose the role they move to.')).toBeInTheDocument()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
    expect(fetchSpy.mock.calls.some(([url, init]) => init?.method === 'DELETE' && url === '/api/v1/roles/4?move_to=2')).toBe(true)
  })
})
