import { describe, expect, it } from 'vitest'
import { screen } from '@testing-library/react'
import { renderWithApp, testSession } from '../../test/render'
import type { RepairPriority } from '../../lib/types'
import { PriorityBadge } from './priorities'
import { priorityBody } from './PrioritySelect'

const urgent: RepairPriority = { id: 1, builtin_key: 'urgent', custom_name: null, color: 'red', sort_order: 1, is_default: false, project_count: 0 }
const custom: RepairPriority = { id: 5, builtin_key: null, custom_name: 'Someday', color: 'violet', sort_order: 5, is_default: false, project_count: 0 }

describe('PriorityBadge', () => {
  it('shows the translated built-in name with its color', () => {
    renderWithApp(<PriorityBadge priority={urgent} />)
    expect(screen.getByText('Urgent')).toBeInTheDocument()
  })

  it('keeps the name accessible with color only', () => {
    renderWithApp(<PriorityBadge priority={custom} />, { ...testSession, user: { ...testSession.user, priority_display: 'color' } })
    expect(screen.queryByText('Someday')).toBeNull()
    expect(screen.getByRole('img', { name: 'Priority: Someday' })).toHaveAttribute('title', 'Someday')
  })

  it('shows only the name when asked', () => {
    const { container } = renderWithApp(<PriorityBadge priority={custom} display="name" />)
    expect(screen.getByText('Someday')).toBeInTheDocument()
    expect(container.querySelector('.bg-violet-400')).toBeNull()
  })
})

describe('priorityBody', () => {
  it('omits an untouched choice, so the server uses the default', () => {
    expect(priorityBody(null)).toEqual({})
    expect(priorityBody('')).toEqual({ priority_id: null })
    expect(priorityBody('3')).toEqual({ priority_id: 3 })
  })
})
