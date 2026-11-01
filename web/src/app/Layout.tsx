import { useCallback, useState } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { LogOut, Menu, Search, X } from 'lucide-react'
import { api } from '../lib/api'
import { QUICK_ACTIONS_HOTKEY } from '../lib/brand'
import { queryKeys } from '../lib/queryKeys'
import { IconButton } from '../components/Button'
import { HomeLink } from '../components/Logo'
import { mainNav } from './nav'
import { QuickActions, useQuickActionsHotkey } from './QuickActions'
import { VehicleSwitcher } from './VehicleSwitcher'
import { useSession } from './session'

export function Layout() {
  const { t } = useTranslation()
  const { user } = useSession()
  const qc = useQueryClient()
  const [drawer, setDrawer] = useState(false)
  const [quickOpen, setQuickOpen] = useState(false)
  const openQuick = useCallback(() => { setQuickOpen(true) }, [])
  useQuickActionsHotkey(openQuick)
  const location = useLocation()

  const logout = useMutation({
    mutationFn: () => api.post('/auth/logout'),
    onSettled: () => {
      qc.clear()
      qc.setQueryData(queryKeys.setupStatus, { setup_required: false })
      qc.setQueryData(queryKeys.me, null)
    },
  })

  const nav = (
    <nav aria-label={t('nav.main')} className="flex flex-col gap-0.5 px-3">
      {mainNav.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end ?? false}
          onClick={() => { setDrawer(false) }}
          className={({ isActive }) =>
            `focus-ring flex items-center gap-3 rounded-md px-3 py-2 text-sm ${isActive ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:bg-zinc-800/60 hover:text-zinc-100'}`
          }
        >
          <item.icon size={18} />
          {t(item.labelKey)}
        </NavLink>
      ))}
    </nav>
  )

  const userBox = (
    <div className="border-t border-zinc-800 px-3 py-3">
      <div className="flex items-center justify-between gap-2 px-3">
        <div className="min-w-0">
          <div className="truncate text-sm text-zinc-200">{user.display_name}</div>
          <div className="truncate text-xs text-zinc-400">{user.username}</div>
        </div>
        <IconButton aria-label={t('actions.logout')} title={t('actions.logout')} onClick={() => { logout.mutate() }}>
          <LogOut size={16} />
        </IconButton>
      </div>
    </div>
  )

  const quickButton = (
    <button
      type="button"
      onClick={openQuick}
      className="focus-ring flex w-full items-center gap-2 rounded-md border border-zinc-800 bg-zinc-950 px-3 py-2 text-left text-sm text-zinc-400 hover:border-zinc-700 hover:text-zinc-300"
    >
      <Search size={15} />
      <span className="flex-1">{t('actions.quickActions')}</span>
      <kbd className="rounded border border-zinc-700 px-1 text-[10px] text-zinc-400">{QUICK_ACTIONS_HOTKEY}</kbd>
    </button>
  )

  return (
    <div className="flex min-h-dvh">
      <aside className="sticky top-0 hidden h-dvh w-64 shrink-0 flex-col border-r border-zinc-800 bg-zinc-900 md:flex">
        <div className="px-6 py-5">
          <HomeLink className="h-9" />
        </div>
        <div className="space-y-3 px-3 pb-3">
          {quickButton}
          <VehicleSwitcher />
        </div>
        <div className="flex-1 overflow-y-auto">{nav}</div>
        {userBox}
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-20 flex items-center justify-between border-b border-zinc-800 bg-zinc-900/95 px-4 py-2 backdrop-blur md:hidden">
          <IconButton aria-label={t('nav.openMenu')} onClick={() => { setDrawer(true) }}>
            <Menu size={20} />
          </IconButton>
          <HomeLink className="h-8" />
          <IconButton aria-label={t('actions.quickActions')} onClick={openQuick}>
            <Search size={18} />
          </IconButton>
        </header>

        {drawer && (
          <div className="fixed inset-0 z-30 md:hidden">
            <button type="button" aria-label={t('nav.closeMenu')} className="absolute inset-0 bg-zinc-950/80" onClick={() => { setDrawer(false) }} />
            <div className="absolute inset-y-0 left-0 flex w-72 max-w-[85%] flex-col border-r border-zinc-800 bg-zinc-900">
              <div className="flex items-center justify-between px-5 py-4">
                <HomeLink className="h-8" onClick={() => { setDrawer(false) }} />
                <IconButton aria-label={t('nav.closeMenu')} onClick={() => { setDrawer(false) }}>
                  <X size={18} />
                </IconButton>
              </div>
              <div className="px-3 pb-3"><VehicleSwitcher onNavigate={() => { setDrawer(false) }} /></div>
              <div className="flex-1 overflow-y-auto">{nav}</div>
              {userBox}
            </div>
          </div>
        )}

        <main key={location.pathname.split('/')[1]} className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 md:px-8 md:py-8">
          <Outlet />
        </main>
      </div>

      <QuickActions open={quickOpen} onClose={() => { setQuickOpen(false) }} />
    </div>
  )
}
