import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { PermissionInfo, Role } from '../../lib/types'
import { renderWithApp } from '../../test/render'
import { RolesPage } from './RolesPage'

const permissions: PermissionInfo[] = [
  { key: 'service_logs.write', scope: 'vehicle' }, { key: 'service_logs.delete', scope: 'vehicle' },
  { key: 'vehicles.create', scope: 'garage' },
  { key: 'service_logs.view', scope: 'view' }, { key: 'costs.view', scope: 'view' },
]
const roles: Role[] = [
  { id: 1, name: 'Administrator', custom_name: null, builtin_key: 'admin', description: '', all_vehicles: true, permissions: [], user_count: 1, require_mfa: false },
  { id: 2, name: 'Member', custom_name: null, builtin_key: 'member', description: '', all_vehicles: true, permissions: ['service_logs.write'], user_count: 2, require_mfa: false },
  { id: 4, name: 'Customer', custom_name: 'Customer', builtin_key: null, description: 'Own cars only', all_vehicles: false, permissions: ['service_logs.view', 'service_logs.write'], user_count: 3, require_mfa: false },
]

function respond() {
  return vi.spyOn(globalThis, 'fetch').mockImplementation((_, init) =>
    Promise.resolve(new Response(JSON.stringify(init?.method && init.method !== 'GET' ? { role: roles[2] } : { items: roles, permissions }), { status: 200, headers: { 'Content-Type': 'application/json' } })),
  )
}

afterEach(() => { vi.restoreAllMocks() })

describe('RolesPage', () => {
  it('lists roles; the Administrator role only takes the two-factor requirement', async () => {
    const fetchSpy = respond()
    renderWithApp(<RolesPage />)
    const admin = (await screen.findByText('Administrator')).closest('tr') as HTMLElement
    expect(within(admin).queryByRole('button', { name: 'Delete Administrator' })).toBeNull()
    await userEvent.click(within(admin).getByRole('button', { name: 'Edit Administrator' }))
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).queryByLabelText('Name')).toBeNull()
    await userEvent.click(within(dialog).getByRole('checkbox', { name: 'Require two-factor sign-in' }))
    expect(within(dialog).getByText(/right after saving you can only set one up/)).toBeInTheDocument()
    await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    const patch = fetchSpy.mock.calls.find(([, init]) => init?.method === 'PATCH')
    expect(patch?.[0]).toBe('/api/v1/roles/1')
    expect(JSON.parse(patch?.[1]?.body as string)).toEqual({ require_mfa: true })
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
      name: 'Customer', all_vehicles: false, require_mfa: false, permissions: ['service_logs.view', 'service_logs.write', 'service_logs.delete', 'vehicles.create'],
    })
  })

  it('couples changing an area to seeing it; new roles see everything', async () => {
    const fetchSpy = respond()
    renderWithApp(<RolesPage />)
    await userEvent.click(await screen.findByRole('button', { name: 'Edit Customer' }))
    const see = screen.getByRole('checkbox', { name: 'See the Service Logbook' })
    await userEvent.click(see)
    expect(see).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Add and edit service entries' })).not.toBeChecked()
    await userEvent.click(screen.getByRole('checkbox', { name: 'Delete service entries' }))
    expect(see).toBeChecked()
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    await userEvent.click(screen.getByRole('button', { name: 'Add role' }))
    expect(screen.getByRole('checkbox', { name: 'See costs' })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Add and edit service entries' })).not.toBeChecked()
    await userEvent.type(screen.getByRole('textbox', { name: 'Name' }), 'Helper')
    await userEvent.click(screen.getByRole('checkbox', { name: 'See costs' }))
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    const post = fetchSpy.mock.calls.find(([, init]) => init?.method === 'POST')
    expect((JSON.parse(post?.[1]?.body as string) as { permissions: string[] }).permissions).not.toContain('costs.view')
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
