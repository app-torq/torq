import type { TFunction } from 'i18next'
import type { Formatter } from '../../lib/format'

/** Spec sheet values keyed like the history export and the public record. */
export type SpecValues = Record<string, string | number | null>

const num = (s: SpecValues, k: string) => (typeof s[k] === 'number' ? s[k] : null)
const text = (s: SpecValues, k: string) => (typeof s[k] === 'string' && s[k] !== '' ? s[k] : null)

/** Drivetrain, fork oil and valve clearance of a motorcycle (#28), in the reader's units. */
export function motorcycleSpecLines(s: SpecValues, fmt: Formatter, t: TFunction): string[] {
  const n = (k: string) => num(s, k)
  const mm = (lo: number | null, hi: number | null) =>
    lo === null ? null : `${[lo, hi].filter((x): x is number => x !== null).map((x) => fmt.number(x, { decimals: 2 })).join('–')} mm`
  const drive = text(s, 'final_drive')
  const chain = !drive || drive === 'chain'
  const front = n('sprocket_front_teeth')
  const rear = n('sprocket_rear_teeth')
  const links = n('chain_links')
  const forkOil = n('fork_oil_capacity_ml')
  const intake = mm(n('valve_clearance_intake_min_mm'), n('valve_clearance_intake_max_mm'))
  const exhaust = mm(n('valve_clearance_exhaust_min_mm'), n('valve_clearance_exhaust_max_mm'))
  return [
    drive ? `${t('specs:fields.finalDrive')}: ${t(`specs:finalDrive.${drive}`)}` : null,
    chain && (front !== null || rear !== null) ? `${t('specs:fields.sprockets')}: ${front === null ? '—' : String(front)} / ${rear === null ? '—' : String(rear)}` : null,
    chain && (text(s, 'chain_size') || links !== null)
      ? `${t('specs:fields.chain')}: ${[text(s, 'chain_size'), links !== null ? t('specs:fields.chainLinksValue', { count: links }) : null].filter(Boolean).join(' · ')}`
      : null,
    text(s, 'fork_oil_spec') || forkOil !== null
      ? `${t('specs:groups.forkOil')}: ${[text(s, 'fork_oil_spec'), forkOil !== null ? t('specs:fields.perLeg', { volume: fmt.volume(forkOil) }) : null].filter(Boolean).join(' · ')}`
      : null,
    intake ? `${t('specs:groups.valveClearance')}, ${t('specs:fields.intake')}: ${intake}` : null,
    exhaust ? `${t('specs:groups.valveClearance')}, ${t('specs:fields.exhaust')}: ${exhaust}` : null,
  ].filter((l): l is string => l !== null)
}

/**
 * Every filled spec value as "Label: value" lines: fluids, torque, tyre
 * pressure, and for motorcycles their own values (the differential is the
 * final drive oil there, shown for shaft drive only).
 */
export function specLines(s: SpecValues, motorcycle: boolean, fmt: Formatter, t: TFunction): string[] {
  const n = (k: string) => num(s, k)
  const fluid = (label: string, spec: string | null, ml: number | null) =>
    spec || ml !== null ? `${label}: ${[spec, ml !== null ? fmt.volume(ml) : null].filter(Boolean).join(' · ')}` : null
  const pressure = (label: string, lo: number | null, hi: number | null) => (lo !== null ? `${label}: ${fmt.pressureRange(lo, hi)}` : null)
  const shaft = text(s, 'final_drive') === 'shaft'
  return [
    fluid(t('specs:groups.engineOil'), [text(s, 'engine_oil_viscosity'), text(s, 'engine_oil_spec')].filter(Boolean).join(' · ') || null, n('engine_oil_capacity_ml')),
    fluid(t('specs:groups.transmission'), text(s, 'transmission_fluid_spec'), n('transmission_fluid_capacity_ml')),
    !motorcycle || shaft
      ? fluid(t(motorcycle ? 'specs:groups.finalDriveOil' : 'specs:groups.differential'), text(s, 'differential_fluid_spec'), n('differential_fluid_capacity_ml'))
      : null,
    fluid(t('specs:groups.brake'), text(s, 'brake_fluid_spec'), n('brake_fluid_capacity_ml')),
    fluid(t('specs:groups.coolant'), text(s, 'coolant_spec'), n('coolant_capacity_ml')),
    !motorcycle && n('wheel_lug_torque_nm') !== null ? `${t('specs:fields.lugTorque')}: ${fmt.torque(n('wheel_lug_torque_nm') ?? 0)}` : null,
    pressure(t('specs:fields.pressureFront'), n('tire_pressure_front_bar'), n('tire_pressure_front_max_bar')),
    pressure(t('specs:fields.pressureRear'), n('tire_pressure_rear_bar'), n('tire_pressure_rear_max_bar')),
    ...(motorcycle ? motorcycleSpecLines(s, fmt, t) : []),
  ].filter((l): l is string => l !== null)
}
