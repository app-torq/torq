// Display units (Constitution 3.0.0, "Metric storage"). Values are stored and
// sent in metric units: km, mL, Nm, bar, cm³ and W. These helpers convert
// only where a value is shown or typed.

export type DistanceUnit = 'km' | 'mi'
/** Capacities: litres or US quarts. */
export type VolumeUnit = 'l' | 'qt'
export type TorqueUnit = 'nm' | 'lbft'
export type PressureUnit = 'bar' | 'psi' | 'kpa'
export type DisplacementUnit = 'cc' | 'l' | 'ci'
export type PowerUnit = 'kw' | 'ps' | 'hp'
/** The unit a service item's quantity was entered in. */
export type QuantityUnit = 'L' | 'mL' | 'qt' | 'gal' | 'fl_oz'

/** Keys under common:units. */
export type UnitKey = 'km' | 'mi' | 'l' | 'ml' | 'qt' | 'gal' | 'fl_oz' | 'nm' | 'lbft' | 'bar' | 'psi' | 'kpa' | 'cc' | 'ci' | 'kw' | 'ps' | 'hp'

/** A display unit: how many stored metric units one of it is, and its decimals. */
export interface UnitSpec {
  /** Stored metric units per display unit, e.g. 1.609344 km per mile. */
  factor: number
  /** Decimals shown and accepted. */
  decimals: number
  label: UnitKey
}

export const DISTANCE: Record<DistanceUnit, UnitSpec> = {
  km: { factor: 1, decimals: 0, label: 'km' },
  mi: { factor: 1.609344, decimals: 0, label: 'mi' },
}

/** Capacities, stored in mL. */
export const VOLUME: Record<VolumeUnit, UnitSpec> = {
  l: { factor: 1000, decimals: 3, label: 'l' },
  qt: { factor: 946.352946, decimals: 2, label: 'qt' },
}

/** Service item quantities, stored in mL. */
export const QUANTITY: Record<QuantityUnit, UnitSpec> = {
  L: { factor: 1000, decimals: 3, label: 'l' },
  mL: { factor: 1, decimals: 0, label: 'ml' },
  qt: { factor: 946.352946, decimals: 2, label: 'qt' },
  gal: { factor: 3785.411784, decimals: 2, label: 'gal' },
  fl_oz: { factor: 29.5735295625, decimals: 1, label: 'fl_oz' },
}

export const TORQUE: Record<TorqueUnit, UnitSpec> = {
  nm: { factor: 1, decimals: 1, label: 'nm' },
  lbft: { factor: 1.3558179483, decimals: 1, label: 'lbft' },
}

export const PRESSURE: Record<PressureUnit, UnitSpec> = {
  bar: { factor: 1, decimals: 2, label: 'bar' },
  psi: { factor: 0.0689475729, decimals: 1, label: 'psi' },
  kpa: { factor: 0.01, decimals: 0, label: 'kpa' },
}

export const DISPLACEMENT: Record<DisplacementUnit, UnitSpec> = {
  cc: { factor: 1, decimals: 0, label: 'cc' },
  l: { factor: 1000, decimals: 1, label: 'l' },
  ci: { factor: 16.387064, decimals: 0, label: 'ci' },
}

/** Power, stored in W. */
export const POWER: Record<PowerUnit, UnitSpec> = {
  kw: { factor: 1000, decimals: 0, label: 'kw' },
  ps: { factor: 735.49875, decimals: 0, label: 'ps' },
  hp: { factor: 745.69987158, decimals: 0, label: 'hp' },
}

/** Quantity units offered per volume preference, large unit first. */
export const QUANTITY_UNITS: Record<VolumeUnit, QuantityUnit[]> = {
  l: ['L', 'mL'],
  qt: ['qt', 'gal', 'fl_oz'],
}

/** A distance in km as a whole number in unit, e.g. for placeholders. */
export function kmIn(km: number, unit: DistanceUnit): number {
  return Math.round(km / DISTANCE[unit].factor)
}

/** Converts a stored metric value to the display unit (unrounded). */
export function fromMetric(value: number, spec: UnitSpec): number {
  return value / spec.factor
}

/**
 * Converts a value in the display unit to the stored metric value: whole
 * numbers for integer columns (km, mL, cm³, W), otherwise four decimals,
 * which keeps a value typed in psi or lb-ft stable when it is read back.
 */
export function toMetric(value: number, spec: UnitSpec, integer: boolean): number {
  const v = value * spec.factor
  return integer ? Math.round(v) : Math.round(v * 10_000) / 10_000
}

/** Unit choices of the presets offered in the preferences and at setup. */
export interface UnitChoices {
  distance_unit: DistanceUnit
  volume_unit: VolumeUnit
  torque_unit: TorqueUnit
  pressure_unit: PressureUnit
  displacement_unit: DisplacementUnit
  power_unit: PowerUnit
}

export type UnitPreset = 'metric' | 'us' | 'uk'

export const UNIT_PRESETS: Record<UnitPreset, UnitChoices> = {
  metric: { distance_unit: 'km', volume_unit: 'l', torque_unit: 'nm', pressure_unit: 'bar', displacement_unit: 'cc', power_unit: 'kw' },
  us: { distance_unit: 'mi', volume_unit: 'qt', torque_unit: 'lbft', pressure_unit: 'psi', displacement_unit: 'l', power_unit: 'hp' },
  uk: { distance_unit: 'mi', volume_unit: 'l', torque_unit: 'nm', pressure_unit: 'psi', displacement_unit: 'cc', power_unit: 'hp' },
}

/** The preset whose choices equal c, if any. */
export function presetOf(c: UnitChoices): UnitPreset | null {
  for (const [name, p] of Object.entries(UNIT_PRESETS) as [UnitPreset, UnitChoices][]) {
    if ((Object.keys(p) as (keyof UnitChoices)[]).every((k) => p[k] === c[k])) return name
  }
  return null
}
