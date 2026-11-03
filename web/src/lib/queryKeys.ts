// Centralized React Query keys so invalidation after writes stays consistent.

export const queryKeys = {
  setupStatus: ['setup-status'] as const,
  me: ['me'] as const,
  users: ['users'] as const,
  sessions: ['sessions'] as const,
  instance: ['instance-settings'] as const,
  vehicles: ['vehicles'] as const,
  vehicleList: (archived: string) => ['vehicles', 'list', archived] as const,
  vehicle: (id: number) => ['vehicles', 'detail', id] as const,
  odometerReadings: (id: number) => ['vehicles', 'readings', id] as const,
  specs: (vehicleId: number) => ['specs', vehicleId] as const,
  categories: ['service-categories'] as const,
  serviceLogs: ['service-logs'] as const,
  serviceLogList: (filters: object) => ['service-logs', 'list', filters] as const,
  serviceLog: (id: number) => ['service-logs', 'detail', id] as const,
  repairs: ['repairs'] as const,
  repairList: (filters: object) => ['repairs', 'list', filters] as const,
  repair: (id: number) => ['repairs', 'detail', id] as const,
  reminders: ['reminders'] as const,
  reminderList: (filters: object) => ['reminders', 'list', filters] as const,
  reminder: (id: number) => ['reminders', 'detail', id] as const,
  dashboard: ['dashboard'] as const,
  reports: ['reports'] as const,
  costReport: (params: object) => ['reports', 'costs', params] as const,
  timeline: (vehicleId: number) => ['reports', 'timeline', vehicleId] as const,
  history: (vehicleId: number, params: object) => ['reports', 'history', vehicleId, params] as const,
  catalogKba: (hsn: string, tsn: string, lang: string) => ['catalog', 'kba', lang, hsn, tsn] as const,
  catalogList: (level: string, parents: readonly string[], q: string) => ['catalog', level, ...parents, q] as const,
  catalogMeta: ['catalog', 'meta'] as const,
  webhooks: ['webhooks'] as const,
  webhook: (id: number) => ['webhooks', 'detail', id] as const,
  deliveries: (id: number, page: number) => ['webhooks', 'deliveries', id, page] as const,
}

/** Keys touched by any write that changes an odometer or the service history. */
export const odometerDependentKeys = [
  queryKeys.vehicles,
  queryKeys.reminders,
  queryKeys.dashboard,
  queryKeys.reports,
] as const
