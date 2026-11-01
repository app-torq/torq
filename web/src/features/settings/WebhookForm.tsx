import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Plus, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { Webhook, WebhookEvents, WebhookPreset } from '../../lib/types'
import { Button, IconButton } from '../../components/Button'
import { Checkbox, SelectField, TextField, inputClass } from '../../components/Field'
import { Dialog } from '../../components/Dialog'
import { ErrorBanner } from '../../components/ui'

const presets: WebhookPreset[] = ['discord', 'ntfy', 'gotify', 'home_assistant', 'generic_json']
const eventKeys: (keyof WebhookEvents)[] = ['due_soon', 'overdue', 'weekly_digest', 'monthly_digest']

interface HeaderDraft {
  key: number
  name: string
  /** null = keep the stored value (write-only, never shown). */
  value: string | null
}
let headerKey = 1

export function WebhookForm({ webhook, onClose }: { webhook: Webhook | null; onClose: () => void }) {
  const { t } = useTranslation('webhooks')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [name, setName] = useState(webhook?.name ?? '')
  const [url, setUrl] = useState(webhook?.url ?? '')
  const [preset, setPreset] = useState<WebhookPreset>(webhook?.preset ?? 'ntfy')
  const [enabled, setEnabled] = useState(webhook?.enabled ?? true)
  const [events, setEvents] = useState<WebhookEvents>(webhook?.events ?? { due_soon: true, overdue: true, weekly_digest: false, monthly_digest: false })
  const [headers, setHeaders] = useState<HeaderDraft[]>(webhook?.headers.map((h) => ({ key: headerKey++, name: h.name, value: null })) ?? [])
  const save = useMutation({
    mutationFn: () => {
      const body = {
        name, url, preset, enabled, events,
        headers: headers.filter((h) => h.name.trim() !== '').map((h) => (h.value === null ? { name: h.name } : { name: h.name, value: h.value })),
      }
      return webhook ? api.patch(`/webhooks/${webhook.id}`, body) : api.post('/webhooks', body)
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: queryKeys.webhooks })
      onClose()
    },
  })
  const fe = fieldErrors(save.error)
  const update = (key: number, patch: Partial<HeaderDraft>) => {
    setHeaders((prev) => prev.map((h) => (h.key === key ? { ...h, ...patch } : h)))
  }
  return (
    <Dialog
      open
      wide
      onClose={onClose}
      title={webhook ? t('edit') : t('add')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" onClick={() => { save.mutate() }} disabled={save.isPending}>{t('common:actions.save')}</Button>
        </>
      }
    >
      <div className="space-y-4">
        {save.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(save.error)} />}
        <div className="form-grid grid grid-cols-1 gap-4 sm:grid-cols-2">
          <TextField label={t('fields.name')} value={name} onChange={setName} error={fe.name} maxLength={64} />
          <SelectField label={t('fields.preset')} value={preset} onChange={(v) => { setPreset(v as WebhookPreset) }} error={fe.preset} options={presets.map((p) => ({ value: p, label: t(`presets.${p}`) }))} />
        </div>
        <TextField label={t('fields.url')} type="url" value={url} onChange={setUrl} error={fe.url} hint={t(`presetHelp.${preset}`)} placeholder="https://" />
        <Checkbox label={t('fields.enabled')} checked={enabled} onChange={setEnabled} />
        <fieldset className="space-y-2">
          <legend className="mb-1 text-sm font-medium text-zinc-300">{t('fields.events')}</legend>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            {eventKeys.map((k) => <Checkbox key={k} label={t(`events.${k}`)} checked={events[k]} onChange={(v) => { setEvents({ ...events, [k]: v }) }} />)}
          </div>
        </fieldset>
        <fieldset className="space-y-2">
          <legend className="mb-1 text-sm font-medium text-zinc-300">{t('fields.headers')}</legend>
          {fe.headers && <p className="text-xs text-red-400">{t(`errors:${fe.headers}`)}</p>}
          {headers.map((h) => (
            <div key={h.key} className="flex items-center gap-2">
              <input aria-label={t('fields.headerName')} placeholder="Authorization" className={`${inputClass} w-44 font-mono`} value={h.name} onChange={(e) => { update(h.key, { name: e.target.value }) }} />
              {h.value === null ? (
                <div className="flex flex-1 items-center gap-2">
                  <span className="flex-1 rounded-md border border-zinc-800 px-3 py-2 text-sm text-zinc-400">{t('headerSet')}</span>
                  <Button size="sm" onClick={() => { update(h.key, { value: '' }) }}>{t('replace')}</Button>
                </div>
              ) : (
                <input aria-label={t('fields.headerValue')} type="password" autoComplete="off" placeholder="Bearer …" className={`${inputClass} flex-1 font-mono`} value={h.value} onChange={(e) => { update(h.key, { value: e.target.value }) }} />
              )}
              <IconButton aria-label={t('common:actions.remove')} onClick={() => { setHeaders((prev) => prev.filter((x) => x.key !== h.key)) }}>
                <Trash2 size={15} />
              </IconButton>
            </div>
          ))}
          <Button size="sm" onClick={() => { setHeaders((prev) => [...prev, { key: headerKey++, name: '', value: '' }]) }}>
            <Plus size={15} />
            {t('addHeader')}
          </Button>
        </fieldset>
      </div>
    </Dialog>
  )
}
