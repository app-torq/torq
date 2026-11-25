import { useSession } from '../app/session'
import type { Permission, Vehicle } from './types'

/** Per-vehicle permissions, in the order role editors show them. */
export const vehiclePermissions: Permission[] = [
  'vehicles.edit', 'vehicles.delete', 'specs.edit', 'service_logs.write', 'service_logs.delete',
  'reminders.write', 'repairs.write', 'repairs.delete', 'shares.manage', 'history.export',
]

/** Garage-wide permissions. */
export const garagePermissions: Permission[] = ['vehicles.create', 'categories.manage', 'repair_lists.manage', 'catalog.update']

/** Whether the signed-in user may change the vehicle in this way. The server decides; this only hides actions. */
export function canOn(vehicle: Pick<Vehicle, 'permissions'> | undefined, p: Permission): boolean {
  return vehicle?.permissions.includes(p) ?? false
}

/** Whether the signed-in user is an administrator. */
export function useIsAdmin(): boolean {
  return useSession().user.role === 'admin'
}

/**
 * Whether the signed-in user holds a permission: garage-wide keys apply
 * everywhere; for per-vehicle keys this says the role grants it, which
 * canOn narrows to one vehicle.
 */
export function useCan(p: Permission): boolean {
  const { user } = useSession()
  return user.role === 'admin' || (user.permissions?.includes(p) ?? false)
}
