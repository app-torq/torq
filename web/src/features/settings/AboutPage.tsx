import { useEffect, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ExternalLink, RefreshCw } from 'lucide-react'
import { useSession } from '../../app/session'
import { useErrorText } from '../../lib/errors'
import { useFormat } from '../../lib/useFormat'
import type { CatalogMeta } from '../../lib/types'
import { Button } from '../../components/Button'
import { Card, DefinitionList, ErrorBanner, Loading, Notice, PageHeader } from '../../components/ui'
import { useCatalogMeta, useCatalogUpdate } from '../vehicles/catalog'

const linkClass = 'focus-ring inline-flex items-center gap-1 rounded text-sky-300 underline-offset-2 hover:underline'

/**
 * About torq and the vehicle data (FR-028, Constitution VII). Visible to every
 * signed-in user; admins can also update the catalog online. Attribution
 * texts are shown verbatim because the licenses prescribe their wording
 * (research C13).
 */
export function AboutPage() {
  const { t } = useTranslation('settings')
  const { instance, user } = useSession()
  const fmt = useFormat()
  const errorText = useErrorText()
  const meta = useCatalogMeta()

  return (
    <div className="max-w-3xl space-y-6">
      <PageHeader title={t('about.title')} subtitle={t('about.intro')} />
      <Card>
        <DefinitionList items={[{ label: t('about.version'), value: <span className="font-mono">{instance.version}</span> }]} />
      </Card>

      <section aria-labelledby="about-datasets" className="space-y-3">
        <h2 id="about-datasets" className="text-base font-semibold text-zinc-100">{t('about.datasets')}</h2>
        <p className="text-sm text-zinc-400">{t('about.offline')}</p>
        {meta.data && <CatalogStatus meta={meta.data} canUpdate={user.role === 'admin'} />}
        {meta.isPending && <Loading />}
        {meta.isError && <ErrorBanner message={errorText(meta.error)} onRetry={() => void meta.refetch()} />}
        {meta.data && meta.data.datasets.length === 0 && <p className="text-sm text-zinc-400">{t('about.empty')}</p>}
        {meta.data?.datasets.map((d) => (
          <Card key={d.id} title={d.name}>
            <div className="space-y-3 text-sm">
              <p className="rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-zinc-200">{d.attribution}</p>
              <DefinitionList
                items={[
                  { label: t('about.publisher'), value: d.publisher },
                  {
                    label: t('about.license'),
                    value: (
                      <a href={d.license_url} target="_blank" rel="noreferrer" className={linkClass}>
                        {d.license}
                        <ExternalLink aria-hidden size={13} />
                      </a>
                    ),
                  },
                  {
                    label: t('about.source'),
                    value: (
                      <a href={d.uri} target="_blank" rel="noreferrer" className={`${linkClass} break-all`}>
                        {d.uri}
                        <ExternalLink aria-hidden size={13} />
                      </a>
                    ),
                  },
                  ...(d.stand ? [{ label: t('about.stand'), value: fmt.date(d.stand) }] : []),
                  { label: t('about.retrieved'), value: fmt.date(d.retrieved) },
                  ...(d.commit ? [{ label: t('about.commit'), value: <span className="font-mono text-xs">{d.commit}</span> }] : []),
                ]}
              />
              {d.modified && <p className="text-xs text-zinc-400">{t('about.modified')}</p>}
            </div>
          </Card>
        ))}
        {meta.data && meta.data.datasets.length > 0 && (
          <p className="text-xs text-zinc-400">
            {t('about.counts', { kba: fmt.number(meta.data.counts.kba), vehicles: fmt.number(meta.data.counts.vehicles) })}{' '}
            {t('about.derived')}
          </p>
        )}
      </section>
    </div>
  )
}

/** Where the catalog came from and, for admins, the online update. */
function CatalogStatus({ meta, canUpdate }: { meta: CatalogMeta; canUpdate: boolean }) {
  const { t } = useTranslation('settings')
  const fmt = useFormat()
  const errorText = useErrorText()
  const qc = useQueryClient()
  const update = useCatalogUpdate()
  const { state, started_at, finished_at, error } = meta.update
  const running = state === 'running' || update.isPending

  // A finished update replaces the catalog: drop cached lookups and counts.
  const previous = useRef(state)
  useEffect(() => {
    if (previous.current === 'running' && state === 'succeeded') {
      void qc.invalidateQueries({ queryKey: ['catalog'] })
    }
    previous.current = state
  }, [state, qc])

  const when = state === 'running' ? started_at : finished_at
  return (
    <Card>
      <div className="space-y-3 text-sm">
        <DefinitionList
          items={[
            { label: t('about.catalogSource'), value: t(`about.sources.${meta.source}`) },
            ...(meta.imported_at ? [{ label: t('about.imported'), value: fmt.dateTime(meta.imported_at) }] : []),
          ]}
        />
        {canUpdate && (
          <div className="space-y-2 border-t border-zinc-800 pt-3">
            {meta.update.enabled ? (
              <>
                <p className="text-zinc-400">{t('about.updateHint')}</p>
                <Button onClick={() => { update.mutate() }} disabled={running}>
                  <RefreshCw aria-hidden size={14} className={running ? 'animate-spin' : ''} />
                  {running ? t('about.updating') : t('about.update')}
                </Button>
              </>
            ) : (
              <p className="text-zinc-400">{t('about.updateDisabled')}</p>
            )}
            {update.isError && <ErrorBanner message={errorText(update.error)} />}
            {state !== 'idle' && when && (
              <Notice tone={state === 'failed' ? 'amber' : state === 'running' ? 'sky' : 'emerald'}>
                {t(`about.updateStates.${state}`, { time: fmt.dateTime(when) })}
                {state === 'failed' && error && <span className="mt-1 block break-all font-mono text-xs">{error}</span>}
              </Notice>
            )}
          </div>
        )}
      </div>
    </Card>
  )
}
