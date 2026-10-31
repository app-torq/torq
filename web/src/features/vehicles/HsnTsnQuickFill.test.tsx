import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../../lib/api'
import type { KbaVariant } from '../../lib/types'
import { renderWithApp } from '../../test/render'
import { HsnTsnQuickFill } from './HsnTsnQuickFill'

const variant = (over: Partial<KbaVariant>): KbaVariant => ({
  make: 'Mercedes-Benz', model: 'C-Klasse', trade_name: 'C 220 CDI', generation: '', engine_code: '',
  displacement_cc: null, power_kw: null, fuel_type: null, body_code: '', year_from: null, match: 'unique', ...over,
})

const catalog: Record<string, KbaVariant[]> = {
  '0710/472': [variant({})],
  '1313/ADL': [variant({}), variant({ model: 'CLC 220 CDI', trade_name: 'CLC 220 CDI', body_code: 'AD', power_kw: 110 })],
}

function Harness({ onFill }: { onFill: (v: KbaVariant) => void }) {
  const [key, setKey] = useState({ hsn: '', tsn: '' })
  return (
    <HsnTsnQuickFill
      hsn={key.hsn}
      tsn={key.tsn}
      onChange={(hsn, tsn) => { setKey({ hsn, tsn }) }}
      onFill={onFill}
      canUndo={false}
      onUndo={() => undefined}
    />
  )
}

let get: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  get = vi.spyOn(api, 'get').mockImplementation((_path: string, query?: Record<string, unknown>) =>
    Promise.resolve({ items: catalog[`${String(query?.hsn)}/${String(query?.tsn)}`] ?? [] }),
  )
})
afterEach(() => { vi.restoreAllMocks() })

describe('HsnTsnQuickFill', () => {
  it('accepts digits only, auto-advances to TSN, upper-cases and fills a single match', async () => {
    const user = userEvent.setup()
    const onFill = vi.fn()
    renderWithApp(<Harness onFill={onFill} />)
    const hsn = screen.getByLabelText('HSN (2.1)')
    const tsn = screen.getByLabelText('TSN (2.2)')
    await user.type(hsn, '07a10')
    expect(hsn).toHaveValue('0710')
    expect(tsn).toHaveFocus()
    await user.keyboard('47')
    expect(get).not.toHaveBeenCalled()
    await user.keyboard('2')
    expect(tsn).toHaveValue('472')
    await waitFor(() => { expect(onFill).toHaveBeenCalledTimes(1) })
    expect(onFill.mock.calls[0]?.[0]).toMatchObject({ make: 'Mercedes-Benz', model: 'C-Klasse' })
    expect(get).toHaveBeenCalledWith('/catalog/kba', { hsn: '0710', tsn: '472', lang: 'en' })
  })

  it('rejects non-alphanumerics in TSN and splits pasted keys', async () => {
    const user = userEvent.setup()
    const onFill = vi.fn()
    renderWithApp(<Harness onFill={onFill} />)
    const tsn = screen.getByLabelText('TSN (2.2)')
    await user.click(tsn)
    await user.keyboard('a-b')
    expect(tsn).toHaveValue('AB')
    await user.clear(tsn)
    await user.click(screen.getByLabelText('HSN (2.1)'))
    await user.paste('0710/472123')
    expect(screen.getByLabelText('HSN (2.1)')).toHaveValue('0710')
    expect(tsn).toHaveValue('472')
    await waitFor(() => { expect(onFill).toHaveBeenCalledTimes(1) })
  })

  it('shows variant pills and fills only after a pick', async () => {
    const user = userEvent.setup()
    const onFill = vi.fn()
    renderWithApp(<Harness onFill={onFill} />)
    await user.click(screen.getByLabelText('HSN (2.1)'))
    await user.paste('1313 adl')
    const pill = await screen.findByRole('button', { name: /CLC 220 CDI/ })
    expect(pill).toHaveTextContent('Coupé')
    expect(pill).toHaveTextContent('110 kW (150 PS)')
    expect(onFill).not.toHaveBeenCalled()
    pill.focus()
    await user.keyboard('{Enter}')
    expect(onFill).toHaveBeenCalledTimes(1)
    expect(onFill.mock.calls[0]?.[0]).toMatchObject({ trade_name: 'CLC 220 CDI' })
    expect(screen.queryByRole('button', { name: /CLC 220 CDI/ })).not.toBeInTheDocument()
  })

  it('shows a non-blocking hint when the key is unknown', async () => {
    const user = userEvent.setup()
    const onFill = vi.fn()
    renderWithApp(<Harness onFill={onFill} />)
    await user.type(screen.getByLabelText('HSN (2.1)'), '9999ZZZ')
    expect(await screen.findByText(/not found in local catalog/)).toBeInTheDocument()
    expect(onFill).not.toHaveBeenCalled()
  })

  it('does not look up a key it was opened with', () => {
    renderWithApp(
      <HsnTsnQuickFill hsn="0710" tsn="472" onChange={() => undefined} onFill={() => undefined} canUndo={false} onUndo={() => undefined} />,
    )
    expect(get).not.toHaveBeenCalled()
  })
})
