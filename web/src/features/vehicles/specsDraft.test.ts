import { describe, expect, it } from 'vitest'
import type { AutofillValue, VehicleSpecs } from '../../lib/types'
import { applyAutofill, editField, initialDraft, toRequestBody } from './specsDraft'

const empty: VehicleSpecs = {
  engine_oil_viscosity: '', engine_oil_spec: '', engine_oil_capacity_ml: null,
  transmission_fluid_spec: '', transmission_fluid_capacity_ml: null,
  differential_fluid_spec: '', differential_fluid_capacity_ml: null,
  brake_fluid_spec: '', brake_fluid_capacity_ml: null,
  coolant_spec: '', coolant_capacity_ml: null,
  wheel_lug_torque_nm: null, tire_pressure_front_bar: null, tire_pressure_rear_bar: null,
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
})
