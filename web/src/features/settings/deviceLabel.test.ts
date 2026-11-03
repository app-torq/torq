import { describe, expect, it } from 'vitest'
import i18n from '../../i18n'
import { deviceLabel } from './deviceLabel'

const session = { device_name: '', browser: '', os: '' }

describe('deviceLabel', () => {
  const en = i18n.getFixedT('en')
  it('prefers the name the app sent', () => {
    expect(deviceLabel({ ...session, device_name: "Marek's iPhone", browser: 'torq', os: 'iOS' }, en)).toBe("Marek's iPhone")
  })
  it('names a browser by browser and system, localized', () => {
    expect(deviceLabel({ ...session, browser: 'Firefox', os: 'macOS' }, en)).toBe('Firefox on macOS')
    expect(deviceLabel({ ...session, browser: 'Firefox', os: 'macOS' }, i18n.getFixedT('de'))).toBe('Firefox unter macOS')
  })
  it('falls back to whatever is known', () => {
    expect(deviceLabel({ ...session, os: 'Linux' }, en)).toBe('Linux')
    expect(deviceLabel({ ...session, browser: 'Safari' }, en)).toBe('Safari')
    expect(deviceLabel(session, en)).toBe('Unknown device')
  })
})
