import { useQueryClient } from '@tanstack/react-query'
import { queryKeys } from '../../lib/queryKeys'
import type { SpecEntry } from '../../lib/types'
import { AttachmentGallery } from '../../components/AttachmentGallery'
import { FileDrop } from '../../components/FileDrop'

/** Photos and PDFs on a custom spec entry, e.g. a belt routing diagram or a data sheet (FR-015, FR-054). */
export function SpecEntryAttachments({ entry, vehicleId, canEdit }: { entry: SpecEntry; vehicleId: number; canEdit: boolean }) {
  const qc = useQueryClient()
  const refresh = () => void qc.invalidateQueries({ queryKey: queryKeys.specs(vehicleId) })
  return (
    <div className="mt-2 space-y-2">
      {entry.attachments.length > 0 && <AttachmentGallery items={entry.attachments} onChanged={refresh} readOnly={!canEdit} />}
      {canEdit && <FileDrop path={`/spec-entries/${entry.id}/attachments`} compact onUploaded={refresh} />}
    </div>
  )
}
