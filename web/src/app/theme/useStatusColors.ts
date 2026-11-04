import { useTheme } from '@mui/material'
import { yellow } from '@mui/material/colors'
import { useAppearance } from './useAppearance'
import { playbookTokens } from './playbook'

type StatusColors = {
  ok: string
  warn: string
  err: string
}

function useStatusColors(): StatusColors {
  const theme = useTheme()
  const appearance = useAppearance()

  if (appearance === 'playbook') {
    // Amber for acknowledged/warn, green for resolved/ok, red for errors.
    return {
      ok: theme.palette.success.main,
      warn: playbookTokens(theme.palette.mode).warnBorder,
      err: theme.palette.error.main,
    }
  }

  return {
    ok: theme.palette.success.main,
    warn: yellow[600], // TODO if practical, use theme.palette.warning.main,
    err: theme.palette.error.main,
  }
}

export default useStatusColors
