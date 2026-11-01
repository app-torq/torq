import { useState, type SubmitEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import type { DateFormat, Locale, MeResponse, NumberFormat, PowerUnit, User } from '../../lib/types'
import { createFormatter } from '../../lib/format'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { useSession } from '../../app/session'
import { Button } from '../../components/Button'
import { SelectField, TextField } from '../../components/Field'
import { Card, ErrorBanner, Notice, PageHeader } from '../../components/ui'

const currencies = ['EUR', 'USD', 'GBP', 'CHF'] as const

export function PreferencesPage() {
  const { t, i18n } = useTranslation(['settings', 'common'])
  const { user } = useSession()
  const qc = useQueryClient()
  const errorText = useErrorText()

  const [displayName, setDisplayName] = useState(user.display_name)
  const [locale, setLocale] = useState<Locale>(user.locale)
  const [dateFormat, setDateFormat] = useState<DateFormat>(user.date_format)
  const [numberFormat, setNumberFormat] = useState<NumberFormat>(user.number_format)
  const [currencyCode, setCurrencyCode] = useState(user.currency_code)
  const [currencySymbol, setCurrencySymbol] = useState(user.currency_symbol ?? '')
  const [powerUnit, setPowerUnit] = useState<PowerUnit>(user.power_unit)

  const save = useMutation({
    mutationFn: () =>
      api.patch<{ user: User }>('/me/preferences', {
        display_name: displayName,
        locale,
        date_format: dateFormat,
        number_format: numberFormat,
        currency_code: currencyCode,
        currency_symbol: currencySymbol,
        power_unit: powerUnit,
      }),
    onSuccess: ({ user: updated }) => {
      qc.setQueryData<MeResponse | null>(queryKeys.me, (prev) => (prev ? { ...prev, user: updated } : prev))
      void i18n.changeLanguage(updated.locale)
    },
  })
  const fe = fieldErrors(save.error)

  const preview = createFormatter(
    {
      dateFormat,
      numberFormat,
      currencyCode: /^[A-Z]{3}$/.test(currencyCode) ? currencyCode : 'EUR',
      currencySymbol: currencySymbol.trim() || null,
      powerUnit,
    },
    (u) => t(`common:units.${u}`),
  )

  const currencyOptions = currencies.includes(currencyCode as (typeof currencies)[number])
    ? currencies.map((c) => ({ value: c, label: t(`preferences.currencies.${c}`) }))
    : [...currencies.map((c) => ({ value: c, label: t(`preferences.currencies.${c}`) })), { value: currencyCode, label: currencyCode }]

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
            <SelectField
              label={t('preferences.dateFormat')}
              value={dateFormat}
              onChange={(v) => { setDateFormat(v as DateFormat) }}
              options={[
                { value: 'DD.MM.YYYY', label: 'DD.MM.YYYY' },
                { value: 'YYYY-MM-DD', label: 'YYYY-MM-DD' },
              ]}
              error={fe.date_format}
            />
            <SelectField
              label={t('preferences.numberFormat')}
              value={numberFormat}
              onChange={(v) => { setNumberFormat(v as NumberFormat) }}
              options={[
                { value: 'point', label: t('preferences.numberPoint') },
                { value: 'comma', label: t('preferences.numberComma') },
              ]}
              error={fe.number_format}
            />
            <SelectField label={t('preferences.currency')} value={currencyCode} onChange={setCurrencyCode} options={currencyOptions} error={fe.currency_code} />
            <SelectField
              label={t('preferences.powerUnit')}
              value={powerUnit}
              onChange={(v) => { setPowerUnit(v as PowerUnit) }}
              options={[
                { value: 'kw', label: t('preferences.powerUnits.kw') },
                { value: 'ps', label: t('preferences.powerUnits.ps') },
              ]}
              hint={t('preferences.powerUnitHint')}
              error={fe.power_unit}
            />
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
          <div className="rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-sm">
            <span className="text-zinc-400">{t('preferences.preview')}: </span>
            <span className="tabular-nums text-zinc-200">
              {[preview.date('2026-10-15'), preview.km(215000), preview.liters(6500), preview.money(8950), preview.power(110)].join(' · ')}
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
