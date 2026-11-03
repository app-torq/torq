import type { ReactNode } from 'react'
import { render } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { I18nextProvider } from 'react-i18next'
import i18n from '../i18n'
import { SessionContext, type Session } from '../app/session'

export const testSession: Session = {
  user: {
    id: 1, username: 'marek', display_name: 'Marek', role: 'admin', locale: 'en', date_format: 'YYYY-MM-DD',
    number_format: 'point', currency_code: 'EUR', currency_symbol: null, power_unit: 'kw', disabled: false,
  },
  instance: { api_version: 1, max_upload_mb: 25, version: 'test', demo_mode: false, token_auth: true, spec_autofill: false },
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
