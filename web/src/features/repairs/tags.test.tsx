import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithApp } from '../../test/render'
import { normalizeTag, TagList } from './tags'

const tags = ['ABS', 'Brake fluid', 'Brembo', 'Rear axle', 'Rust']

describe('TagList', () => {
  it('shows the first tags and "+ n more" with the rest in the tooltip', () => {
    renderWithApp(<TagList tags={tags} limit={3} />)
    expect(screen.getAllByRole('listitem').map((li) => li.textContent)).toEqual(['ABS', 'Brake fluid', 'Brembo', '+ 2 more'])
    expect(screen.getByText('+ 2 more')).toHaveAttribute('title', 'Rear axle, Rust')
  })

  it('shows all tags with limit 0 or when they fit', () => {
    renderWithApp(<TagList tags={tags} limit={0} />)
    expect(screen.getAllByRole('listitem')).toHaveLength(5)
    expect(screen.queryByText(/more/)).toBeNull()
  })

  it('renders nothing without tags', () => {
    const { container } = renderWithApp(<TagList tags={[]} limit={3} />)
    expect(container.querySelector('ul')).toBeNull()
  })
})

describe('normalizeTag', () => {
  it('trims and collapses whitespace like the server', () => {
    expect(normalizeTag('  brake   fluid ')).toBe('brake fluid')
  })
})
