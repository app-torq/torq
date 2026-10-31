import { useTranslation } from 'react-i18next'
import { Info } from 'lucide-react'
import { useFormat } from '../../lib/useFormat'
import type { ServiceCategory, VehicleSpecs } from '../../lib/types'
import { useSpecs } from '../vehicles/SpecSheet'

/** Shows the vehicle's stored fluid spec and capacity for fluid categories (FR-016). */
export function SpecHint({ vehicleId, category }: { vehicleId: number; category: ServiceCategory | undefined }) {
  const { t } = useTranslation('specs')
  const fmt = useFormat()
  const specs = useSpecs(vehicleId)
  if (!category?.builtin_key || !specs.data) return null
  const s: VehicleSpecs = specs.data.specs
  const cap = (ml: number | null) => (ml ? fmt.liters(ml) : null)
  const parts: (string | null)[] = (() => {
    switch (category.builtin_key) {
      case 'engine_oil':
        return [s.engine_oil_viscosity, s.engine_oil_spec, cap(s.engine_oil_capacity_ml)]
      case 'transmission_fluid':
        return [s.transmission_fluid_spec, cap(s.transmission_fluid_capacity_ml)]
      case 'differential_fluid':
        return [s.differential_fluid_spec, cap(s.differential_fluid_capacity_ml)]
      case 'brake_fluid':
        return [s.brake_fluid_spec, cap(s.brake_fluid_capacity_ml)]
      case 'coolant':
        return [s.coolant_spec, cap(s.coolant_capacity_ml)]
      default:
        return []
    }
  })()
  const value = parts.filter((p): p is string => !!p).join(' · ')
  if (!value) return null
  return (
    <p className="flex items-center gap-1.5 text-xs text-sky-300">
      <Info size={13} />
      {t('hint', { value })}
    </p>
  )
}
