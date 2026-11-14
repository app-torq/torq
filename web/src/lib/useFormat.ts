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
          timeFormat: user.time_format,
          powerUnit: user.power_unit,
          distanceUnit: user.distance_unit,
          volumeUnit: user.volume_unit,
          torqueUnit: user.torque_unit,
          pressureUnit: user.pressure_unit,
          displacementUnit: user.displacement_unit,
        },
        (u: UnitKey) => t(`units.${u}`),
      ),
    [user, t],
  )
}
