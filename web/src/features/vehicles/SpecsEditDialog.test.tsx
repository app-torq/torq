import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from '../../lib/api'
import type { VehicleSpecs } from '../../lib/types'
import { renderWithApp, testSession } from '../../test/render'
import { SpecsEditDialog } from './SpecSheet'

const specs: VehicleSpecs = {
  engine_oil_viscosity: '', engine_oil_spec: 'MB 229.31', engine_oil_capacity_ml: null,
  transmission_fluid_spec: '', transmission_fluid_capacity_ml: null,
  differential_fluid_spec: '', differential_fluid_capacity_ml: null,
  brake_fluid_spec: '', brake_fluid_capacity_ml: null,
  coolant_spec: '', coolant_capacity_ml: null,
  wheel_lug_torque_nm: null, tire_pressure_front_bar: null, tire_pressure_rear_bar: null,
  tire_pressure_front_max_bar: null, tire_pressure_rear_max_bar: null,
  meta: {},
}

const withAutofill = { ...testSession, instance: { ...testSession.instance, spec_autofill: true } }

afterEach(() => { vi.restoreAllMocks() })

describe('SpecsEditDialog auto-fill', () => {
  it('fills empty fields with source links, marks manual changes and saves both on Save', async () => {
    const user = userEvent.setup()
    vi.spyOn(api, 'post').mockResolvedValue({
      fields: {
        engine_oil_viscosity: { value: '5W-30', source_url: 'https://www.carwiki.de/w203', source_name: 'carwiki.de' },
        engine_oil_spec: { value: 'MB 229.51', source_url: 'https://www.carwiki.de/w203', source_name: 'carwiki.de' },
        engine_oil_capacity_l: { value: 6.5, source_url: 'https://www.liqui-moly.com/oel', source_name: 'liqui-moly.com' },
        differential_fluid_spec: { value: null, source_url: null, source_name: null },
        tire_pressure_front_bar: { value: 2.2, value_max: 2.4, source_url: 'https://www.tpressure.com/w203', source_name: 'tpressure.com' },
      },
    })
    const put = vi.spyOn(api, 'put').mockResolvedValue({})
    renderWithApp(<SpecsEditDialog vehicleId={7} specs={specs} onClose={() => undefined} />, withAutofill)

    await user.click(screen.getByRole('button', { name: 'Look up data automatically' }))
    await screen.findByText(/3 values found and filled in, 1 kept/)
    expect(screen.getByRole('textbox', { name: 'Tire pressure front' })).toHaveValue('2.2')
    expect(screen.getByRole('textbox', { name: 'Tire pressure front, up to' })).toHaveValue('2.4')

    const viscosity = screen.getByRole('textbox', { name: 'Viscosity' })
    expect(viscosity).toHaveValue('5W-30')
    expect(screen.getAllByRole('textbox', { name: 'Specification / approval' })[0]).toHaveValue('MB 229.31')
    const link = screen.getByRole('link', { name: /Filled automatically, source: liqui-moly.com/ })
    expect(link).toHaveAttribute('href', 'https://www.liqui-moly.com/oel')
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noopener noreferrer')
    expect(link).toHaveTextContent('Auto · liqui-moly.com')

    await user.clear(viscosity)
    await user.type(viscosity, '0W-30')
    expect(screen.getByText('Adjusted manually')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /source: carwiki.de/ })).toBeNull()
    expect(screen.getByRole('link', { name: /source: liqui-moly.com/ })).toBeInTheDocument()

    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Save' }))
    await waitFor(() => { expect(put).toHaveBeenCalledTimes(1) })
    const [path, body] = put.mock.calls[0] as [string, Record<string, unknown>]
    expect(path).toBe('/vehicles/7/specs')
    expect(body).toMatchObject({
      engine_oil_viscosity: '0W-30', engine_oil_spec: 'MB 229.31', engine_oil_capacity_ml: 6500,
      meta: {
        engine_oil_viscosity: { is_auto: false },
        engine_oil_capacity_ml: { is_auto: true, source_url: 'https://www.liqui-moly.com/oel', source_name: 'liqui-moly.com' },
      },
    })
  })

  it('explains that the lookup is not set up', () => {
    renderWithApp(<SpecsEditDialog vehicleId={7} specs={specs} onClose={() => undefined} />)
    expect(screen.getByRole('button', { name: 'Look up data automatically' })).toBeDisabled()
    expect(screen.getByText(/needs to set GEMINI_API_KEY and TAVILY_API_KEY/)).toBeInTheDocument()
  })
})
