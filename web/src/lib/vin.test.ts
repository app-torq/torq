import { describe, expect, it } from 'vitest'
import { vinCheckDigitValid, vinWarning } from './vin'

describe('vinWarning', () => {
  it('accepts standard VINs', () => {
    expect(vinWarning('')).toBeNull()
    expect(vinWarning('WDB2032081F123456')).toBeNull()
    expect(vinWarning('1m8gdm9axkp042788')).toBeNull()
  })

  it('flags chassis numbers that are not 17-character VINs', () => {
    expect(vinWarning('JZA80-0012345')).toBe('nonStandard')
    expect(vinWarning('WDB2032081F12345')).toBe('nonStandard')
    expect(vinWarning('WDB2032081O123456')).toBe('nonStandard')
  })

  it('checks the check digit of North American VINs only', () => {
    expect(vinCheckDigitValid('1M8GDM9AXKP042788')).toBe(true)
    expect(vinWarning('1M8GDM9A1KP042788')).toBe('checkDigit')
    expect(vinWarning('WDB2032081F123457')).toBeNull()
  })
})
