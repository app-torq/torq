import { MutationCache, QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createBrowserRouter, Navigate, RouterProvider } from 'react-router'
import '../i18n'
import { AuthGate } from './AuthGate'
import { Layout } from './Layout'
import { RequireAdmin, RequirePermission } from './RequireAdmin'
import { NotFoundPage } from './NotFoundPage'
import { SettingsLayout } from '../features/settings/SettingsLayout'
import { PreferencesPage } from '../features/settings/PreferencesPage'
import { UsersPage } from '../features/settings/UsersPage'
import { RolesPage } from '../features/settings/RolesPage'
import { DevicesPage } from '../features/settings/DevicesPage'
import { WebhooksPage } from '../features/settings/WebhooksPage'
import { InstanceSettingsPage } from '../features/settings/InstanceSettingsPage'
import { AboutPage } from '../features/settings/AboutPage'
import { SecurityPage } from '../features/settings/SecurityPage'
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
import { PrioritiesPage } from '../features/settings/PrioritiesPage'
import { TagsPage } from '../features/settings/TagsPage'
import { RepairBoardPage } from '../features/repairs/RepairBoardPage'
import { RepairDetailPage } from '../features/repairs/RepairDetailPage'
import { ReportsPage } from '../features/reports/ReportsPage'
import { HistoryPrintPage } from '../features/reports/HistoryPrintPage'
import { VehicleQrPrintPage } from '../features/vehicles/VehicleQrPrintPage'
import { markClean } from './pwa/dirty'
import { SharePage, shareToken } from '../features/share/SharePage'

const queryClient = new QueryClient({
  // A successful save leaves nothing unsaved, so updates may apply again.
  mutationCache: new MutationCache({ onSuccess: () => { markClean() } }),
  defaultOptions: { queries: { retry: 1, staleTime: 15_000, refetchOnWindowFocus: true } },
})

const router = createBrowserRouter([
  // The printable history and QR label render without the app chrome.
  { path: '/vehicles/:id/history/print', element: <HistoryPrintPage /> },
  { path: '/vehicles/:id/qr/print', element: <VehicleQrPrintPage /> },
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
          { path: 'security', element: <SecurityPage /> },
          { path: 'password', element: <Navigate to="../security" replace /> },
          { path: 'devices', element: <DevicesPage /> },
          { path: 'categories', element: <RequirePermission permission="categories.manage"><CategoriesPage /></RequirePermission> },
          { path: 'priorities', element: <RequirePermission permission="repair_lists.manage"><PrioritiesPage /></RequirePermission> },
          { path: 'tags', element: <RequirePermission permission="repair_lists.manage"><TagsPage /></RequirePermission> },
          { path: 'users', element: <RequireAdmin><UsersPage /></RequireAdmin> },
          { path: 'roles', element: <RequireAdmin><RolesPage /></RequireAdmin> },
          { path: 'webhooks', element: <RequireAdmin><WebhooksPage /></RequireAdmin> },
          { path: 'instance', element: <RequireAdmin><InstanceSettingsPage /></RequireAdmin> },
          { path: 'about', element: <AboutPage /> },
        ],
      },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
])

export function App() {
  // A public share link needs no account, so it renders outside the sign-in gate.
  const path = window.location.pathname
  if (path.startsWith('/s/')) {
    return (
      <QueryClientProvider client={queryClient}>
        <SharePage token={shareToken(path)} />
      </QueryClientProvider>
    )
  }
  return (
    <QueryClientProvider client={queryClient}>
      <AuthGate>
        <RouterProvider router={router} />
      </AuthGate>
    </QueryClientProvider>
  )
}
