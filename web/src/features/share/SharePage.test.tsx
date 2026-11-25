import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import i18n from '../../i18n'
import { renderWithApp } from '../../test/render'
import type { PublicVehicleRecord } from '../../lib/types'
import { SharePage, shareToken } from './SharePage'

const record: PublicVehicleRecord = {
  vehicle: {
    vehicle_type: 'car', make: 'Volkswagen', model: 'Golf', chassis_code: 'Mk7', model_year: 2015, engine_code: 'CJZA',
    hsn: null, tsn: null, displacement_cc: 1197, power_kw: 77, power_w: 77000, fuel_type: 'petrol', has_odometer: true,
    current_odometer_km: 120000, odometer_unit: 'km', displacement_unit: null, power_unit: null, hour_meters: [],
    license_plate: null, vin: null,
  },
  service_logs: [{
    service_date: '2026-03-01', odometer_km: 118000, odometer_recorded: true, meter_readings: [],
    items: [{ category: 'Engine oil', brand: 'Castrol', specification: '5W-30', quantity_ml: 4000, quantity_unit: 'L', oem_part_number: '', aftermarket_part_number: '', cost_cents: null }],
    extra_cost_cents: null, extra_cost_label: '', total_cents: null,
  }],
  reminders: [{
    name: 'Engine oil', state: 'overdue', trigger: 'km', next_due_date: null, next_due_km: 119000, next_due_hours: null,
    remaining_days: null, remaining_km: -1000, remaining_hours: null, hour_meter: null,
  }],
  specs: { engine_oil_spec: 'VW 504 00', engine_oil_capacity_ml: 4000 },
  spec_entries: [{ label: 'Wiper blades', value: '600/400 mm' }],
  formats: {
    date_format: 'YYYY-MM-DD', number_format: 'point', time_format: '24h', currency_code: 'EUR', power_unit: 'kw',
    distance_unit: 'km', volume_unit: 'l', torque_unit: 'nm', pressure_unit: 'bar', displacement_unit: 'cc',
  },
  branding: { name: 'Garage Nord', logo_url: null },
}

function respond(status: number, body: unknown) {
  return vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }))
}

afterEach(() => {
  vi.restoreAllMocks()
  void i18n.changeLanguage('en')
})

describe('SharePage', () => {
  it('reads the token from the path', () => {
    expect(shareToken('/s/abc-DEF_123')).toBe('abc-DEF_123')
  })

  it('shows the record without costs, plate or VIN when they are left out', async () => {
    const fetchSpy = respond(200, record)
    renderWithApp(<SharePage token="tok" />)
    expect(await screen.findByRole('heading', { name: 'Volkswagen Golf' })).toBeInTheDocument()
    expect(fetchSpy.mock.calls[0]?.[0]).toBe('/api/v1/shared/tok')
    expect(screen.getByText('Garage Nord')).toBeInTheDocument()
    expect(screen.getByText('Overdue')).toBeInTheDocument()
    expect(screen.getByText(/Castrol · 5W-30/)).toBeInTheDocument()
    expect(screen.getByText('Wiper blades: 600/400 mm')).toBeInTheDocument()
    expect(screen.getByText(/VW 504 00/)).toBeInTheDocument()
    expect(screen.queryByText('License plate')).toBeNull()
    expect(screen.queryByText('VIN')).toBeNull()
    expect(screen.queryByText(/€/)).toBeNull()
    expect(screen.getByRole('link', { name: 'Sign in to torq' })).toHaveAttribute('href', '/')
  })

  it('shows costs, plate and the app link when the record has them', async () => {
    respond(200, {
      ...record,
      vehicle_id: 7,
      vehicle: { ...record.vehicle, license_plate: 'HH-AB 123', vin: 'WVWZZZAUZFW000001' },
      service_logs: record.service_logs.map((l) => ({ ...l, total_cents: 8990 })),
    })
    renderWithApp(<SharePage token="tok" />)
    expect(await screen.findByText('HH-AB 123')).toBeInTheDocument()
    expect(screen.getByText('WVWZZZAUZFW000001')).toBeInTheDocument()
    expect(screen.getByText(/89\.90/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open in torq' })).toHaveAttribute('href', '/vehicles/7')
  })

  it('shows the not-found page for an unknown link', async () => {
    respond(404, { error: { code: 'not_found' } })
    renderWithApp(<SharePage token="nope" />)
    expect(await screen.findByRole('heading', { name: 'Link not found' })).toBeInTheDocument()
  })

  it('follows a German browser language', async () => {
    vi.spyOn(navigator, 'languages', 'get').mockReturnValue(['de-DE', 'de'])
    respond(200, record)
    renderWithApp(<SharePage token="tok" />)
    expect(await screen.findByText('Wartungsnachweis')).toBeInTheDocument()
    expect(screen.getByText('Wartungshistorie')).toBeInTheDocument()
  })
})
