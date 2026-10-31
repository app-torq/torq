import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useSession } from '../app/session'
import { createFormatter, type Formatter, type UnitKey } from './format'

/** Formatter bound to the current user's preferences. */
export function useFormat(): Formatter {
  const { user } = useSession()
  const { t } = useTranslation('common')
  return useMemo(
    () =>
      createFormatter(
        {
          dateFormat: user.date_format,
          numberFormat: user.number_format,
          currencyCode: user.currency_code,
          currencySymbol: user.currency_symbol,
          powerUnit: user.power_unit,
        },
        (u: UnitKey) => t(`units.${u}`),
      ),
    [user.date_format, user.number_format, user.currency_code, user.currency_symbol, user.power_unit, t],
  )
}
