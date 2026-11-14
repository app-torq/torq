// Plausibility check for VINs. The server accepts any chassis number of 1–32
// letters, digits or hyphens; this only warns about likely typos.

const standard = /^[A-HJ-NPR-Z0-9]{17}$/
const weights = [8, 7, 6, 5, 4, 3, 2, 10, 0, 9, 8, 7, 6, 5, 4, 3, 2]
const letterValues = 'A1B2C3D4E5F6G7H8J1K2L3M4N5P7R9S2T3U4V5W6X7Y8Z9'

function charValue(c: string): number {
  if (c >= '0' && c <= '9') return Number(c)
  const i = letterValues.indexOf(c)
  return i >= 0 ? Number(letterValues[i + 1]) : 0
}

/** Whether a North American VIN's check digit (position 9) matches. */
export function vinCheckDigitValid(vin: string): boolean {
  let sum = 0
  for (let i = 0; i < 17; i++) sum += charValue(vin.charAt(i)) * (weights[i] ?? 0)
  const r = sum % 11
  return vin.charAt(8) === (r === 10 ? 'X' : String(r))
}

/**
 * Why a VIN looks unusual, or null: not a standard 17-character VIN (fine for
 * JDM chassis numbers and old vehicles), or a North American VIN (first
 * character 1–5) whose check digit does not match.
 */
export function vinWarning(raw: string): 'nonStandard' | 'checkDigit' | null {
  const vin = raw.trim().toUpperCase()
  if (vin === '') return null
  if (!standard.test(vin)) return 'nonStandard'
  if (/^[1-5]/.test(vin) && !vinCheckDigitValid(vin)) return 'checkDigit'
  return null
}
