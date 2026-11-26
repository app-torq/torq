import type { RouteObject } from 'react-router'
import { VehicleOverview } from '../features/vehicles/VehicleOverview'
import { SpecSheetPage } from '../features/vehicles/SpecSheet'
import { useVehicleContext } from '../features/vehicles/VehicleDetailLayout'
import { ServiceLogListPage } from '../features/service-log/ServiceLogListPage'
import { ReminderListPage } from '../features/reminders/ReminderListPage'
import { RepairBoardPage } from '../features/repairs/RepairBoardPage'
import { TimelinePage } from '../features/reports/TimelinePage'
import { RequirePermission } from './RequireAdmin'

function VehicleServiceTab() {
  return <ServiceLogListPage vehicleId={useVehicleContext().id} />
}

function VehicleRepairsTab() {
  return <RepairBoardPage vehicleId={useVehicleContext().id} />
}

function VehicleRemindersTab() {
  return <ReminderListPage vehicleId={useVehicleContext().id} />
}

/** Tabs under /vehicles/:id (see features/vehicles/tabs.ts). */
export const vehicleChildRoutes: RouteObject[] = [
  { index: true, element: <VehicleOverview /> },
  { path: 'specs', element: <SpecSheetPage /> },
  { path: 'service', element: <RequirePermission permission="service_logs.view"><VehicleServiceTab /></RequirePermission> },
  { path: 'reminders', element: <RequirePermission permission="reminders.view"><VehicleRemindersTab /></RequirePermission> },
  { path: 'repairs', element: <RequirePermission permission="repairs.view"><VehicleRepairsTab /></RequirePermission> },
  { path: 'history', element: <RequirePermission permission="reports.view"><TimelinePage /></RequirePermission> },
]
