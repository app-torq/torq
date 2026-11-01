import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { ApiError } from './api'

/** Translates an API error (or any error) into a user-facing message. */
export function useErrorText(): (err: unknown) => string {
  const { t } = useTranslation('errors')
  return useCallback(
    (err: unknown) => {
      if (err instanceof ApiError) {
        return t(err.code, { ...err.params, defaultValue: t('generic') })
      }
      return t('generic')
    },
    [t],
  )
}

/** Returns field-level error codes from a validation error. */
export function fieldErrors(err: unknown): Record<string, string> {
  return err instanceof ApiError ? err.fields : {}
}
