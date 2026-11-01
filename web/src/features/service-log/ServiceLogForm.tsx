import { useState, type SubmitEvent } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Plus, Trash2 } from 'lucide-react'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { parseMoneyToCents, parseNumber, toInputValue } from '../../lib/format'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { vehicleName, type CostType, type ServiceCategory, type ServiceLog } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button, IconButton } from '../../components/Button'
import { FieldPair, SelectField, TextAreaField, TextField } from '../../components/Field'
import { NumberField } from '../../components/NumberField'
import { Card, ErrorBanner, Loading, PageHeader } from '../../components/ui'
import { invalidateOdometerDependents, useVehicles } from '../vehicles/hooks'
import { categoryLabel, useCategories } from './hooks'
import { SpecHint } from './SpecHint'

interface ItemDraft {
  key: number
  categoryId: string
  brand: string
  specification: string
  quantity: string
  unit: 'L' | 'mL'
  oemPart: string
  aftermarketPart: string
  cost: string
  costType: CostType | ''
}

let nextKey = 1
function emptyItem(): ItemDraft {
  return { key: nextKey++, categoryId: '', brand: '', specification: '', quantity: '', unit: 'L', oemPart: '', aftermarketPart: '', cost: '', costType: '' }
}

export function ServiceLogFormPage() {
  const { id } = useParams()
  const errorText = useErrorText()
  const log = useQuery({
    queryKey: queryKeys.serviceLog(Number(id)),
    queryFn: () => api.get<{ service_log: ServiceLog }>(`/service-logs/${id ?? ''}`).then((r) => r.service_log),
    enabled: id !== undefined,
  })
  if (id === undefined) return <ServiceLogForm />
  if (log.isPending) return <Loading />
  if (log.isError) return <ErrorBanner message={errorText(log.error)} />
  return <ServiceLogForm log={log.data} />
}

function ServiceLogForm({ log }: { log?: ServiceLog }) {
  const { t } = useTranslation('service')
  const { user } = useSession()
  const nf = user.number_format
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const vehicles = useVehicles('false')
  const categories = useCategories(log !== undefined)

  const [vehicleId, setVehicleId] = useState(String(log?.vehicle_id ?? params.get('vehicle') ?? ''))
  const [date, setDate] = useState(log?.service_date ?? new Date().toISOString().slice(0, 10))
  const [odometer, setOdometer] = useState(log ? String(log.odometer_km) : '')
  const [notes, setNotes] = useState(log?.notes ?? '')
  const [extraCost, setExtraCost] = useState(log && log.extra_cost_cents > 0 ? toInputValue(log.extra_cost_cents / 100, nf, 2) : '')
  const [extraLabel, setExtraLabel] = useState(log?.extra_cost_label ?? '')
  const [items, setItems] = useState<ItemDraft[]>(() =>
    log
      ? log.items.map((it) => ({
          key: nextKey++,
          categoryId: String(it.category_id),
          brand: it.brand,
          specification: it.specification,
          quantity: it.quantity_ml === null ? '' : toInputValue(it.quantity_unit === 'L' ? it.quantity_ml / 1000 : it.quantity_ml, nf),
          unit: it.quantity_unit ?? 'L',
          oemPart: it.oem_part_number,
          aftermarketPart: it.aftermarket_part_number,
          cost: it.cost_cents === null ? '' : toInputValue(it.cost_cents / 100, nf, 2),
          costType: it.cost_type,
        }))
      : [emptyItem()],
  )
  const [local, setLocal] = useState<Record<string, string>>({})

  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      log ? api.put<{ service_log: ServiceLog; warnings?: string[] }>(`/service-logs/${log.id}`, body) : api.post<{ service_log: ServiceLog; warnings?: string[] }>('/service-logs', body),
    onSuccess: async (res) => {
      await Promise.all([invalidateOdometerDependents(qc), qc.invalidateQueries({ queryKey: queryKeys.serviceLogs })])
      void navigate(`/service/${res.service_log.id}`, { state: { warnings: res.warnings ?? [] } })
    },
  })
  const fe = { ...fieldErrors(save.error), ...local }

  const catById = new Map(categories.data?.items.map((c) => [c.id, c]))
  const update = (key: number, patch: Partial<ItemDraft>) => {
    setItems((prev) => prev.map((it) => (it.key === key ? { ...it, ...patch } : it)))
  }

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    const errs: Record<string, string> = {}
    const km = parseNumber(odometer, nf)
    if (km === null || !Number.isInteger(km) || km < 0) errs.odometer_km = 'validation.number'
    if (!vehicleId) errs.vehicle_id = 'validation.required'
    let extraCents = 0
    if (extraCost.trim() !== '') {
      const c = parseMoneyToCents(extraCost, nf)
      if (c === null) errs.extra_cost_cents = 'validation.number'
      else extraCents = c
    }
    const bodyItems = items.map((it, i) => {
      const p = `items.${i}.`
      if (!it.categoryId) errs[`${p}category_id`] = 'validation.required'
      let quantityMl: number | null = null
      if (it.quantity.trim() !== '') {
        const q = parseNumber(it.quantity, nf)
        if (q === null || q <= 0) errs[`${p}quantity_ml`] = 'validation.positive'
        else quantityMl = Math.round(it.unit === 'L' ? q * 1000 : q)
      }
      let costCents: number | null = null
      if (it.cost.trim() !== '') {
        costCents = parseMoneyToCents(it.cost, nf)
        if (costCents === null) errs[`${p}cost_cents`] = 'validation.number'
      }
      return {
        category_id: Number(it.categoryId),
        brand: it.brand,
        specification: it.specification,
        quantity_ml: quantityMl,
        quantity_unit: quantityMl === null ? '' : it.unit,
        oem_part_number: it.oemPart,
        aftermarket_part_number: it.aftermarketPart,
        cost_cents: costCents,
        cost_type: it.costType,
      }
    })
    setLocal(errs)
    if (Object.keys(errs).length > 0) return
    save.mutate({
      vehicle_id: Number(vehicleId), service_date: date, odometer_km: km, notes,
      extra_cost_cents: extraCents, extra_cost_label: extraLabel, items: bodyItems,
    })
  }

  const usedCategories = new Set(items.map((i) => i.categoryId))
  const categoryOptions = (current: string) => [
    { value: '', label: '—' },
    ...(categories.data?.items ?? [])
      .filter((c) => (!c.archived || String(c.id) === current) && (String(c.id) === current || !usedCategories.has(String(c.id))))
      .map((c) => ({ value: String(c.id), label: categoryLabel(c, t) })),
  ]
  const costTypeDefault = (c: ServiceCategory | undefined): CostType => (c?.is_fluid ? 'fluids' : 'parts')
  const generalError = save.isError && Object.keys(fieldErrors(save.error)).length === 0 ? errorText(save.error) : undefined
  const unitL = t('common:units.l')
  const unitMl = t('common:units.ml')

  return (
    <div className="max-w-4xl">
      <PageHeader title={log ? t('edit') : t('add')} />
      <form onSubmit={submit} className="space-y-6" noValidate>
        {generalError && <ErrorBanner message={generalError} />}
        <Card>
          <div className="form-grid grid grid-cols-1 gap-4 sm:grid-cols-3">
            <SelectField
              label={t('fields.vehicle')}
              value={vehicleId}
              onChange={setVehicleId}
              error={fe.vehicle_id}
              options={[{ value: '', label: '—' }, ...(vehicles.data?.items ?? []).map((v) => ({ value: String(v.id), label: vehicleName(v) }))]}
            />
            <TextField label={t('fields.date')} type="date" value={date} onChange={setDate} error={fe.service_date} />
            <NumberField label={t('fields.odometer')} unit={t('common:units.km')} decimal={false} value={odometer} onChange={setOdometer} error={fe.odometer_km} />
          </div>
        </Card>

        <Card
          title={t('items.title')}
          actions={
            <Button size="sm" onClick={() => { setItems((prev) => [...prev, emptyItem()]) }}>
              <Plus size={15} />
              {t('items.add')}
            </Button>
          }
        >
          <div className="space-y-4">
            {items.map((it, i) => {
              const p = `items.${i}.`
              const cat = catById.get(Number(it.categoryId))
              return (
                <fieldset key={it.key} className="space-y-3 rounded-md border border-zinc-800 bg-zinc-950 p-3">
                  <div className="flex items-start gap-2">
                    <SelectField
                      label={t('fields.category')}
                      value={it.categoryId}
                      onChange={(v) => { update(it.key, { categoryId: v }) }}
                      options={categoryOptions(it.categoryId)}
                      error={fe[`${p}category_id`]}
                      className="flex-1"
                    />
                    <IconButton aria-label={t('items.remove')} className="mt-7" disabled={items.length === 1} onClick={() => { setItems((prev) => prev.filter((x) => x.key !== it.key)) }}>
                      <Trash2 size={15} />
                    </IconButton>
                  </div>
                  {vehicleId && <SpecHint vehicleId={Number(vehicleId)} category={cat} />}
                  <div className="form-grid grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
                    <TextField label={t('fields.brand')} value={it.brand} onChange={(v) => { update(it.key, { brand: v }) }} error={fe[`${p}brand`]} optional />
                    <TextField label={t('fields.specification')} value={it.specification} onChange={(v) => { update(it.key, { specification: v }) }} error={fe[`${p}specification`]} optional />
                    <FieldPair>
                      <NumberField label={t('fields.quantity')} value={it.quantity} onChange={(v) => { update(it.key, { quantity: v }) }} error={fe[`${p}quantity_ml`]} className="flex-1" optional />
                      <SelectField
                        label={t('fields.unit')}
                        value={it.unit}
                        onChange={(v) => { update(it.key, { unit: v as 'L' | 'mL' }) }}
                        options={[{ value: 'L', label: unitL }, { value: 'mL', label: unitMl }]}
                        className="w-20"
                      />
                    </FieldPair>
                    <NumberField label={t('fields.cost')} value={it.cost} onChange={(v) => { update(it.key, { cost: v }) }} error={fe[`${p}cost_cents`]} optional />
                    <TextField label={t('fields.oemPart')} value={it.oemPart} onChange={(v) => { update(it.key, { oemPart: v }) }} error={fe[`${p}oem_part_number`]} optional />
                    <TextField label={t('fields.aftermarketPart')} value={it.aftermarketPart} onChange={(v) => { update(it.key, { aftermarketPart: v }) }} error={fe[`${p}aftermarket_part_number`]} optional />
                    <SelectField
                      label={t('fields.costType')}
                      value={it.costType || costTypeDefault(cat)}
                      onChange={(v) => { update(it.key, { costType: v as CostType }) }}
                      options={[{ value: 'parts', label: t('costTypes.parts') }, { value: 'fluids', label: t('costTypes.fluids') }]}
                    />
                  </div>
                </fieldset>
              )
            })}
          </div>
        </Card>

        <Card>
          <div className="form-grid grid grid-cols-1 gap-4 sm:grid-cols-2">
            <NumberField label={t('fields.extraCost')} value={extraCost} onChange={setExtraCost} error={fe.extra_cost_cents} optional />
            <TextField label={t('fields.extraCostLabel')} value={extraLabel} onChange={setExtraLabel} hint={t('fields.extraCostHint')} error={fe.extra_cost_label} optional />
            <TextAreaField label={t('fields.notes')} value={notes} onChange={setNotes} error={fe.notes} className="sm:col-span-2" optional />
          </div>
        </Card>

        <div className="flex gap-2">
          <Button type="submit" variant="primary" disabled={save.isPending}>
            {save.isPending ? t('common:actions.saving') : t('common:actions.save')}
          </Button>
          <Button onClick={() => void navigate(-1)}>{t('common:actions.cancel')}</Button>
        </div>
      </form>
    </div>
  )
}
