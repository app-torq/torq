import { useState, type SubmitEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { Formats, Locale, MeResponse, PriorityDisplay, User } from '../../lib/types'
import { createFormatter } from '../../lib/format'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { useSession } from '../../app/session'
import { Button } from '../../components/Button'
import { SelectField, TextField } from '../../components/Field'
import { Card, ErrorBanner, Notice, PageHeader } from '../../components/ui'
import { FormatFields, UnitFields, unitChoices } from './FormatFields'

/** The format and unit fields of a user. */
function formatsOf(u: User): Formats {
  return {
    date_format: u.date_format, number_format: u.number_format, time_format: u.time_format, currency_code: u.currency_code,
    power_unit: u.power_unit, distance_unit: u.distance_unit, volume_unit: u.volume_unit, torque_unit: u.torque_unit,
    pressure_unit: u.pressure_unit, displacement_unit: u.displacement_unit,
  }
}

export function PreferencesPage() {
  const { t, i18n } = useTranslation(['settings', 'common'])
  const { user } = useSession()
  const qc = useQueryClient()
  const errorText = useErrorText()

  const [displayName, setDisplayName] = useState(user.display_name)
  const [locale, setLocale] = useState<Locale>(user.locale)
  const [formats, setFormats] = useState<Formats>(() => formatsOf(user))
  const [currencySymbol, setCurrencySymbol] = useState(user.currency_symbol ?? '')
  const [priorityDisplay, setPriorityDisplay] = useState<PriorityDisplay>(user.priority_display)
  const patch = (p: Partial<Formats>) => { setFormats((prev) => ({ ...prev, ...p })) }

  const save = useMutation({
    mutationFn: () =>
      api.patch<{ user: User }>('/me/preferences', { display_name: displayName, locale, currency_symbol: currencySymbol, priority_display: priorityDisplay, ...formats }),
    onSuccess: ({ user: updated }) => {
      qc.setQueryData<MeResponse | null>(queryKeys.me, (prev) => (prev ? { ...prev, user: updated } : prev))
      void i18n.changeLanguage(updated.locale)
    },
  })
  const fe = fieldErrors(save.error)

  const preview = createFormatter(
    {
      dateFormat: formats.date_format,
      numberFormat: formats.number_format,
      currencyCode: /^[A-Z]{3}$/.test(formats.currency_code) ? formats.currency_code : 'EUR',
      currencySymbol: currencySymbol.trim() || null,
      powerUnit: formats.power_unit,
      volumeUnit: formats.volume_unit,
      torqueUnit: formats.torque_unit,
      pressureUnit: formats.pressure_unit,
      displacementUnit: formats.displacement_unit,
    },
    (u) => t(`common:units.${u}`),
  )

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    save.mutate()
  }

  return (
    <div className="max-w-2xl">
      <PageHeader title={t('preferences.title')} subtitle={t('preferences.intro')} />
      <Card>
        <form onSubmit={submit} className="space-y-4" noValidate>
          {save.isSuccess && <Notice tone="emerald">{t('common:state.saved')}</Notice>}
          {save.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(save.error)} />}
          <TextField label={t('auth:fields.displayName')} value={displayName} onChange={setDisplayName} error={fe.display_name} />
          <div className="form-grid grid grid-cols-1 gap-4 sm:grid-cols-2">
            <SelectField
              label={t('preferences.language')}
              value={locale}
              onChange={(v) => { setLocale(v as Locale) }}
              options={[
                { value: 'en', label: t('preferences.languages.en') },
                { value: 'de', label: t('preferences.languages.de') },
              ]}
              error={fe.locale}
            />
            <FormatFields value={formats} onChange={patch} errors={fe} />
          </div>
          <TextField
            label={t('preferences.currencySymbol')}
            value={currencySymbol}
            onChange={setCurrencySymbol}
            maxLength={5}
            optional
            hint={t('preferences.currencySymbolHint')}
            error={fe.currency_symbol}
          />
          <SelectField
            label={t('preferences.priorityDisplay')}
            value={priorityDisplay}
            onChange={(v) => { setPriorityDisplay(v as PriorityDisplay) }}
            options={(['both', 'color', 'name'] as const).map((d) => ({ value: d, label: t(`preferences.priorityDisplays.${d}`) }))}
            hint={t('preferences.priorityDisplayHint')}
            error={fe.priority_display}
          />
          <fieldset className="space-y-2 border-t border-zinc-800 pt-4">
            <legend className="text-sm font-semibold text-zinc-200">{t('preferences.units')}</legend>
            <p className="text-xs text-zinc-400">{t('preferences.unitsHint')}</p>
            <UnitFields value={unitChoices(formats)} onChange={patch} errors={fe} />
          </fieldset>
          <div className="rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm">
            <span className="text-zinc-400">{t('preferences.preview')}: </span>
            <span className="tabular-nums text-zinc-200">
              {[
                preview.date('2026-10-15'),
                preview.distance(215000, formats.distance_unit),
                preview.volume(6500),
                preview.money(8950),
                preview.power(110000),
                preview.torque(120),
                preview.pressure(2.3),
              ].join(' · ')}
            </span>
          </div>
          <Button type="submit" variant="primary" disabled={save.isPending}>
            {save.isPending ? t('common:actions.saving') : t('common:actions.save')}
          </Button>
        </form>
      </Card>
    </div>
  )
}
