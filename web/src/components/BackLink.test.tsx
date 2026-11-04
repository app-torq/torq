import { describe, expect, it } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router'
import { I18nextProvider } from 'react-i18next'
import i18n from '../i18n'
import { BackLink } from './BackLink'

function renderAt(entries: string[]) {
  const router = createMemoryRouter(
    [
      { path: '/repairs', element: <p>board</p> },
      { path: '/dashboard', element: <p>dashboard</p> },
      { path: '/repairs/1', element: <BackLink fallback="/repairs" /> },
    ],
    { initialEntries: entries, initialIndex: entries.length - 1 },
  )
  render(<I18nextProvider i18n={i18n}><RouterProvider router={router} /></I18nextProvider>)
  return router
}

describe('BackLink', () => {
  it('goes to the fallback when the app was opened right here', () => {
    window.history.replaceState({ idx: 0 }, '')
    renderAt(['/repairs/1'])
    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    expect(screen.getByText('board')).toBeInTheDocument()
  })

  it('goes back to the previous screen when there is one', () => {
    window.history.replaceState({ idx: 1 }, '')
    renderAt(['/dashboard', '/repairs/1'])
    fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    expect(screen.getByText('dashboard')).toBeInTheDocument()
  })
})
