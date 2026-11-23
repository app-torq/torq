import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Trash2, Upload } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { Branding, Contact, Formats, InstanceSettings } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button } from '../../components/Button'
import { SelectField, TextAreaField, TextField } from '../../components/Field'
import { Card, DefinitionList, ErrorBanner, Loading, Notice, PageHeader } from '../../components/ui'
import { FormatFields, UnitFields, unitChoices } from './FormatFields'

/** Must match maxLogoBytes in internal/httpapi/branding.go. */
const MAX_LOGO_KB = 512

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
  const [defaults, setDefaults] = useState<Formats>(settings.defaults)
  const patchDefaults = (p: Partial<Formats>) => { setDefaults((prev) => ({ ...prev, ...p })) }
  const saveDefaults = useMutation({
    mutationFn: () => api.patch<InstanceSettings>('/settings/instance', { defaults }),
    onSuccess: (data) => { qc.setQueryData(queryKeys.instance, data) },
  })
  const defaultsErrors = fieldErrors(saveDefaults.error)
  const hours = Array.from({ length: 24 }, (_, h) => ({ value: String(h), label: `${String(h).padStart(2, '0')}:00` }))
  const days = Array.from({ length: 28 }, (_, d) => ({ value: String(d + 1), label: String(d + 1) }))
  const weekdays = [1, 2, 3, 4, 5, 6, 7].map((d) => ({ value: String(d), label: t(`instance.weekdays.${d}`) }))
  return (
    <div className="max-w-2xl space-y-6">
      <PageHeader title={t('instance.title')} subtitle={t('instance.intro')} />
      <BrandingCard branding={settings.branding} />
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
      <Card title={t('instance.defaults')}>
        <div className="space-y-4">
          <p className="text-sm text-zinc-400">{t('instance.defaultsIntro')}</p>
          {saveDefaults.isSuccess && <Notice tone="emerald">{t('common:state.saved')}</Notice>}
          {saveDefaults.isError && Object.keys(defaultsErrors).length === 0 && <ErrorBanner message={errorText(saveDefaults.error)} />}
          <div className="form-grid grid grid-cols-1 gap-4 sm:grid-cols-2">
            <FormatFields value={defaults} onChange={patchDefaults} errors={defaultsErrors} errorPrefix="default_" />
          </div>
          <UnitFields value={unitChoices(defaults)} onChange={patchDefaults} errors={defaultsErrors} errorPrefix="default_" />
          <Button variant="primary" onClick={() => { saveDefaults.mutate() }} disabled={saveDefaults.isPending}>{t('common:actions.save')}</Button>
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

/** Instance name, logo and contact details (#32). */
function BrandingCard({ branding }: { branding: InstanceSettings['branding'] }) {
  const { t } = useTranslation('settings')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const { instance } = useSession()
  const fileInput = useRef<HTMLInputElement>(null)
  const [name, setName] = useState(branding.name)
  const [contact, setContact] = useState<Contact>(branding.contact)
  const patchContact = (p: Partial<Contact>) => { setContact((prev) => ({ ...prev, ...p })) }
  // Each mutation merges only the fields it changes, so a slower response
  // cannot undo the other. The header, sign-in page and title read the
  // branding from the session and GET /instance.
  const refresh = (b: Partial<InstanceSettings['branding']>) => {
    qc.setQueryData<InstanceSettings>(queryKeys.instance, (prev) => (prev ? { ...prev, branding: { ...prev.branding, ...b } } : prev))
    void qc.invalidateQueries({ queryKey: queryKeys.me })
    void qc.invalidateQueries({ queryKey: queryKeys.serverInfo })
  }
  const save = useMutation({
    mutationFn: () => api.patch<InstanceSettings>('/settings/instance', { branding: { name, ...contact } }),
    onSuccess: (data) => { refresh({ name: data.branding.name, contact: data.branding.contact }) },
  })
  const fe = fieldErrors(save.error)
  const [logoError, setLogoError] = useState<string | null>(null)
  const upload = useMutation({
    mutationFn: (file: File) => api.upload<{ branding: Branding }>('PUT', '/settings/instance/logo', file),
    onSuccess: (data) => { setLogoError(null); refresh({ logo_url: data.branding.logo_url }) },
    onError: (err) => { setLogoError(errorText(err)) },
  })
  const remove = useMutation({
    mutationFn: () => api.del<{ branding: Branding }>('/settings/instance/logo'),
    onSuccess: (data) => { setLogoError(null); refresh({ logo_url: data.branding.logo_url }) },
    onError: (err) => { setLogoError(errorText(err)) },
  })
  const pick = (file: File | undefined) => {
    if (!file) return
    if (file.size > MAX_LOGO_KB * 1024) {
      setLogoError(t('errors:instance.logo_too_large', { max_kb: MAX_LOGO_KB }))
      return
    }
    upload.mutate(file)
  }
  const locked = instance.demo_mode
  return (
    <Card title={t('instance.branding.title')}>
      <div className="space-y-4">
        <p className="text-sm text-zinc-400">{t('instance.branding.intro')}</p>
        {locked && <Notice tone="amber">{t('errors:demo.branding_locked')}</Notice>}
        <div className="space-y-2">
          <div className="text-sm font-medium text-zinc-300">{t('instance.branding.logo')}</div>
          <div className="flex flex-wrap items-center gap-3">
            <div className="flex h-16 w-40 items-center justify-center rounded-md border border-zinc-800 bg-zinc-950 p-2">
              {branding.logo_url
                ? <img src={branding.logo_url} alt={t('instance.branding.logo')} className="max-h-full max-w-full object-contain" />
                : <span className="text-xs text-zinc-500">{t('instance.branding.noLogo')}</span>}
            </div>
            <input
              ref={fileInput}
              type="file"
              accept="image/png,image/jpeg,image/svg+xml,.svg"
              className="hidden"
              onChange={(e) => { pick(e.target.files?.[0]); e.target.value = '' }}
            />
            <Button onClick={() => { fileInput.current?.click() }} disabled={locked || upload.isPending}>
              <Upload size={15} />
              {branding.logo_url ? t('instance.branding.replaceLogo') : t('instance.branding.uploadLogo')}
            </Button>
            {branding.logo_url && (
              <Button variant="ghost" onClick={() => { remove.mutate() }} disabled={locked || remove.isPending}>
                <Trash2 size={15} />
                {t('instance.branding.removeLogo')}
              </Button>
            )}
          </div>
          <p className="text-xs text-zinc-400">{t('instance.branding.logoHint', { max_kb: MAX_LOGO_KB })}</p>
          {logoError && <ErrorBanner message={logoError} />}
        </div>
        {save.isSuccess && <Notice tone="emerald">{t('common:state.saved')}</Notice>}
        {save.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(save.error)} />}
        <TextField label={t('instance.branding.name')} value={name} onChange={setName} maxLength={60} optional hint={t('instance.branding.nameHint')} error={fe['branding.name']} disabled={locked} />
        <div>
          <h3 className="text-sm font-medium text-zinc-300">{t('instance.branding.contact')}</h3>
          <p className="text-xs text-zinc-400">{t('instance.branding.contactHint')}</p>
        </div>
        <div className="form-grid grid grid-cols-1 gap-4 sm:grid-cols-2">
          <TextAreaField label={t('instance.branding.address')} value={contact.address} onChange={(v) => { patchContact({ address: v }) }} rows={3} maxLength={300} optional error={fe['branding.address']} className="sm:col-span-2" disabled={locked} />
          <TextField label={t('instance.branding.phone')} value={contact.phone} onChange={(v) => { patchContact({ phone: v }) }} type="tel" maxLength={40} optional error={fe['branding.phone']} disabled={locked} />
          <TextField label={t('instance.branding.email')} value={contact.email} onChange={(v) => { patchContact({ email: v }) }} type="email" maxLength={120} optional error={fe['branding.email']} disabled={locked} />
          <TextField label={t('instance.branding.website')} value={contact.website} onChange={(v) => { patchContact({ website: v }) }} maxLength={200} optional error={fe['branding.website']} className="sm:col-span-2" disabled={locked} />
        </div>
        <Button variant="primary" onClick={() => { save.mutate() }} disabled={locked || save.isPending}>{t('common:actions.save')}</Button>
      </div>
    </Card>
  )
}
