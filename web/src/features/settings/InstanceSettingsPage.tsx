import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { InstanceSettings } from '../../lib/types'
import { Button } from '../../components/Button'
import { SelectField } from '../../components/Field'
import { Card, DefinitionList, ErrorBanner, Loading, Notice, PageHeader } from '../../components/ui'

export function InstanceSettingsPage() {
  const errorText = useErrorText()
  const settings = useQuery({ queryKey: queryKeys.instance, queryFn: () => api.get<InstanceSettings>('/settings/instance') })
  if (settings.isPending) return <Loading />
  if (settings.isError) return <ErrorBanner message={errorText(settings.error)} />
  return <InstanceForm settings={settings.data} />
}

function InstanceForm({ settings }: { settings: InstanceSettings }) {
  const { t } = useTranslation('settings')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [weekday, setWeekday] = useState(String(settings.weekly_digest_weekday))
  const [weeklyHour, setWeeklyHour] = useState(String(settings.weekly_digest_hour))
  const [monthDay, setMonthDay] = useState(String(settings.monthly_digest_day))
  const [monthlyHour, setMonthlyHour] = useState(String(settings.monthly_digest_hour))
  const save = useMutation({
    mutationFn: () => api.patch<InstanceSettings>('/settings/instance', {
      weekly_digest_weekday: Number(weekday), weekly_digest_hour: Number(weeklyHour),
      monthly_digest_day: Number(monthDay), monthly_digest_hour: Number(monthlyHour),
    }),
    onSuccess: (data) => { qc.setQueryData(queryKeys.instance, data) },
  })
  const fe = fieldErrors(save.error)
  const hours = Array.from({ length: 24 }, (_, h) => ({ value: String(h), label: `${String(h).padStart(2, '0')}:00` }))
  const days = Array.from({ length: 28 }, (_, d) => ({ value: String(d + 1), label: String(d + 1) }))
  const weekdays = [1, 2, 3, 4, 5, 6, 7].map((d) => ({ value: String(d), label: t(`instance.weekdays.${d}`) }))
  return (
    <div className="max-w-2xl space-y-6">
      <PageHeader title={t('instance.title')} subtitle={t('instance.intro')} />
      <Card title={t('instance.digests')}>
        <div className="space-y-4">
          {save.isSuccess && <Notice tone="emerald">{t('common:state.saved')}</Notice>}
          {save.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(save.error)} />}
          <div className="form-grid grid grid-cols-1 gap-4 sm:grid-cols-2">
            <SelectField label={t('instance.weeklyDay')} value={weekday} onChange={setWeekday} options={weekdays} error={fe.weekly_digest_weekday} />
            <SelectField label={t('instance.weeklyHour')} value={weeklyHour} onChange={setWeeklyHour} options={hours} error={fe.weekly_digest_hour} />
            <SelectField label={t('instance.monthlyDay')} value={monthDay} onChange={setMonthDay} options={days} hint={t('instance.monthlyDayHint')} error={fe.monthly_digest_day} />
            <SelectField label={t('instance.monthlyHour')} value={monthlyHour} onChange={setMonthlyHour} options={hours} error={fe.monthly_digest_hour} />
          </div>
          <Button variant="primary" onClick={() => { save.mutate() }} disabled={save.isPending}>{t('common:actions.save')}</Button>
        </div>
      </Card>
      <Card title={t('instance.about')}>
        <DefinitionList
          items={[
            { label: t('instance.version'), value: <span className="font-mono">{settings.version}</span> },
            { label: t('instance.timeZone'), value: settings.time_zone },
            { label: t('instance.baseUrl'), value: settings.base_url_configured ? t('instance.baseUrlSet') : t('instance.baseUrlUnset') },
            { label: t('instance.maxUpload'), value: `${settings.max_upload_mb} MB` },
          ]}
        />
      </Card>
    </div>
  )
}
