import { useQuery } from '@tanstack/react-query'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { ListResponse, Reminder } from '../../lib/types'

export function useReminders(filters: { vehicle_id?: number; include_paused?: boolean }) {
  return useQuery({
    queryKey: queryKeys.reminderList(filters),
    queryFn: () =>
      api.get<ListResponse<Reminder>>('/reminders', {
        vehicle_id: filters.vehicle_id,
        include_paused: filters.include_paused ? 'true' : undefined,
      }),
  })
}
