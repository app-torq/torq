import { NavLink, Outlet } from 'react-router'
import { useTranslation } from 'react-i18next'
import { settingsNav } from '../../app/nav'
import { useSession } from '../../app/session'

export function SettingsLayout() {
  const { t } = useTranslation()
  const { user } = useSession()
  return (
    <div className="flex flex-col gap-6 lg:flex-row">
      <nav aria-label={t('nav.settings')} className="flex gap-1 overflow-x-auto lg:w-52 lg:shrink-0 lg:flex-col">
        {settingsNav
          .filter((i) => !i.adminOnly || user.role === 'admin')
          .map((i) => (
            <NavLink
              key={i.to}
              to={i.to}
              className={({ isActive }) =>
                `focus-ring whitespace-nowrap rounded-md px-3 py-2 text-sm ${isActive ? 'bg-zinc-800 text-zinc-100' : 'text-zinc-400 hover:bg-zinc-900 hover:text-zinc-100'}`
              }
            >
              {t(i.labelKey)}
            </NavLink>
          ))}
      </nav>
      <div className="min-w-0 flex-1">
        <Outlet />
      </div>
    </div>
  )
}
