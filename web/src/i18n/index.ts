import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

const modules = import.meta.glob<{ default: Record<string, unknown> }>('./locales/*/*.json', { eager: true })

type Resources = Record<string, Record<string, Record<string, unknown>>>

const resources: Resources = {}
for (const [path, mod] of Object.entries(modules)) {
  const m = /\.\/locales\/(\w+)\/(\w+)\.json$/.exec(path)
  if (!m?.[1] || !m[2]) continue
  const lng = m[1]
  const ns = m[2]
  resources[lng] ??= {}
  resources[lng][ns] = mod.default
}

export const namespaces = [
  'common',
  'auth',
  'vehicles',
  'specs',
  'service',
  'repairs',
  'reminders',
  'reports',
  'settings',
  'webhooks',
  'errors',
] as const

void i18n.use(initReactI18next).init({
  resources,
  lng: 'en',
  fallbackLng: 'en',
  ns: [...namespaces],
  defaultNS: 'common',
  returnNull: false,
  interpolation: { escapeValue: false },
})

export default i18n
