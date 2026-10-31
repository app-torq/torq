import { useCallback, useState, type SubmitEvent } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api } from '../../lib/api'
import { queryKeys } from '../../lib/queryKeys'
import { parseNumber } from '../../lib/format'
import { fieldErrors, useErrorText } from '../../lib/errors'
import { FUEL_TYPES, type CatalogEngine, type CatalogGeneration, type CatalogModel, type KbaVariant, type Vehicle } from '../../lib/types'
import { useSession } from '../../app/session'
import { Button } from '../../components/Button'
import { SelectField, TextField } from '../../components/Field'
import { NumberField } from '../../components/NumberField'
import { Card, ErrorBanner, Loading, PageHeader } from '../../components/ui'
import { useVehicle } from './hooks'
import { AvatarUpload } from './AvatarUpload'
import { HsnTsnQuickFill } from './HsnTsnQuickFill'
import { useAutoFill } from './useAutoFill'
import { useCatalogEngines, useCatalogGenerations, useCatalogMakes, useCatalogModels } from './catalog'
import { CreatableCombobox } from '../../components/CreatableCombobox'
import { useDebouncedValue } from '../../lib/useDebouncedValue'
import { useFormat } from '../../lib/useFormat'

export function VehicleFormPage() {
  const { id } = useParams()
  if (id) return <EditVehicle id={Number(id)} />
  return <VehicleForm />
}

function EditVehicle({ id }: { id: number }) {
  const vehicle = useVehicle(id)
  const errorText = useErrorText()
  if (vehicle.isPending) return <Loading />
  if (vehicle.isError) return <ErrorBanner message={errorText(vehicle.error)} />
  return <VehicleForm vehicle={vehicle.data} />
}

/** Raw form text per field; numbers are parsed on submit with the user's number format. */
export interface VehicleFormValues extends Record<string, string> {
  make: string
  model: string
  chassis: string
  engine: string
  /** Engine/trim label picked from the catalog; shown only, never stored (the engine code is). */
  trim: string
  year: string
  vin: string
  plate: string
  hsn: string
  tsn: string
  displacement: string
  power: string
  fuel: string
  odometer: string
}

const num = (n: number | null | undefined) => (n == null ? '' : String(n))

function initialValues(v?: Vehicle): VehicleFormValues {
  return {
    make: v?.make ?? '',
    model: v?.model ?? '',
    chassis: v?.chassis_code ?? '',
    engine: v?.engine_code ?? '',
    trim: '',
    year: num(v?.model_year),
    vin: v?.vin ?? '',
    plate: v?.license_plate ?? '',
    hsn: v?.hsn ?? '',
    tsn: v?.tsn ?? '',
    displacement: num(v?.displacement_cc),
    power: num(v?.power_kw),
    fuel: v?.fuel_type ?? '',
    odometer: '',
  }
}

/** Values a quick-fill sets; empty catalog values never overwrite the form (spec edge case "partial data"). */
function kbaFill(v: KbaVariant): Partial<VehicleFormValues> {
  const out: Partial<VehicleFormValues> = { make: v.make }
  if (v.model) out.model = v.model
  if (v.generation) out.chassis = v.generation
  if (v.engine_code) out.engine = v.engine_code
  if (v.displacement_cc != null) out.displacement = String(v.displacement_cc)
  if (v.power_kw != null) out.power = String(v.power_kw)
  if (v.fuel_type) out.fuel = v.fuel_type
  return out
}

type TaxonomyField = 'make' | 'model' | 'chassis' | 'trim'

/** Fields cleared when a taxonomy level changes (FR-015); technical values stay. */
const dependants: Record<TaxonomyField, TaxonomyField[]> = {
  make: ['model', 'chassis', 'trim'],
  model: ['chassis', 'trim'],
  chassis: ['trim'],
  trim: [],
}

const years = (from: number | null, to: number | null) =>
  from == null ? '' : to == null ? `${String(from)}–` : from === to ? String(from) : `${String(from)}–${String(to)}`

function VehicleForm({ vehicle }: { vehicle?: Vehicle }) {
  const { t } = useTranslation('vehicles')
  const { user } = useSession()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const errorText = useErrorText()
  const [values, setValues] = useState(() => initialValues(vehicle))
  const [local, setLocal] = useState<Record<string, string>>({})
  const auto = useAutoFill(values, setValues)
  const { touch, fill } = auto

  /** Manual edit: updates the value and drops it from the undo snapshot. */
  const set = useCallback(
    (field: keyof VehicleFormValues) => (value: string) => {
      setValues((prev) => ({ ...prev, [field]: value }))
      touch(field)
    },
    [touch],
  )
  const onKbaFill = useCallback((v: KbaVariant) => { fill(kbaFill(v)) }, [fill])

  // Cascading catalog suggestions (US2). Each level queries only while its
  // field has focus; parents are matched by normalized text on the server.
  const fmt = useFormat()
  const [focused, setFocused] = useState<TaxonomyField | null>(null)
  const focus = (field: TaxonomyField) => (on: boolean) => { setFocused((f) => (on ? field : f === field ? null : f)) }
  const q = useDebouncedValue(values, 120)
  const makes = useCatalogMakes(q.make, focused === 'make')
  const models = useCatalogModels(values.make, q.model, focused === 'model')
  const modelsAll = useCatalogModels(values.make, '', focused === 'model')
  const gens = useCatalogGenerations(values.make, values.model, q.chassis, focused === 'chassis')
  const gensAll = useCatalogGenerations(values.make, values.model, '', focused === 'chassis')
  const engines = useCatalogEngines(values.make, values.model, values.chassis, q.trim, focused === 'trim')
  const enginesAll = useCatalogEngines(values.make, values.model, values.chassis, '', focused === 'trim')

  /** A committed change of a taxonomy level clears the levels below it. */
  const commit = (field: TaxonomyField) => (_text: string, changed: boolean) => {
    if (!changed) return
    const cleared = Object.fromEntries(dependants[field].map((f) => [f, '']))
    setValues((prev) => ({ ...prev, ...cleared }))
  }
  const pickEngine = (e: CatalogEngine) => {
    setValues((prev) => ({ ...prev, trim: e.name }))
    fill({
      ...(e.engine_code ? { engine: e.engine_code } : {}),
      ...(e.power_kw != null ? { power: String(e.power_kw) } : {}),
      ...(e.displacement_cc != null ? { displacement: String(e.displacement_cc) } : {}),
      ...(e.fuel_type ? { fuel: e.fuel_type } : {}),
    })
  }
  const engineLabel = (e: CatalogEngine) => {
    const details = [
      e.engine_code || null,
      e.power_kw != null ? `${fmt.number(e.power_kw)} ${t('common:units.kw')}` : null,
      e.displacement_cc != null ? fmt.displacement(e.displacement_cc) : null,
    ].filter(Boolean)
    return details.length > 0 ? `${e.name} (${details.join(', ')})` : e.name
  }
  const generationLabel = (g: CatalogGeneration) => (g.body ? `${g.name} · ${g.body}` : g.name)
  const modelLabel = (m: CatalogModel) => {
    const y = years(m.year_from, m.year_to)
    return y ? `${m.name} (${y})` : m.name
  }

  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      vehicle
        ? api.patch<{ vehicle: Vehicle }>(`/vehicles/${vehicle.id}`, body)
        : api.post<{ vehicle: Vehicle }>('/vehicles', body),
    onSuccess: async ({ vehicle: saved }) => {
      await qc.invalidateQueries({ queryKey: queryKeys.vehicles })
      await qc.invalidateQueries({ queryKey: queryKeys.dashboard })
      void navigate(`/vehicles/${saved.id}`)
    },
  })
  const fe = { ...fieldErrors(save.error), ...local }

  const submit = (e: SubmitEvent) => {
    e.preventDefault()
    const v = values
    const errs: Record<string, string> = {}
    const yearNum = v.year.trim() === '' ? null : Number(v.year)
    if (yearNum !== null && !Number.isInteger(yearNum)) errs.model_year = 'validation.number'
    const ccNum = parseNumber(v.displacement, user.number_format)
    if (v.displacement.trim() !== '' && (ccNum === null || !Number.isInteger(ccNum))) errs.displacement_cc = 'validation.number'
    const kwNum = parseNumber(v.power, user.number_format)
    if (v.power.trim() !== '' && (kwNum === null || !Number.isInteger(kwNum))) errs.power_kw = 'validation.number'
    const body: Record<string, unknown> = {
      make: v.make, model: v.model, chassis_code: v.chassis, model_year: yearNum, engine_code: v.engine,
      vin: v.vin.trim() === '' ? null : v.vin, license_plate: v.plate.trim() === '' ? null : v.plate,
      // HSN/TSN are saved whenever present, whether or not they matched (FR-022).
      hsn: v.hsn === '' ? null : v.hsn, tsn: v.tsn === '' ? null : v.tsn,
      displacement_cc: ccNum, power_kw: kwNum, fuel_type: v.fuel === '' ? null : v.fuel,
    }
    if (!vehicle) {
      const km = parseNumber(v.odometer, user.number_format)
      if (km === null || !Number.isInteger(km) || km < 0) errs.current_odometer_km = 'validation.number'
      body.current_odometer_km = km
    }
    setLocal(errs)
    if (Object.keys(errs).length === 0) save.mutate(body)
  }

  const hi = auto.isHighlighted

  return (
    <div className="max-w-2xl">
      <PageHeader title={vehicle ? t('edit') : t('add')} />
      <Card>
        <form onSubmit={submit} className="space-y-4" noValidate>
          {save.isError && Object.keys(fieldErrors(save.error)).length === 0 && <ErrorBanner message={errorText(save.error)} />}
          <HsnTsnQuickFill
            hsn={values.hsn}
            tsn={values.tsn}
            onChange={(hsn, tsn) => { setValues((prev) => ({ ...prev, hsn, tsn })) }}
            onFill={onKbaFill}
            canUndo={auto.canUndo}
            onUndo={auto.undo}
            hsnError={fe.hsn}
            tsnError={fe.tsn}
          />
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <CreatableCombobox
              label={t('fields.make')}
              value={values.make}
              onChange={set('make')}
              onSelect={(name: string) => { set('make')(name) }}
              onCommit={commit('make')}
              onFocusChange={focus('make')}
              items={makes.data?.map((m) => m.name)}
              itemText={(name) => name}
              catalogAvailable
              loading={makes.isFetching}
              error={fe.make}
              highlighted={hi('make')}
              required
            />
            <CreatableCombobox
              label={t('fields.model')}
              value={values.model}
              onChange={set('model')}
              onSelect={(m: CatalogModel) => { set('model')(m.name) }}
              onCommit={commit('model')}
              onFocusChange={focus('model')}
              items={models.data}
              itemText={(m) => m.name}
              renderItem={modelLabel}
              catalogAvailable={(modelsAll.data?.length ?? 0) > 0}
              loading={models.isFetching}
              error={fe.model}
              highlighted={hi('model')}
              required
            />
            <CreatableCombobox
              label={t('fields.chassisCode')}
              value={values.chassis}
              onChange={set('chassis')}
              onSelect={(g: CatalogGeneration) => { set('chassis')(g.name) }}
              onCommit={commit('chassis')}
              onFocusChange={focus('chassis')}
              items={gens.data}
              itemText={(g) => g.name}
              renderItem={generationLabel}
              catalogAvailable={(gensAll.data?.length ?? 0) > 0}
              loading={gens.isFetching}
              error={fe.chassis_code}
              hint={t('fields.chassisCodeHint')}
              highlighted={hi('chassis')}
              optional
            />
            <CreatableCombobox
              label={t('fields.engineTrim')}
              value={values.trim}
              onChange={set('trim')}
              onSelect={pickEngine}
              onCommit={commit('trim')}
              onFocusChange={focus('trim')}
              items={engines.data}
              itemText={(e) => e.name}
              renderItem={engineLabel}
              catalogAvailable={(enginesAll.data?.length ?? 0) > 0}
              loading={engines.isFetching}
              hint={t('fields.engineTrimHint')}
              optional
            />
            <TextField label={t('fields.engineCode')} value={values.engine} onChange={set('engine')} error={fe.engine_code} hint={t('fields.engineCodeHint')} highlighted={hi('engine')} optional />
            <NumberField label={t('fields.displacement')} value={values.displacement} onChange={set('displacement')} error={fe.displacement_cc} unit={t('common:units.cc')} decimal={false} highlighted={hi('displacement')} optional />
            <NumberField label={t('fields.power')} value={values.power} onChange={set('power')} error={fe.power_kw} unit={t('common:units.kw')} decimal={false} highlighted={hi('power')} optional />
            <SelectField
              label={t('fields.fuelType')}
              value={values.fuel}
              onChange={set('fuel')}
              options={[{ value: '', label: t('fields.notSet') }, ...FUEL_TYPES.map((f) => ({ value: f, label: t(`fuel.${f}`) }))]}
              error={fe.fuel_type}
              highlighted={hi('fuel')}
              optional
            />
            <NumberField label={t('fields.modelYear')} value={values.year} onChange={set('year')} error={fe.model_year} decimal={false} optional />
            {!vehicle && (
              <NumberField label={t('fields.odometer')} value={values.odometer} onChange={set('odometer')} error={fe.current_odometer_km} unit={t('common:units.km')} decimal={false} />
            )}
            <TextField label={t('fields.vin')} value={values.vin} onChange={set('vin')} error={fe.vin} maxLength={17} optional />
            <TextField label={t('fields.licensePlate')} value={values.plate} onChange={set('plate')} error={fe.license_plate} maxLength={16} optional />
          </div>
          <div className="flex gap-2">
            <Button type="submit" variant="primary" disabled={save.isPending}>
              {save.isPending ? t('common:actions.saving') : t('common:actions.save')}
            </Button>
            <Button onClick={() => void navigate(-1)}>{t('common:actions.cancel')}</Button>
          </div>
        </form>
      </Card>
      {vehicle && <AvatarUpload vehicle={vehicle} />}
    </div>
  )
}
