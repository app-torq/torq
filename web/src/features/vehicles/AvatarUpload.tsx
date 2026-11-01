import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { useErrorText } from '../../lib/errors'
import type { Vehicle } from '../../lib/types'
import { Button } from '../../components/Button'
import { FileDrop } from '../../components/FileDrop'
import { Card, ErrorBanner } from '../../components/ui'
import { VehicleAvatar } from './VehicleAvatar'

/** Vehicle photo: upload replaces the current one. */
export function AvatarUpload({ vehicle }: { vehicle: Vehicle }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const refresh = () => Promise.all([qc.invalidateQueries({ queryKey: queryKeys.vehicles }), qc.invalidateQueries({ queryKey: queryKeys.dashboard })])
  const remove = useMutation({ mutationFn: () => api.del(`/vehicles/${vehicle.id}/avatar`), onSuccess: refresh })
  return (
    <Card title={t('attachments.photo')} className="mt-6">
      <div className="flex flex-wrap items-center gap-4">
        <VehicleAvatar vehicle={vehicle} size={96} />
        <div className="min-w-[14rem] flex-1">
          <FileDrop path={`/vehicles/${vehicle.id}/avatar`} method="PUT" imagesOnly multiple={false} compact onUploaded={() => void refresh()} />
        </div>
        {vehicle.avatar_url && (
          <Button variant="ghost" onClick={() => { remove.mutate() }} disabled={remove.isPending}>
            <Trash2 size={15} />
            {t('attachments.removePhoto')}
          </Button>
        )}
      </div>
      {remove.isError && <div className="mt-3"><ErrorBanner message={errorText(remove.error)} /></div>}
    </Card>
  )
}
