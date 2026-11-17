import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Pencil, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { fieldErrors, useErrorText } from '../../lib/errors'
import type { RepairTag } from '../../lib/types'
import { Button, IconButton } from '../../components/Button'
import { TextField } from '../../components/Field'
import { ConfirmDialog, Dialog } from '../../components/Dialog'
import { EmptyState, ErrorBanner, Loading, PageHeader, Table } from '../../components/ui'
import { TagChip, useRepairTags } from '../repairs/tags'

/** Settings → Project tags: rename or delete the garage's tags (#16). Tags are created on projects. */
export function TagsPage() {
  const { t } = useTranslation('repairs')
  const qc = useQueryClient()
  const errorText = useErrorText()
  const tags = useRepairTags()
  const [renaming, setRenaming] = useState<RepairTag | null>(null)
  const [deleting, setDeleting] = useState<RepairTag | null>(null)
  const invalidate = () => Promise.all([qc.invalidateQueries({ queryKey: queryKeys.tags }), qc.invalidateQueries({ queryKey: queryKeys.repairs })])
  const remove = useMutation({
    mutationFn: (id: number) => api.del(`/repair-tags/${id}`),
    onSuccess: async () => {
      await invalidate()
      setDeleting(null)
    },
  })
  const items = tags.data?.items ?? []
  return (
    <div>
      <PageHeader title={t('tags.title')} subtitle={t('tags.intro')} />
      {tags.isPending && <Loading />}
      {tags.isError && <ErrorBanner message={errorText(tags.error)} onRetry={() => void tags.refetch()} />}
      {tags.isSuccess && items.length === 0 && <EmptyState title={t('tags.empty')} />}
      {items.length > 0 && (
        <Table>
          <thead>
            <tr>
              <th>{t('tags.name')}</th>
              <th className="text-right">{t('priorities.projects')}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {items.map((tag) => (
              <tr key={tag.id}>
                <td><TagChip name={tag.name} /></td>
                <td className="text-right tabular-nums text-zinc-300">{tag.project_count}</td>
                <td>
                  <div className="flex justify-end gap-0.5">
                    <IconButton aria-label={t('tags.rename', { name: tag.name })} onClick={() => { setRenaming(tag) }}>
                      <Pencil size={15} />
                    </IconButton>
                    <IconButton aria-label={t('tags.delete', { name: tag.name })} onClick={() => { setDeleting(tag) }}>
                      <Trash2 size={15} />
                    </IconButton>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
      {renaming && <RenameTagDialog tag={renaming} onClose={() => { setRenaming(null) }} onSaved={invalidate} />}
      <ConfirmDialog
        open={deleting !== null}
        title={t('tags.delete', { name: deleting?.name ?? '' })}
        body={deleting && deleting.project_count > 0 ? t('tags.deleteBody', { count: deleting.project_count }) : t('tags.deleteUnused')}
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

function RenameTagDialog({ tag, onClose, onSaved }: { tag: RepairTag; onClose: () => void; onSaved: () => Promise<unknown> }) {
  const { t } = useTranslation('repairs')
  const errorText = useErrorText()
  const [name, setName] = useState(tag.name)
  const save = useMutation({
    mutationFn: () => api.patch(`/repair-tags/${tag.id}`, { name }),
    onSuccess: async () => {
      await onSaved()
      onClose()
    },
  })
  const fe = fieldErrors(save.error)
  return (
    <Dialog
      open
      onClose={onClose}
      title={t('tags.rename', { name: tag.name })}
      footer={
        <>
          <Button onClick={onClose}>{t('common:actions.cancel')}</Button>
          <Button variant="primary" onClick={() => { save.mutate() }} disabled={save.isPending}>
            {t('common:actions.save')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        {save.isError && Object.keys(fe).length === 0 && <ErrorBanner message={errorText(save.error)} />}
        <TextField label={t('tags.name')} value={name} onChange={setName} error={fe.name} maxLength={40} hint={t('tags.renameHint')} autoFocus />
      </div>
    </Dialog>
  )
}
