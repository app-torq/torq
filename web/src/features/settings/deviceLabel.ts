import type { TFunction } from 'i18next'
import type { Session } from '../../lib/types'

/**
 * Names a session in the Devices list: the name an app sent at sign-in, else
 * "Firefox on macOS" from the parsed user agent, else "Unknown device".
 */
export function deviceLabel(s: Pick<Session, 'device_name' | 'browser' | 'os'>, t: TFunction): string {
  if (s.device_name) return s.device_name
  if (s.browser && s.os) return t('settings:devices.browserOn', { browser: s.browser, os: s.os })
  return s.browser || s.os || t('settings:devices.unknown')
}
