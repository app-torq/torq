import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { queryKeys } from '../../lib/queryKeys'
import type { ServiceLog } from '../../lib/types'
import { AttachmentGallery } from '../../components/AttachmentGallery'
import { FileDrop } from '../../components/FileDrop'
import { Card } from '../../components/ui'

export function ServiceLogAttachments({ log }: { log: ServiceLog }) {
  const { t } = useTranslation('repairs')
  const qc = useQueryClient()
  const refresh = () => void qc.invalidateQueries({ queryKey: queryKeys.serviceLog(log.id) })
  return (
    <Card title={t('sections.attachments')}>
      <div className="space-y-4">
        <AttachmentGallery items={log.attachments ?? []} onChanged={refresh} />
        <FileDrop path={`/service-logs/${log.id}/attachments`} onUploaded={refresh} />
      </div>
    </Card>
  )
}
