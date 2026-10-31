import { act, renderHook, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { catalogKey } from '../lib/catalogKey'
import { useDebouncedValue } from '../lib/useDebouncedValue'
import { renderWithApp } from '../test/render'
import { CreatableCombobox } from './CreatableCombobox'

const makes = ['Mercedes-AMG', 'Mercedes-Benz', 'Citroën']

function Harness(props: { available?: boolean; onSelect?: (m: string) => void; onCommit?: (t: string, changed: boolean) => void }) {
  const [value, setValue] = useState('')
  const items = makes.filter((m) => catalogKey(m).includes(catalogKey(value)))
  return (
    <>
      <CreatableCombobox
        label="Make"
        value={value}
        onChange={setValue}
        onSelect={(m) => { setValue(m); props.onSelect?.(m) }}
        {...(props.onCommit ? { onCommit: props.onCommit } : {})}
        items={items}
        itemText={(m) => m}
        catalogAvailable={props.available ?? true}
      />
      <output data-testid="value">{value}</output>
      <button type="button">next</button>
    </>
  )
}

afterEach(() => { vi.useRealTimers() })

describe('catalogKey', () => {
  it('mirrors the server normalization', () => {
    expect(catalogKey('Mercedes-Benz')).toBe('mercedesbenz')
    expect(catalogKey('Citroën')).toBe('citroen')
    expect(catalogKey('Straße')).toBe('strasse')
    expect(catalogKey('W203 / S203')).toBe('w203s203')
  })
})

describe('CreatableCombobox', () => {
  it('exposes combobox/listbox semantics and navigates with the keyboard', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    renderWithApp(<Harness onSelect={onSelect} />)
    const input = screen.getByRole('combobox', { name: 'Make' })
    expect(input).toHaveAttribute('aria-expanded', 'false')
    await user.type(input, 'merc')
    expect(input).toHaveAttribute('aria-expanded', 'true')
    const list = screen.getByRole('listbox')
    expect(input).toHaveAttribute('aria-controls', list.id)
    const options = screen.getAllByRole('option')
    expect(options[0]).toHaveTextContent("Use custom: 'merc'")
    expect(options[0]).toHaveAttribute('aria-selected', 'true')
    expect(input).toHaveAttribute('aria-activedescendant', options[0]?.id)
    await user.keyboard('{End}')
    expect(options[2]).toHaveAttribute('aria-selected', 'true')
    await user.keyboard('{Home}{ArrowDown}{ArrowDown}{ArrowUp}{Enter}')
    expect(onSelect).toHaveBeenCalledWith('Mercedes-AMG')
    expect(screen.getByTestId('value')).toHaveTextContent('Mercedes-AMG')
    expect(input).toHaveAttribute('aria-expanded', 'false')
  })

  it('offers no custom option for an exact (normalized) match or empty input', async () => {
    const user = userEvent.setup()
    renderWithApp(<Harness />)
    const input = screen.getByRole('combobox', { name: 'Make' })
    await user.click(input)
    expect(screen.queryByText(/Use custom/)).not.toBeInTheDocument()
    await user.type(input, 'citroen')
    expect(screen.queryByText(/Use custom/)).not.toBeInTheDocument()
    expect(screen.getByRole('option')).toHaveTextContent('Citroën')
  })

  it('keeps free text on Escape, Tab and blur, and reports commits', async () => {
    const user = userEvent.setup()
    const onCommit = vi.fn()
    renderWithApp(<Harness onCommit={onCommit} />)
    const input = screen.getByRole('combobox', { name: 'Make' })
    await user.type(input, 'Trabbi Custom')
    await user.keyboard('{Escape}')
    expect(input).toHaveAttribute('aria-expanded', 'false')
    expect(input).toHaveValue('Trabbi Custom')
    await user.tab()
    expect(screen.getByRole('button', { name: 'next' })).toHaveFocus()
    expect(screen.getByTestId('value')).toHaveTextContent('Trabbi Custom')
    expect(onCommit).toHaveBeenLastCalledWith('Trabbi Custom', true)
  })

  it('chooses the custom option with Enter', async () => {
    const user = userEvent.setup()
    const onCommit = vi.fn()
    renderWithApp(<Harness onCommit={onCommit} />)
    await user.type(screen.getByRole('combobox', { name: 'Make' }), 'Trabant  {Enter}')
    expect(screen.getByTestId('value')).toHaveTextContent('Trabant')
    expect(onCommit).toHaveBeenCalledWith('Trabant', true)
  })

  it('is a plain text input when the catalog does not know the level', async () => {
    const user = userEvent.setup()
    renderWithApp(<Harness available={false} />)
    const input = screen.getByRole('combobox', { name: 'Make' })
    await user.type(input, 'merc')
    expect(input).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('option')).not.toBeInTheDocument()
  })
})

describe('useDebouncedValue', () => {
  it('waits 120 ms after the last change', () => {
    vi.useFakeTimers()
    const { result, rerender } = renderHook(({ v }) => useDebouncedValue(v, 120), { initialProps: { v: 'a' } })
    rerender({ v: 'ab' })
    act(() => { vi.advanceTimersByTime(100) })
    rerender({ v: 'abc' })
    act(() => { vi.advanceTimersByTime(100) })
    expect(result.current).toBe('a')
    act(() => { vi.advanceTimersByTime(20) })
    expect(result.current).toBe('abc')
  })
})
