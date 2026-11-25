import { screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { Vehicle } from '../../lib/types'
import { renderWithApp } from '../../test/render'
import { VehicleReadingUpdate } from './VehicleReadingUpdate'

const base: Vehicle = {
  id: 1, vehicle_type: 'car', make: 'Lux', model: 'Schneefräse', chassis_code: '', model_year: null, engine_code: '',
  vin: null, license_plate: null, hsn: null, tsn: null, displacement_cc: null, power_kw: null, power_w: null, fuel_type: null,
  current_odometer_km: 0, odometer_unit: 'km', has_odometer: true, hour_meters: [], displacement_unit: null, power_unit: null,
  avatar_url: null, archived: false, reminder_counts: { due_soon: 0, overdue: 0 }, created_at: 'x', updated_at: 'x',
}
const meter = { id: 3, name: 'Engine hours', current_hours: 20 }

// #39: the inputs follow what the vehicle is tracked by.
describe('VehicleReadingUpdate', () => {
  it('shows only the odometer for a vehicle without an hour meter', () => {
    renderWithApp(<VehicleReadingUpdate vehicle={base} compact />)
    expect(screen.getByRole('textbox', { name: 'New odometer reading' })).toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: /New reading/ })).toBeNull()
  })

  it('shows only the hours for a vehicle tracked by hours only', () => {
    renderWithApp(<VehicleReadingUpdate vehicle={{ ...base, has_odometer: false, hour_meters: [meter] }} compact />)
    expect(screen.getByRole('textbox', { name: 'New reading (Engine hours)' })).toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: 'New odometer reading' })).toBeNull()
  })

  it('shows both for a vehicle tracked by odometer and hours, focusing the first', () => {
    renderWithApp(<VehicleReadingUpdate vehicle={{ ...base, hour_meters: [meter] }} autoFocus />)
    expect(screen.getByRole('textbox', { name: 'New odometer reading' })).toHaveFocus()
    expect(screen.getByRole('textbox', { name: 'New reading (Engine hours)' })).toBeInTheDocument()
  })
})
