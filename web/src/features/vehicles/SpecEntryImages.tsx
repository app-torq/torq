import { useQueryClient } from '@tanstack/react-query'
import { queryKeys } from '../../lib/queryKeys'
import type { SpecEntry } from '../../lib/types'
import { AttachmentGallery } from '../../components/AttachmentGallery'
import { FileDrop } from '../../components/FileDrop'

/** Images on a custom spec entry, e.g. a belt routing diagram (FR-015). */
export function SpecEntryImages({ entry, vehicleId }: { entry: SpecEntry; vehicleId: number }) {
  const qc = useQueryClient()
  const refresh = () => void qc.invalidateQueries({ queryKey: queryKeys.specs(vehicleId) })
  return (
    <div className="mt-2 space-y-2">
      {entry.attachments.length > 0 && <AttachmentGallery items={entry.attachments} onChanged={refresh} />}
      <FileDrop path={`/spec-entries/${entry.id}/attachments`} imagesOnly compact onUploaded={refresh} />
    </div>
  )
}
