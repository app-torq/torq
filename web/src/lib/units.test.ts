import { describe, expect, it } from 'vitest'
import { distanceField, metricField, parseDistance, readDistance, readMetricField } from './format'
import { DISPLACEMENT, DISTANCE, POWER, PRESSURE, QUANTITY, TORQUE, UNIT_PRESETS, fromMetric, kmIn, presetOf, toMetric } from './units'

describe('unit conversion', () => {
  it('round-trips whole numbers in the larger unit through integer storage', () => {
    for (let mi = 0; mi < 300_000; mi += 997) {
      expect(Math.round(fromMetric(toMetric(mi, DISTANCE.mi, true), DISTANCE.mi))).toBe(mi)
    }
    for (let ci = 1; ci < 1000; ci += 7) {
      expect(Math.round(fromMetric(toMetric(ci, DISPLACEMENT.ci, true), DISPLACEMENT.ci))).toBe(ci)
    }
    for (let hp = 1; hp < 2000; hp += 3) {
      expect(Math.round(fromMetric(toMetric(hp, POWER.hp, true), POWER.hp))).toBe(hp)
    }
  })

  it('keeps psi and lb-ft stable at four decimals', () => {
    expect(fromMetric(toMetric(33, PRESSURE.psi, false), PRESSURE.psi).toFixed(1)).toBe('33.0')
    expect(fromMetric(toMetric(88.5, TORQUE.lbft, false), TORQUE.lbft).toFixed(1)).toBe('88.5')
    expect(toMetric(5, QUANTITY.qt, true)).toBe(4732)
  })

  it('parses distances in the vehicle unit', () => {
    expect(parseDistance('12,345', 'mi', 'point')).toBe(19867)
    expect(parseDistance('12345', 'km', 'point')).toBe(12345)
    expect(parseDistance('1.5', 'mi', 'point')).toBeNull()
    expect(parseDistance('-1', 'km', 'point')).toBeNull()
    expect(kmIn(19868, 'mi')).toBe(12345)
  })

  it('keeps the stored value of untouched fields', () => {
    const cc = metricField(1998, DISPLACEMENT.l, 'point')
    expect(cc.text).toBe('2')
    expect(readMetricField('2', cc, DISPLACEMENT.l, 'point', true)).toBe(1998)
    expect(readMetricField('2.5', cc, DISPLACEMENT.l, 'point', true)).toBe(2500)
    expect(readMetricField('x', cc, DISPLACEMENT.l, 'point', true)).toBeUndefined()
    const km = distanceField(15000, 'mi', 'point')
    expect(km.text).toBe('9321')
    expect(readDistance('9321', km, 'mi', 'point')).toBe(15000)
    expect(readDistance('9000', km, 'mi', 'point')).toBe(14484)
  })

  it('recognizes presets', () => {
    expect(presetOf(UNIT_PRESETS.us)).toBe('us')
    expect(presetOf({ ...UNIT_PRESETS.metric, power_unit: 'ps' })).toBeNull()
  })
})
