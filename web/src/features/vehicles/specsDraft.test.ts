import { describe, expect, it } from 'vitest'
import type { AutofillValue, VehicleSpecs } from '../../lib/types'
import { applyAutofill, editField, initialDraft, rangeMeta, toRequestBody } from './specsDraft'

const empty: VehicleSpecs = {
  engine_oil_viscosity: '', engine_oil_spec: '', engine_oil_capacity_ml: null,
  transmission_fluid_spec: '', transmission_fluid_capacity_ml: null,
  differential_fluid_spec: '', differential_fluid_capacity_ml: null,
  brake_fluid_spec: '', brake_fluid_capacity_ml: null,
  coolant_spec: '', coolant_capacity_ml: null,
  wheel_lug_torque_nm: null, tire_pressure_front_bar: null, tire_pressure_rear_bar: null,
  tire_pressure_front_max_bar: null, tire_pressure_rear_max_bar: null,
  meta: {},
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
