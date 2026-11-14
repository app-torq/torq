import { useTranslation } from 'react-i18next'
import type { DateFormat, Formats, NumberFormat, TimeFormat } from '../../lib/types'
import { UNIT_PRESETS, presetOf, type UnitChoices, type UnitPreset } from '../../lib/units'
import { SelectField } from '../../components/Field'

const currencies = ['EUR', 'USD', 'GBP', 'CHF'] as const

type FieldErrors = Record<string, string | undefined>

/**
 * Date, time, number and currency selects, shared by the preferences and the
 * instance defaults. errorPrefix maps fields to API error keys ("default_").
 */
export function FormatFields({ value, onChange, errors = {}, errorPrefix = '' }: { value: Formats; onChange: (patch: Partial<Formats>) => void; errors?: FieldErrors; errorPrefix?: string }) {
  const { t } = useTranslation('settings')
  const err = (field: string) => errors[errorPrefix + field]
  const currencyOptions = [...new Set<string>([...currencies, value.currency_code])].map((c) => ({
    value: c,
    label: (currencies as readonly string[]).includes(c) ? t(`preferences.currencies.${c}`) : c,
  }))
  return (
    <>
      <SelectField
        label={t('preferences.dateFormat')}
        value={value.date_format}
        onChange={(v) => { onChange({ date_format: v as DateFormat }) }}
        options={(['YYYY-MM-DD', 'DD.MM.YYYY', 'MM/DD/YYYY'] as const).map((f) => ({ value: f, label: f }))}
        error={err('date_format')}
      />
      <SelectField
        label={t('preferences.timeFormat')}
        value={value.time_format}
        onChange={(v) => { onChange({ time_format: v as TimeFormat }) }}
        options={(['24h', '12h'] as const).map((f) => ({ value: f, label: t(`preferences.timeFormats.${f}`) }))}
        error={err('time_format')}
      />
      <SelectField
        label={t('preferences.numberFormat')}
        value={value.number_format}
        onChange={(v) => { onChange({ number_format: v as NumberFormat }) }}
        options={[
          { value: 'point', label: t('preferences.numberPoint') },
          { value: 'comma', label: t('preferences.numberComma') },
        ]}
        error={err('number_format')}
      />
      <SelectField label={t('preferences.currency')} value={value.currency_code} onChange={(v) => { onChange({ currency_code: v }) }} options={currencyOptions} error={err('currency_code')} />
    </>
  )
}

const unitFields = {
  distance_unit: ['km', 'mi'],
  volume_unit: ['l', 'qt'],
  torque_unit: ['nm', 'lbft'],
  pressure_unit: ['bar', 'psi', 'kpa'],
  displacement_unit: ['cc', 'l', 'ci'],
  power_unit: ['kw', 'ps', 'hp'],
} as const satisfies Record<keyof UnitChoices, readonly string[]>

/** The unit choices from a user or the instance defaults. */
export function unitChoices(f: Formats): UnitChoices {
  return {
    distance_unit: f.distance_unit, volume_unit: f.volume_unit, torque_unit: f.torque_unit,
    pressure_unit: f.pressure_unit, displacement_unit: f.displacement_unit, power_unit: f.power_unit,
  }
}

/** Unit selects with Metric / US / UK presets (preferences, instance defaults, setup). */
export function UnitFields({ value, onChange, errors = {}, errorPrefix = '' }: { value: UnitChoices; onChange: (patch: Partial<UnitChoices>) => void; errors?: FieldErrors; errorPrefix?: string }) {
  const { t } = useTranslation('settings')
  const current = presetOf(value)
  return (
    <div className="space-y-3">
      <div role="group" aria-label={t('preferences.unitPreset')} className="flex flex-wrap items-center gap-2">
        <span className="text-sm text-zinc-400">{t('preferences.unitPreset')}:</span>
        {(Object.keys(UNIT_PRESETS) as UnitPreset[]).map((p) => (
          <button
            key={p}
            type="button"
            aria-pressed={current === p}
            onClick={() => { onChange(UNIT_PRESETS[p]) }}
            className={`focus-ring rounded-md border px-2.5 py-1 text-sm ${current === p ? 'border-sky-500/60 bg-sky-500/15 text-sky-200' : 'border-zinc-700 bg-zinc-900 text-zinc-300 hover:bg-zinc-800'}`}
          >
            {t(`preferences.presets.${p}`)}
          </button>
        ))}
      </div>
      <div className="form-grid grid grid-cols-1 gap-4 sm:grid-cols-2">
        {(Object.keys(unitFields) as (keyof UnitChoices)[]).map((field) => (
          <SelectField
            key={field}
            label={t(`preferences.${field}.label`)}
            value={value[field]}
            onChange={(v) => { onChange({ [field]: v }) }}
            options={unitFields[field].map((u) => ({ value: u, label: t(`preferences.${field}.options.${u}`) }))}
            {...(field === 'distance_unit' ? { hint: t('preferences.distance_unit.hint') } : {})}
            error={errors[errorPrefix + field]}
          />
        ))}
      </div>
    </div>
  )
}
