import React, {
  ReactNode,
  useDeferredValue,
  useEffect,
  useMemo,
  useState,
} from 'react'
import { PaletteOptions } from '@mui/material/styles/createPalette'
import { isCypress } from '../env'
import {
  createTheme,
  Theme,
  ThemeProvider as MUIThemeProvider,
} from '@mui/material/styles'
import {
  argbFromHex,
  hexFromArgb,
  themeFromSourceColor,
  Scheme,
} from '@material/material-color-utilities'
import { blueGrey } from '@mui/material/colors'
import GlobalStyles from '@mui/material/GlobalStyles'
import {
  Appearance,
  playbookFont,
  playbookMono,
  playbookThemeOptions,
  playbookTokens,
} from './playbook'

interface ThemeProviderProps {
  children: ReactNode
  mode?: MUIThemeMode
}

interface ThemeContextParams {
  themeMode: string
  setThemeMode: (newMode: ThemeModeOption) => void
  sourceColor: string
  setSourceColor: (newColor: string) => void
  appearance: Appearance
  setAppearance: (newAppearance: Appearance) => void
}

export type MUIThemeMode = 'dark' | 'light'
type ThemeModeOption = 'dark' | 'light' | 'system'

export const ThemeContext = React.createContext<ThemeContextParams>({
  themeMode: '',
  setThemeMode: (): void => {},
  sourceColor: '',
  setSourceColor: (): void => {},
  appearance: 'existing',
  setAppearance: (): void => {},
})
ThemeContext.displayName = 'ThemeContext'

function makePalette(
  scheme: Scheme,
  useSurfaceVariant?: boolean,
): PaletteOptions {
  return {
    primary: {
      main: hexFromArgb(scheme.primary),
    },
    secondary: {
      main: hexFromArgb(scheme.secondary),
    },
    background: {
      default: useSurfaceVariant
        ? hexFromArgb(scheme.surfaceVariant)
        : hexFromArgb(scheme.background),
      paper: hexFromArgb(scheme.surface),
    },
    error: {
      main: hexFromArgb(scheme.error),
    },
  }
}

function validHexColor(hex: string | null): boolean {
  if (!hex) return false
  return /^#[0-9A-F]{6}$/i.test(hex)
}

function safeArgbFromHex(hex: string): number {
  if (!validHexColor(hex)) return argbFromHex(blueGrey[500])

  return argbFromHex(hex)
}

function getPalette(
  mode: MUIThemeMode,
  sourceColorHex: string,
): PaletteOptions {
  const sourceColor = safeArgbFromHex(sourceColorHex)
  const theme = themeFromSourceColor(sourceColor)

  if (mode === 'dark') {
    return {
      mode: 'dark',
      ...makePalette(theme.schemes.dark),
    }
  }

  return {
    mode: 'light',
    ...makePalette(theme.schemes.light, true),
  }
}

function makeTheme(
  mode: MUIThemeMode,
  sourceColor: string,
  appearance: Appearance = 'existing',
): Theme {
  let testOverrides = {}
  if (isCypress) {
    testOverrides = {
      transitions: {
        // So we have `transition: none;` everywhere
        create: () => 'none',
      },
    }
  }

  if (appearance === 'playbook') {
    return createTheme({ ...playbookThemeOptions(mode), ...testOverrides })
  }

  return createTheme({
    palette: getPalette(mode, sourceColor),
    components: {
      MuiIconButton: {
        defaultProps: {
          color: 'primary',
        },
      },
      MuiTextField: {
        defaultProps: {
          margin: 'dense',
        },
      },
      MuiBreadcrumbs: {
        styleOverrides: {
          separator: {
            margin: 4,
          },
        },
      },
    },
    ...testOverrides,
  })
}

function saveThemeMode(theme: ThemeModeOption): void {
  if (!window.localStorage) return
  window.localStorage.setItem('theme', theme)
}
function loadTheme(): ThemeModeOption {
  if (!window.localStorage) return 'system'

  const theme = window.localStorage.getItem('theme')
  switch (theme) {
    case 'dark':
    case 'light':
      return theme
  }

  return 'system'
}

function saveThemeColor(hex: string): void {
  if (!window.localStorage) return
  window.localStorage.setItem('themeColor', hex)
}
function loadThemeColor(): string {
  const savedColor = window?.localStorage?.getItem('themeColor')
  return validHexColor(savedColor) ? (savedColor as string) : blueGrey[500]
}

// The appearance (Existing / Playbook) is stored separately from the
// light/dark/system mode and the accent color, so switching it never
// changes those saved preferences.
function saveAppearance(appearance: Appearance): void {
  try {
    window.localStorage?.setItem('appearance', appearance)
  } catch {
    // storage unavailable (e.g. private mode); keep the in-memory choice
  }
}
function loadAppearance(): Appearance {
  try {
    return window.localStorage?.getItem('appearance') === 'playbook'
      ? 'playbook'
      : 'existing'
  } catch {
    return 'existing'
  }
}

function PlaybookGlobalStyles(props: { mode: MUIThemeMode }): JSX.Element {
  const t = playbookTokens(props.mode)
  return (
    <GlobalStyles
      styles={{
        ':root': { '--goalert-mono': playbookMono, colorScheme: props.mode },
        body: {
          fontFamily: playbookFont,
          fontSize: 14,
          lineHeight: 1.45,
          color: t.ink,
          backgroundColor: t.page,
          WebkitFontSmoothing: 'antialiased',
        },
        'code, kbd, pre, samp': { fontFamily: playbookMono },
        ':focus-visible': { outlineColor: t.soft },
      }}
    />
  )
}

export function ThemeProvider(props: ThemeProviderProps): JSX.Element {
  const [savedThemeMode, setSavedThemeMode] = useState(loadTheme())
  const [sourceColor, setSourceColor] = useState(loadThemeColor())
  const [appearance, setAppearanceState] =
    useState<Appearance>(loadAppearance())

  // used for watching if system theme mode changes
  const [systemThemeMode, setSystemThemeMode] = useState<MUIThemeMode>(
    window.matchMedia('(prefers-color-scheme: dark)').matches
      ? 'dark'
      : 'light',
  )

  useEffect(() => {
    const listener = (e: { matches: boolean }): void => {
      setSystemThemeMode(e.matches ? 'dark' : 'light')
    }
    window
      .matchMedia('(prefers-color-scheme: dark)')
      .addEventListener('change', listener)

    return () =>
      window
        .matchMedia('(prefers-color-scheme: dark)')
        .removeEventListener('change', listener)
  }, [])

  const mode =
    props.mode ||
    (savedThemeMode === 'system' ? systemThemeMode : savedThemeMode)
  // Use deferred and memoized values so we don't regenerate the entire theme on every render/change event
  const defMode = useDeferredValue(mode)
  const defSrc = useDeferredValue(sourceColor)
  const theme = useMemo(
    () => makeTheme(defMode, defSrc, appearance),
    [defMode, defSrc, appearance],
  )

  return (
    <ThemeContext.Provider
      value={{
        themeMode: savedThemeMode,
        setThemeMode: (newMode: ThemeModeOption) => {
          setSavedThemeMode(newMode)
          saveThemeMode(newMode)
        },
        sourceColor,
        setSourceColor: (newColor: string) => {
          setSourceColor(newColor)
          saveThemeColor(newColor)
        },
        appearance,
        setAppearance: (newAppearance: Appearance) => {
          setAppearanceState(newAppearance)
          saveAppearance(newAppearance)
        },
      }}
    >
      <MUIThemeProvider theme={theme}>
        {appearance === 'playbook' && <PlaybookGlobalStyles mode={defMode} />}
        {props.children}
      </MUIThemeProvider>
    </ThemeContext.Provider>
  )
}
