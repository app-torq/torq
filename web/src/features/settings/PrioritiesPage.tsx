import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ArrowDown, ArrowUp, Pencil, Plus, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { PRIORITY_COLORS, priorityClasses } from '../../lib/priorityColors'
import type { RepairPriority } from '../../lib/types'
import { Button, IconButton } from '../../components/Button'
import { Checkbox, SelectField, TextField } from '../../components/Field'
import { Dialog } from '../../components/Dialog'
import { Badge, ErrorBanner, Loading, PageHeader, Table } from '../../components/ui'
import { PriorityBadge, priorityName, useRepairPriorities } from '../repairs/priorities'

/** Settings → Project priorities: the garage's ordered priority list (#15). */
export function PrioritiesPage() {
  const { t } = useTranslation('repairs')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const priorities = useRepairPriorities()
  const [editing, setEditing] = useState<RepairPriority | 'new' | null>(null)
  const [deleting, setDeleting] = useState<RepairPriority | null>(null)
  const items = priorities.data?.items ?? []

  const invalidate = () => Promise.all([qc.invalidateQueries({ queryKey: queryKeys.priorities }), qc.invalidateQueries({ queryKey: queryKeys.repairs })])
  const reorder = useMutation({
    mutationFn: (ids: number[]) => api.put('/repair-priorities/order', { ids }),
    onSuccess: invalidate,
  })
  const move = (index: number, by: -1 | 1) => {
    const ids = items.map((p) => p.id)
    const [id] = ids.splice(index, 1)
    if (id === undefined) return
    ids.splice(index + by, 0, id)
    reorder.mutate(ids)
  }

  return (
    <div>
      <PageHeader
        title={t('priorities.title')}
        subtitle={t('priorities.intro')}
        actions={
          <Button variant="primary" onClick={() => { setEditing('new') }}>
            <Plus size={16} />
            {t('priorities.add')}
          </Button>
        }
      />
      {reorder.isError && <div className="mb-3"><ErrorBanner message={errorText(reorder.error)} /></div>}
      {priorities.isPending && <Loading />}
      {priorities.data && (
        <Table>
          <thead>
            <tr>
              <th>{t('priorities.name')}</th>
              <th className="text-right">{t('priorities.projects')}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {items.map((p, i) => (
              <tr key={p.id}>
                <td>
                  <span className="inline-flex flex-wrap items-center gap-2">
                    <PriorityBadge priority={p} display="both" />
                    {p.is_default && <Badge tone="sky">{t('priorities.default')}</Badge>}
                  </span>
                </td>
                <td className="text-right tabular-nums text-zinc-300">{p.project_count}</td>
                <td>
                  <div className="flex justify-end gap-0.5">
                    <IconButton aria-label={t('priorities.moveUp', { name: priorityName(p, t) })} disabled={i === 0 || reorder.isPending} onClick={() => { move(i, -1) }}>
                      <ArrowUp size={15} />
                    </IconButton>
                    <IconButton aria-label={t('priorities.moveDown', { name: priorityName(p, t) })} disabled={i === items.length - 1 || reorder.isPending} onClick={() => { move(i, 1) }}>
                      <ArrowDown size={15} />
                    </IconButton>
                    <IconButton aria-label={t('common:actions.edit')} onClick={() => { setEditing(p) }}>
                      <Pencil size={15} />
                    </IconButton>
                    <IconButton aria-label={t('common:actions.delete')} onClick={() => { setDeleting(p) }}>
                      <Trash2 size={15} />
                    </IconButton>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
      {editing !== null && <PriorityDialog priority={editing === 'new' ? null : editing} onClose={() => { setEditing(null) }} onSaved={invalidate} />}
      {deleting && <DeletePriorityDialog priority={deleting} others={items.filter((p) => p.id !== deleting.id)} onClose={() => { setDeleting(null) }} onDeleted={invalidate} />}
    </div>
  )
}

function PriorityDialog({ priority, onClose, onSaved }: { priority: RepairPriority | null; onClose: () => void; onSaved: () => Promise<unknown> }) {
  const { t } = useTranslation('repairs')
  const errorText = useErrorText()
  const builtin = priority?.builtin_key != null
  const [name, setName] = useState(priority ? priorityName(priority, t) : '')
  const [color, setColor] = useState(priority?.color ?? 'sky')
  const [isDefault, setIsDefault] = useState(priority?.is_default ?? false)
  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) => (priority ? api.patch(`/repair-priorities/${priority.id}`, body) : api.post('/repair-priorities', body)),
    onSuccess: async () => {
      await onSaved()
      onClose()
    },
  })
  const fe = fieldErrors(save.error)
  const translated = priority?.builtin_key ? t(`priorities.builtin.${priority.builtin_key}`) : ''
  const submit = () => {
    // A built-in keeps its translated name unless it was changed.
    const trimmed = name.trim()
    const nameValue = builtin && trimmed === translated ? '' : trimmed
    save.mutate({ name: nameValue, color, is_default: isDefault })
  }
  return (
    <Dialog
      open
      onClose={onClose}
      title={priority ? t('priorities.editTitle') : t('priorities.add')}
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
        {save.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(save.error)} />}
        <TextField
          label={t('priorities.name')}
          value={name}
          onChange={setName}
          error={fe.name}
          maxLength={40}
          autoFocus
          {...(builtin ? { hint: t('priorities.builtinHint', { name: translated }) } : {})}
        />
        <fieldset>
          <legend className="mb-1.5 text-sm font-medium text-zinc-300">{t('priorities.color')}</legend>
          <div role="radiogroup" aria-label={t('priorities.color')} className="flex flex-wrap gap-2">
            {PRIORITY_COLORS.map((c) => (
              <button
                key={c}
                type="button"
                role="radio"
                aria-checked={color === c}
                aria-label={t(`priorities.colors.${c}`)}
                title={t(`priorities.colors.${c}`)}
                onClick={() => { setColor(c) }}
                className={`focus-ring h-8 w-8 rounded-full ring-offset-2 ring-offset-zinc-900 ${priorityClasses(c).dot} ${color === c ? 'ring-2 ring-zinc-100' : ''}`}
              />
            ))}
          </div>
          {fe.color && <p className="mt-1 text-xs text-red-400">{t(`errors:${fe.color}`)}</p>}
        </fieldset>
        <Checkbox label={t('priorities.makeDefault')} checked={isDefault} onChange={setIsDefault} />
        <p className="text-xs text-zinc-400">{t('priorities.defaultHint')}</p>
      </div>
    </Dialog>
  )
}

function DeletePriorityDialog({ priority, others, onClose, onDeleted }: { priority: RepairPriority; others: RepairPriority[]; onClose: () => void; onDeleted: () => Promise<unknown> }) {
  const { t } = useTranslation('repairs')
  const errorText = useErrorText()
  const [moveTo, setMoveTo] = useState('')
  const remove = useMutation({
    mutationFn: () => api.del(`/repair-priorities/${priority.id}${moveTo ? `?move_to=${moveTo}` : ''}`),
    onSuccess: async () => {
      await onDeleted()
      onClose()
    },
  })
  const name = priorityName(priority, t)
  return (
    <Dialog
      open
      onClose={onClose}
      title={t('priorities.deleteTitle', { name })}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="danger" onClick={() => { remove.mutate() }} disabled={remove.isPending}>
            {t('common:actions.delete')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {remove.isError && <ErrorBanner message={errorText(remove.error)} />}
        {priority.project_count > 0 ? (
          <SelectField
            label={t('priorities.moveProjects', { count: priority.project_count })}
            value={moveTo}
            onChange={setMoveTo}
            options={[{ value: '', label: t('priorities.none') }, ...others.map((p) => ({ value: String(p.id), label: priorityName(p, t) }))]}
          />
        ) : (
          <p className="text-sm text-zinc-300">{t('priorities.deleteUnused')}</p>
        )}
        {priority.is_default && <p className="text-sm text-amber-200">{t('priorities.deleteDefault')}</p>}
      </div>
    </Dialog>
  )
}
