import { useSession } from '../app/session'
import type { Permission, User, Vehicle } from './types'

/** Per-vehicle permissions, in the order role editors show them. */
export const vehiclePermissions: Permission[] = [
  'vehicles.edit', 'vehicles.delete', 'specs.edit', 'service_logs.write', 'service_logs.delete',
  'reminders.write', 'repairs.write', 'repairs.delete', 'shares.manage', 'vehicle_access.share', 'history.export',
]

/** Garage-wide permissions. */
export const garagePermissions: Permission[] = ['vehicles.create', 'categories.manage', 'repair_lists.manage', 'catalog.update']

/** What a role sees; areas first, then fields. */
export const viewPermissions: Permission[] = [
  'service_logs.view', 'repairs.view', 'reminders.view', 'reports.view',
  'costs.view', 'notes.view', 'attachments.view', 'vehicle_ids.view',
]

/** The "see" permission a change needs: a role can't change an area it doesn't see. */
export const needsView: Partial<Record<Permission, Permission>> = {
  'service_logs.write': 'service_logs.view',
  'service_logs.delete': 'service_logs.view',
  'repairs.write': 'repairs.view',
  'repairs.delete': 'repairs.view',
  'reminders.write': 'reminders.view',
}

/** Whether the user holds a permission; administrators hold every one. */
export function userCan(user: Pick<User, 'role' | 'permissions'>, p: Permission): boolean {
  return user.role === 'admin' || (user.permissions?.includes(p) ?? false)
}

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
  return userCan(useSession().user, p)
}
