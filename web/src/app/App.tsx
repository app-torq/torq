import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createBrowserRouter, Navigate, RouterProvider } from 'react-router'
import '../i18n'
import { AuthGate } from './AuthGate'
import { Layout } from './Layout'
import { RequireAdmin } from './RequireAdmin'
import { NotFoundPage } from './NotFoundPage'
import { SettingsLayout } from '../features/settings/SettingsLayout'
import { PreferencesPage } from '../features/settings/PreferencesPage'
import { UsersPage } from '../features/settings/UsersPage'
import { WebhooksPage } from '../features/settings/WebhooksPage'
import { InstanceSettingsPage } from '../features/settings/InstanceSettingsPage'
import { ChangePasswordPage } from '../features/auth/ChangePasswordPage'
import { DashboardPage } from '../features/dashboard/DashboardPage'
import { VehicleListPage } from '../features/vehicles/VehicleListPage'
import { VehicleFormPage } from '../features/vehicles/VehicleFormPage'
import { VehicleDetailLayout } from '../features/vehicles/VehicleDetailLayout'
import { vehicleChildRoutes } from './vehicleRoutes'
import { ServiceLogListPage } from '../features/service-log/ServiceLogListPage'
import { ServiceLogFormPage } from '../features/service-log/ServiceLogForm'
import { ServiceLogDetailPage } from '../features/service-log/ServiceLogDetailPage'
import { ReminderListPage } from '../features/reminders/ReminderListPage'
import { CategoriesPage } from '../features/settings/CategoriesPage'
import { RepairBoardPage } from '../features/repairs/RepairBoardPage'
import { RepairDetailPage } from '../features/repairs/RepairDetailPage'
import { ReportsPage } from '../features/reports/ReportsPage'
import { HistoryPrintPage } from '../features/reports/HistoryPrintPage'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 15_000, refetchOnWindowFocus: true } },
})

const router = createBrowserRouter([
  // The printable history renders without the app chrome.
  { path: '/vehicles/:id/history/print', element: <HistoryPrintPage /> },
  {
    path: '/',
    element: <Layout />,
    children: [
      { index: true, element: <DashboardPage /> },
      { path: 'vehicles', element: <VehicleListPage /> },
      { path: 'vehicles/new', element: <VehicleFormPage /> },
      { path: 'vehicles/:id/edit', element: <VehicleFormPage /> },
      {
        path: 'vehicles/:id',
        element: <VehicleDetailLayout />,
        children: vehicleChildRoutes,
      },
      { path: 'service', element: <ServiceLogListPage /> },
      { path: 'service/new', element: <ServiceLogFormPage /> },
      { path: 'service/:id', element: <ServiceLogDetailPage /> },
      { path: 'service/:id/edit', element: <ServiceLogFormPage /> },
      { path: 'reminders', element: <ReminderListPage /> },
      { path: 'repairs', element: <RepairBoardPage /> },
      { path: 'repairs/:id', element: <RepairDetailPage /> },
      { path: 'reports', element: <ReportsPage /> },
      {
        path: 'settings',
        element: <SettingsLayout />,
        children: [
          { index: true, element: <Navigate to="preferences" replace /> },
          { path: 'preferences', element: <PreferencesPage /> },
          { path: 'password', element: <ChangePasswordPage /> },
          { path: 'categories', element: <CategoriesPage /> },
          { path: 'users', element: <RequireAdmin><UsersPage /></RequireAdmin> },
          { path: 'webhooks', element: <RequireAdmin><WebhooksPage /></RequireAdmin> },
          { path: 'instance', element: <RequireAdmin><InstanceSettingsPage /></RequireAdmin> },
        ],
      },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
])

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthGate>
        <RouterProvider router={router} />
      </AuthGate>
    </QueryClientProvider>
  )
}
