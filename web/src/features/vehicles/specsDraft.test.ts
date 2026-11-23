import { describe, expect, it } from 'vitest'
import type { AutofillValue, VehicleSpecs } from '../../lib/types'
import { applyAutofill, editField, editMoto, initialDraft, rangeMeta, toRequestBody } from './specsDraft'

const empty: VehicleSpecs = {
  engine_oil_viscosity: '', engine_oil_spec: '', engine_oil_capacity_ml: null,
  transmission_fluid_spec: '', transmission_fluid_capacity_ml: null,
  differential_fluid_spec: '', differential_fluid_capacity_ml: null,
  brake_fluid_spec: '', brake_fluid_capacity_ml: null,
  coolant_spec: '', coolant_capacity_ml: null,
  wheel_lug_torque_nm: null, tire_pressure_front_bar: null, tire_pressure_rear_bar: null,
  tire_pressure_front_max_bar: null, tire_pressure_rear_max_bar: null,
  meta: {},
  motorcycle: {
    final_drive: null, sprocket_front_teeth: null, sprocket_rear_teeth: null, chain_size: '', chain_links: null,
    fork_oil_spec: '', fork_oil_capacity_ml: null, valve_clearance_intake_min_mm: null, valve_clearance_intake_max_mm: null,
    valve_clearance_exhaust_min_mm: null, valve_clearance_exhaust_max_mm: null,
  },
}

const found = (value: string | number, name = 'carwiki.de'): AutofillValue => ({ value, source_url: `https://${name}/page`, source_name: name })
const none: AutofillValue = { value: null, source_url: null, source_name: null }

describe('spec auto-fill draft', () => {
  it('fills empty fields with sources, converts units and keeps filled ones', () => {
    const start = initialDraft({ ...empty, engine_oil_spec: 'MB 229.31' }, 'comma')
    const { draft, summary } = applyAutofill(start, {
      engine_oil_viscosity: found('5W-30'),
      engine_oil_spec: found('MB 229.51'),
      engine_oil_capacity_l: found(6.5, 'liqui-moly.com'),
      tire_pressure_front_bar: found(2.3, 'adac.de'),
      differential_fluid_spec: none,
    }, false, 'comma')

    expect(draft.values.engine_oil_viscosity).toBe('5W-30')
    expect(draft.values.engine_oil_capacity_ml).toBe('6,5')
    expect(draft.values.tire_pressure_front_bar).toBe('2,3')
    expect(draft.values.engine_oil_spec).toBe('MB 229.31')
    expect(draft.meta.engine_oil_capacity_ml).toEqual({ is_auto: true, source_url: 'https://liqui-moly.com/page', source_name: 'liqui-moly.com' })
    expect(draft.meta.engine_oil_spec).toBeUndefined()
    expect(summary.filled).toEqual(['engine_oil_viscosity', 'engine_oil_capacity_ml', 'tire_pressure_front_bar'])
    expect(summary.kept).toEqual(['engine_oil_spec'])
    expect(summary.missing).toHaveLength(10)

    const replaced = applyAutofill(start, { engine_oil_spec: found('MB 229.51') }, true, 'comma').draft
    expect(replaced.values.engine_oil_spec).toBe('MB 229.51')
    expect(replaced.meta.engine_oil_spec?.is_auto).toBe(true)
  })

  it('marks only the edited field as manual and restores the source when the found value is typed back', () => {
    let d = applyAutofill(initialDraft(empty, 'point'), {
      engine_oil_viscosity: found('5W-30'), coolant_spec: found('MB 325.0'),
    }, false, 'point').draft

    d = editField(d, 'engine_oil_viscosity', '0W-30')
    expect(d.meta.engine_oil_viscosity).toEqual({ is_auto: false })
    expect(d.meta.coolant_spec?.is_auto).toBe(true)

    d = editField(d, 'engine_oil_viscosity', '5W-30')
    expect(d.meta.engine_oil_viscosity?.is_auto).toBe(true)

    d = editField(d, 'engine_oil_viscosity', '')
    expect(d.meta.engine_oil_viscosity).toBeUndefined()

    // Fields that were never auto-filled carry no metadata.
    d = editField(d, 'brake_fluid_spec', 'DOT 4')
    expect(d.meta.brake_fluid_spec).toBeUndefined()
  })

  it('keeps saved sources when the dialog opens again and sends values plus metadata', () => {
    const saved: VehicleSpecs = {
      ...empty,
      engine_oil_capacity_ml: 6500,
      wheel_lug_torque_nm: 110,
      meta: {
        engine_oil_capacity_ml: { is_auto: true, source_url: 'https://carwiki.de/x', source_name: 'carwiki.de' },
        wheel_lug_torque_nm: { is_auto: false },
      },
    }
    let d = initialDraft(saved, 'point')
    expect(d.values.engine_oil_capacity_ml).toBe('6.5')
    d = editField(d, 'engine_oil_capacity_ml', '6.0')
    expect(d.meta.engine_oil_capacity_ml).toEqual({ is_auto: false })
    d = editField(d, 'engine_oil_capacity_ml', '6.5')
    expect(d.meta.engine_oil_capacity_ml?.is_auto).toBe(true)

    const { body, errors } = toRequestBody(d, 'point')
    expect(errors).toEqual({})
    expect(body.engine_oil_capacity_ml).toBe(6500)
    expect(body.wheel_lug_torque_nm).toBe(110)
    expect(body.coolant_capacity_ml).toBeNull()
    expect(body.meta).toEqual({
      engine_oil_capacity_ml: { is_auto: true, source_url: 'https://carwiki.de/x', source_name: 'carwiki.de' },
      wheel_lug_torque_nm: { is_auto: false },
    })

    expect(toRequestBody(editField(d, 'tire_pressure_rear_bar', 'abc'), 'point').errors).toEqual({ tire_pressure_rear_bar: 'validation.number' })
  })

  it('fills tyre pressure ranges and replaces an old range with a single value', () => {
    const range = (lo: number, hi: number | null): AutofillValue => ({ ...found(lo, 'tpressure.com'), value_max: hi })
    let d = applyAutofill(initialDraft(empty, 'comma'), {
      tire_pressure_front_bar: range(2.2, 2.4),
      tire_pressure_rear_bar: range(2.2, 2.9),
    }, false, 'comma').draft
    expect([d.values.tire_pressure_front_bar, d.values.tire_pressure_front_max_bar]).toEqual(['2,2', '2,4'])
    expect(d.meta.tire_pressure_front_max_bar?.is_auto).toBe(true)
    expect(rangeMeta(d, 'tire_pressure_front_bar', 'tire_pressure_front_max_bar')?.source_name).toBe('tpressure.com')

    // Changing one end marks the range as adjusted by hand.
    d = editField(d, 'tire_pressure_front_max_bar', '2,5')
    expect(rangeMeta(d, 'tire_pressure_front_bar', 'tire_pressure_front_max_bar')).toEqual({ is_auto: false })

    const single = applyAutofill(d, { tire_pressure_rear_bar: range(2.5, null) }, true, 'comma')
    expect(single.draft.values.tire_pressure_rear_max_bar).toBe('')
    expect(single.draft.meta.tire_pressure_rear_max_bar).toBeUndefined()
    // The maxima are no separate lookup fields.
    expect(single.summary.missing).not.toContain('tire_pressure_front_max_bar')

    const { body } = toRequestBody(d, 'comma')
    expect(body).toMatchObject({ tire_pressure_front_bar: 2.2, tire_pressure_front_max_bar: 2.5, tire_pressure_rear_max_bar: 2.9 })
  })
})

describe('specs draft in US units', () => {
  const us = { volume: 'qt', torque: 'lbft', pressure: 'psi' } as const
  it('shows, keeps and converts values', () => {
    const saved = { ...empty, engine_oil_capacity_ml: 6500, wheel_lug_torque_nm: 120, tire_pressure_front_bar: 2.3 }
    const d = initialDraft(saved, 'point', us)
    expect(d.values.engine_oil_capacity_ml).toBe('6.87')
    expect(d.values.wheel_lug_torque_nm).toBe('88.5')
    expect(d.values.tire_pressure_front_bar).toBe('33.4')
    // Untouched fields are saved exactly as stored.
    let { body } = toRequestBody(d, 'point', us)
    expect(body.engine_oil_capacity_ml).toBe(6500)
    expect(body.wheel_lug_torque_nm).toBe(120)
    expect(body.tire_pressure_front_bar).toBe(2.3)
    ;({ body } = toRequestBody(editField(d, 'tire_pressure_front_bar', '35'), 'point', us))
    expect(body.tire_pressure_front_bar).toBe(2.4132)
  })

  it('converts auto-filled litres', () => {
    const { draft } = applyAutofill(initialDraft(empty, 'point', us), { engine_oil_capacity_l: found(6.5) }, true, 'point', us)
    expect(draft.values.engine_oil_capacity_ml).toBe('6.87')
  })
})

describe('motorcycle fields', () => {
  const bike: VehicleSpecs = {
    ...empty,
    motorcycle: {
      ...empty.motorcycle, final_drive: 'chain', sprocket_front_teeth: 15, chain_size: '525', fork_oil_capacity_ml: 491,
      valve_clearance_intake_min_mm: 0.1, valve_clearance_intake_max_mm: 0.15,
    },
  }

  it('shows decimals in the number format and fork oil in the volume unit', () => {
    const d = initialDraft(bike, 'comma', { volume: 'qt', torque: 'nm', pressure: 'bar' })
    expect(d.moto.valve_clearance_intake_min_mm).toBe('0,1')
    expect(d.moto.sprocket_front_teeth).toBe('15')
    expect(d.moto.fork_oil_capacity_ml).not.toBe('491')
    // Untouched, the stored mL go back unchanged; nothing is lost by the round trip.
    const { body, errors } = toRequestBody(d, 'comma', { volume: 'qt', torque: 'nm', pressure: 'bar' })
    expect(errors).toEqual({})
    expect(body.motorcycle).toMatchObject({
      final_drive: 'chain', sprocket_front_teeth: 15, chain_size: '525', fork_oil_capacity_ml: 491,
      valve_clearance_intake_min_mm: 0.1, valve_clearance_intake_max_mm: 0.15, chain_links: null,
    })
  })

  it('parses edits and reports errors with the server keys', () => {
    let d = initialDraft(bike, 'comma')
    d = editMoto(d, 'valve_clearance_exhaust_min_mm', '0,2')
    d = editMoto(d, 'fork_oil_capacity_ml', '0,5')
    d = editMoto(d, 'final_drive', '')
    expect(toRequestBody(d, 'comma').body.motorcycle).toMatchObject({ valve_clearance_exhaust_min_mm: 0.2, fork_oil_capacity_ml: 500, final_drive: null })

    d = editMoto(d, 'chain_links', '112,5')
    d = editMoto(d, 'valve_clearance_exhaust_max_mm', 'abc')
    expect(toRequestBody(d, 'comma').errors).toEqual({
      'motorcycle.chain_links': 'validation.number', 'motorcycle.valve_clearance_exhaust_max_mm': 'validation.number',
    })
  })
})
