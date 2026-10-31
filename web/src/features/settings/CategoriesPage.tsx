import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Archive, ArchiveRestore, Pencil, Plus, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { parseNumber } from '../../lib/format'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { ServiceCategory } from '../../lib/types'
import { useFormat } from '../../lib/useFormat'
import { useSession } from '../../app/session'
import { Button, IconButton } from '../../components/Button'
import { Checkbox, TextField } from '../../components/Field'
import { NumberField } from '../../components/NumberField'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { Badge, ErrorBanner, Loading, PageHeader, Table } from '../../components/ui'
import { categoryLabel, useCategories } from '../service-log/hooks'

export function CategoriesPage() {
  const { t } = useTranslation('service')
  const fmt = useFormat()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const categories = useCategories(true)
  const [editing, setEditing] = useState<ServiceCategory | 'new' | null>(null)
  const [deleting, setDeleting] = useState<ServiceCategory | null>(null)
  const invalidate = () => qc.invalidateQueries({ queryKey: queryKeys.categories })
  const archive = useMutation({
    mutationFn: (c: ServiceCategory) => api.post(`/service-categories/${c.id}/${c.archived ? 'unarchive' : 'archive'}`),
    onSuccess: invalidate,
  })
  const remove = useMutation({
    mutationFn: (id: number) => api.del(`/service-categories/${id}`),
    onSuccess: async () => { setDeleting(null); await invalidate() },
  })
  const none = t('common:state.none')

  return (
    <div>
      <PageHeader
        title={t('categoryAdmin.title')}
        subtitle={t('categoryAdmin.intro')}
        actions={
          <Button variant="primary" onClick={() => { setEditing('new') }}>
            <Plus size={16} />
            {t('categoryAdmin.add')}
          </Button>
        }
      />
      {archive.isError && <div className="mb-3"><ErrorBanner message={errorText(archive.error)} /></div>}
      {categories.isPending && <Loading />}
      {categories.data && (
        <Table>
          <thead>
            <tr>
              <th>{t('categoryAdmin.name')}</th>
              <th>{t('fields.costType')}</th>
              <th>{t('categoryAdmin.defaultIntervalKm')}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {categories.data.items.map((c) => (
              <tr key={c.id}>
                <td>
                  <span className="text-zinc-100">{categoryLabel(c, t)}</span>
                  <span className="ml-2">{c.builtin_key ? <Badge>{t('categoryAdmin.builtin')}</Badge> : <Badge tone="sky">{t('categoryAdmin.custom')}</Badge>}</span>
                  {c.archived && <span className="ml-2"><Badge tone="amber">{t('categoryAdmin.archived')}</Badge></span>}
                </td>
                <td className="text-zinc-400">{t(c.is_fluid ? 'costTypes.fluids' : 'costTypes.parts')}</td>
                <td className="tabular-nums text-zinc-300">
                  {[c.default_interval_km !== null ? fmt.km(c.default_interval_km) : null, c.default_interval_months !== null ? t('reminders:due.intervalMonths', { count: c.default_interval_months }) : null]
                    .filter(Boolean)
                    .join(' / ') || none}
                </td>
                <td>
                  <div className="flex justify-end gap-0.5">
                    <IconButton aria-label={t('common:actions.edit')} onClick={() => { setEditing(c) }}>
                      <Pencil size={15} />
                    </IconButton>
                    {!c.builtin_key && (
                      <>
                        <IconButton aria-label={c.archived ? t('categoryAdmin.unarchive') : t('categoryAdmin.archive')} onClick={() => { archive.mutate(c) }}>
                          {c.archived ? <ArchiveRestore size={15} /> : <Archive size={15} />}
                        </IconButton>
                        <IconButton aria-label={t('common:actions.delete')} onClick={() => { setDeleting(c) }}>
                          <Trash2 size={15} />
                        </IconButton>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
      {editing !== null && <CategoryDialog category={editing === 'new' ? null : editing} onClose={() => { setEditing(null) }} />}
      <ConfirmDialog
        open={deleting !== null}
        title={t('categoryAdmin.deleteTitle')}
        body={t('categoryAdmin.deleteBody', { name: deleting?.custom_name ?? '' })}
        confirmLabel={t('common:actions.delete')}
        danger
        busy={remove.isPending}
        error={remove.isError ? errorText(remove.error) : undefined}
        onConfirm={() => { if (deleting) remove.mutate(deleting.id) }}
        onClose={() => { setDeleting(null); remove.reset() }}
      />
    </div>
  )
}

function CategoryDialog({ category, onClose }: { category: ServiceCategory | null; onClose: () => void }) {
  const { t } = useTranslation('service')
  const { user } = useSession()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const builtin = category?.builtin_key !== null && category !== null
  const [name, setName] = useState(category?.custom_name ?? '')
  const [fluid, setFluid] = useState(category?.is_fluid ?? false)
  const [km, setKm] = useState(category?.default_interval_km?.toString() ?? '')
  const [months, setMonths] = useState(category?.default_interval_months?.toString() ?? '')
  const [local, setLocal] = useState<Record<string, string>>({})
  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) => (category ? api.patch(`/service-categories/${category.id}`, body) : api.post('/service-categories', body)),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: queryKeys.categories })
      onClose()
    },
  })
  const fe = { ...fieldErrors(save.error), ...local }
  const submit = () => {
    const errs: Record<string, string> = {}
    const int = (field: string, raw: string) => {
      if (raw.trim() === '') return null
      const n = parseNumber(raw, user.number_format)
      if (n === null || !Number.isInteger(n)) errs[field] = 'validation.number'
      return n
    }
    const body: Record<string, unknown> = { default_interval_km: int('default_interval_km', km), default_interval_months: int('default_interval_months', months) }
    if (!builtin) {
      body.custom_name = name
      body.is_fluid = fluid
    }
    setLocal(errs)
    if (Object.keys(errs).length === 0) save.mutate(body)
  }
  return (
    <Dialog
      open
      onClose={onClose}
      title={category ? t('categoryAdmin.editTitle') : t('categoryAdmin.add')}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" onClick={submit} disabled={save.isPending}>
            {t('common:actions.save')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {save.isError && Object.keys(fieldErrors(save.error)).length === 0 && <ErrorBanner message={errorText(save.error)} />}
        {builtin ? (
          <p className="text-sm text-zinc-200">{categoryLabel(category, t)}</p>
        ) : (
          <>
            <TextField label={t('categoryAdmin.name')} value={name} onChange={setName} error={fe.custom_name} maxLength={64} autoFocus />
            <Checkbox label={t('categoryAdmin.isFluid')} checked={fluid} onChange={setFluid} />
          </>
        )}
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <NumberField label={t('categoryAdmin.defaultIntervalKm')} unit={t('common:units.km')} decimal={false} value={km} onChange={setKm} error={fe.default_interval_km} optional />
          <NumberField label={t('categoryAdmin.defaultIntervalMonths')} decimal={false} value={months} onChange={setMonths} error={fe.default_interval_months} optional />
        </div>
      </div>
    </Dialog>
  )
}
