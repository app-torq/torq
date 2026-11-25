import type { ReactNode } from 'react'
import { render } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { I18nextProvider } from 'react-i18next'
import i18n from '../i18n'
import { SessionContext, type Session } from '../app/session'
import { garagePermissions, vehiclePermissions } from '../lib/permissions'

export const testSession: Session = {
  user: {
    id: 1, username: 'marek', display_name: 'Marek', role: 'admin', role_id: 1, role_name: 'Administrator', role_builtin_key: 'admin',
    permissions: [...vehiclePermissions, ...garagePermissions], all_vehicles: true, mfa_enabled: false, mfa_required: false, locale: 'en', date_format: 'YYYY-MM-DD',
    number_format: 'point', time_format: '24h', currency_code: 'EUR', currency_symbol: null, power_unit: 'kw', distance_unit: 'km',
    volume_unit: 'l', torque_unit: 'nm', pressure_unit: 'bar', displacement_unit: 'cc', priority_display: 'both', card_tag_limit: 3, disabled: false,
  },
  instance: {
    api_version: 1, max_upload_mb: 25, version: 'test', demo_mode: false, token_auth: true, spec_autofill: false, public_links: true, passkeys: false,
    branding: { name: '', logo_url: null, contact: { address: '', phone: '', email: '', website: '' } },
  },
}

/** Renders ui with a fresh query client, i18n (English) and a signed-in session. */
export function renderWithApp(ui: ReactNode, session: Session = testSession) {
  void i18n.changeLanguage('en')
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <SessionContext.Provider value={session}>{ui}</SessionContext.Provider>
      </I18nextProvider>
    </QueryClientProvider>,
  )
}
