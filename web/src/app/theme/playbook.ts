import { ThemeOptions } from '@mui/material/styles'

// Playbook appearance: design tokens taken from the OpsPlaybook console mock
// (light) plus a matching dark variant. Fonts are bundled locally via
// @fontsource (Inter, JetBrains Mono — SIL Open Font License 1.1); no
// external font service is used.

export type Appearance = 'existing' | 'playbook'

declare module '@mui/material/styles' {
  interface Theme {
    // Set only by the Playbook appearance (white app bar instead of colored).
    goalertAppearance?: 'playbook'
  }
  interface ThemeOptions {
    goalertAppearance?: 'playbook'
  }
}

export const playbookFont =
  "'Inter', system-ui, -apple-system, 'Segoe UI', Roboto, Arial, sans-serif"
export const playbookMono =
  "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"

export interface PlaybookTokens {
  nav: string // sidebar background
  nav2: string // active nav item background
  navInk: string
  navMuted: string
  navLabel: string // group labels (orange)
  accent: string // brand orange (active bar, logo)
  accentBtn: string
  accentBtnHover: string
  accentTint: string
  accentInk: string
  page: string
  card: string
  sunk: string
  ink: string
  muted: string
  line: string
  line2: string
  soft: string // teal: focus, toggles
  softTint: string
  softInk: string // links, selected rows
  ok: string
  okBg: string
  bad: string
  badBg: string
  warn: string // amber text
  warnBg: string
  warnBorder: string // amber used as a status border/indicator
  neutral: string
  neutralBg: string
  hover: string
  shadow: string
}

const light: PlaybookTokens = {
  nav: '#133B49',
  nav2: '#1D4D5E',
  navInk: '#FFFFFF',
  navMuted: '#A7C0C8',
  navLabel: '#F28A4B',
  accent: '#E55608',
  accentBtn: '#C94A06',
  accentBtnHover: '#A83E05',
  accentTint: '#FCEDE4',
  accentInk: '#8A3305',
  page: '#EEF2F4',
  card: '#FFFFFF',
  sunk: '#F5F8F9',
  ink: '#212529',
  muted: '#71767B',
  line: '#DCE3E7',
  line2: '#EDF1F3',
  soft: '#4CB7B7',
  softTint: '#E2F4F4',
  softInk: '#1C6A6B',
  ok: '#1F6E4C',
  okBg: '#E1F2E9',
  bad: '#B42318',
  badBg: '#FBE7E4',
  warn: '#835400',
  warnBg: '#FFF1D2',
  warnBorder: '#E29A12',
  neutral: '#4F5A63',
  neutralBg: '#E7ECEF',
  hover: '#F3F8F8',
  shadow: '0 6px 20px rgba(19,59,73,.18)',
}

// Dark variant: same structure and typography; colors chosen for WCAG AA
// contrast on the dark surfaces (not part of the original mock).
const dark: PlaybookTokens = {
  nav: '#0B252E',
  nav2: '#16404E',
  navInk: '#FFFFFF',
  navMuted: '#9DB6BE',
  navLabel: '#F28A4B',
  accent: '#F07A33',
  accentBtn: '#C94A06',
  accentBtnHover: '#E05A0F',
  accentTint: '#3A2416',
  accentInk: '#F6B58C',
  page: '#0E1A1F',
  card: '#15252C',
  sunk: '#112027',
  ink: '#E6EDF0',
  muted: '#9AA8AE',
  line: '#29434D',
  line2: '#1E3640',
  soft: '#4CB7B7',
  softTint: '#173B3D',
  softInk: '#8ED6D6',
  ok: '#6DD3A1',
  okBg: '#133628',
  bad: '#F4897E',
  badBg: '#3D1A17',
  warn: '#F2C060',
  warnBg: '#3A2C0E',
  warnBorder: '#E29A12',
  neutral: '#B4C0C6',
  neutralBg: '#24343B',
  hover: '#1A2E36',
  shadow: '0 6px 24px rgba(0,0,0,.45)',
}

export function playbookTokens(mode: 'light' | 'dark'): PlaybookTokens {
  return mode === 'dark' ? dark : light
}

export function playbookThemeOptions(mode: 'light' | 'dark'): ThemeOptions {
  const t = playbookTokens(mode)
  const radius = 3

  return {
    goalertAppearance: 'playbook',
    shape: { borderRadius: 4 },
    palette: {
      mode,
      primary: { main: t.accentBtn, contrastText: '#FFFFFF' },
      secondary: {
        main: mode === 'dark' ? t.soft : t.softInk,
        contrastText: mode === 'dark' ? '#0B252E' : '#FFFFFF',
      },
      error: { main: mode === 'dark' ? '#E5574A' : t.bad },
      warning: { main: t.warnBorder },
      success: { main: mode === 'dark' ? '#3FA877' : t.ok },
      info: { main: mode === 'dark' ? t.soft : t.softInk },
      background: { default: t.page, paper: t.card },
      text: { primary: t.ink, secondary: t.muted },
      divider: t.line,
      action: { hover: t.hover, selected: t.softTint },
    },
    typography: {
      fontFamily: playbookFont,
      fontSize: 14,
      htmlFontSize: 16,
      h1: { fontSize: '2rem', fontWeight: 600, letterSpacing: '-0.02em' },
      h2: { fontSize: '1.625rem', fontWeight: 600, letterSpacing: '-0.02em' },
      h3: { fontSize: '1.375rem', fontWeight: 600, letterSpacing: '-0.01em' },
      h4: { fontSize: '1.25rem', fontWeight: 600, letterSpacing: '-0.01em' },
      h5: { fontSize: '1.1875rem', fontWeight: 600, letterSpacing: '-0.01em' }, // 19px page titles
      h6: { fontSize: '1rem', fontWeight: 600, lineHeight: 1.4 }, // 16px
      subtitle1: { fontSize: '0.875rem', fontWeight: 500, lineHeight: 1.45 },
      subtitle2: { fontSize: '0.8125rem', fontWeight: 600, lineHeight: 1.45 },
      body1: { fontSize: '0.875rem', lineHeight: 1.45 }, // 14px
      body2: { fontSize: '0.8125rem', lineHeight: 1.45 }, // 13px
      caption: { fontSize: '0.75rem', lineHeight: 1.4 },
      overline: {
        fontSize: '0.65625rem',
        fontWeight: 600,
        letterSpacing: '0.09em',
        lineHeight: 1.6,
      },
      button: {
        fontSize: '0.8125rem',
        fontWeight: 600,
        textTransform: 'none',
        letterSpacing: 0,
      },
    },
    components: {
      MuiIconButton: { defaultProps: { color: 'primary' } },
      MuiTextField: { defaultProps: { margin: 'dense' } },
      MuiBreadcrumbs: { styleOverrides: { separator: { margin: 4 } } },
      MuiAppBar: {
        defaultProps: { elevation: 0, color: 'inherit' },
        styleOverrides: {
          root: {
            backgroundColor: t.card,
            color: t.ink,
            borderBottom: `1px solid ${t.line}`,
            backgroundImage: 'none',
            '& .MuiIconButton-root': { color: t.ink },
          },
        },
      },
      MuiPaper: {
        styleOverrides: {
          root: { backgroundImage: 'none' },
          elevation1: { boxShadow: 'none', border: `1px solid ${t.line}` },
          elevation2: { boxShadow: 'none', border: `1px solid ${t.line}` },
          rounded: { borderRadius: 4 },
        },
      },
      MuiCard: {
        defaultProps: { elevation: 1 },
        styleOverrides: { root: { borderRadius: 4 } },
      },
      MuiCardHeader: {
        styleOverrides: {
          root: { padding: '12px 16px', borderBottom: `1px solid ${t.line2}` },
          title: { fontSize: '0.875rem', fontWeight: 600 },
          subheader: { fontSize: '0.78125rem', color: t.muted },
        },
      },
      MuiCardContent: { styleOverrides: { root: { padding: 16 } } },
      MuiDialog: {
        styleOverrides: {
          paper: { boxShadow: t.shadow, border: `1px solid ${t.line}` },
        },
      },
      MuiDialogTitle: {
        styleOverrides: { root: { fontSize: '1rem', fontWeight: 600 } },
      },
      MuiPopover: {
        styleOverrides: {
          paper: { boxShadow: t.shadow, border: `1px solid ${t.line}` },
        },
      },
      MuiMenu: {
        styleOverrides: {
          paper: { boxShadow: t.shadow, border: `1px solid ${t.line}` },
        },
      },
      MuiButton: {
        defaultProps: { disableElevation: true },
        styleOverrides: {
          root: { borderRadius: radius, minHeight: 34, padding: '6px 14px' },
          sizeSmall: {
            minHeight: 28,
            padding: '3px 10px',
            fontSize: '0.78125rem',
          },
          containedPrimary: {
            backgroundColor: t.accentBtn,
            '&:hover': { backgroundColor: t.accentBtnHover },
          },
          outlined: {
            borderColor: t.line,
            color: t.ink,
            backgroundColor: t.card,
            '&:hover': {
              borderColor: mode === 'dark' ? '#3B5A66' : '#B9C4CA',
              backgroundColor: t.sunk,
            },
          },
          text: { color: mode === 'dark' ? t.softInk : t.softInk },
        },
      },
      MuiFab: {
        styleOverrides: {
          primary: {
            backgroundColor: t.accentBtn,
            '&:hover': { backgroundColor: t.accentBtnHover },
          },
        },
      },
      MuiToggleButton: {
        styleOverrides: {
          root: {
            textTransform: 'none',
            fontWeight: 500,
            fontSize: '0.8125rem',
            borderRadius: radius,
            borderColor: t.line,
            color: t.ink,
            padding: '6px 12px',
            '&.Mui-selected, &.Mui-selected:hover': {
              backgroundColor: mode === 'dark' ? t.nav2 : t.nav,
              color: '#FFFFFF',
            },
          },
        },
      },
      MuiChip: {
        styleOverrides: {
          root: {
            borderRadius: radius,
            fontWeight: 500,
            fontSize: '0.78125rem',
          },
        },
      },
      MuiTabs: {
        styleOverrides: { indicator: { backgroundColor: t.accent, height: 2 } },
      },
      MuiTab: {
        styleOverrides: {
          root: {
            textTransform: 'none',
            fontWeight: 500,
            fontSize: '0.8125rem',
            color: t.muted,
            minHeight: 44,
            '&.Mui-selected': { color: t.ink, fontWeight: 600 },
          },
        },
      },
      MuiTableCell: {
        styleOverrides: {
          root: {
            borderBottom: `1px solid ${t.line2}`,
            padding: '10px 12px',
            fontSize: '0.8125rem',
          },
          head: {
            fontSize: '0.65625rem',
            fontWeight: 600,
            letterSpacing: '0.06em',
            textTransform: 'uppercase',
            color: t.muted,
            padding: '9px 12px',
            borderBottom: `1px solid ${t.line}`,
            whiteSpace: 'nowrap',
          },
        },
      },
      MuiListItemText: {
        styleOverrides: {
          primary: { fontSize: '0.875rem', fontWeight: 500 },
          secondary: { fontSize: '0.78125rem', color: t.muted },
        },
      },
      MuiListSubheader: {
        styleOverrides: {
          root: {
            fontSize: '0.65625rem',
            fontWeight: 600,
            letterSpacing: '0.09em',
            textTransform: 'uppercase',
            color: t.muted,
            lineHeight: '32px',
          },
        },
      },
      MuiOutlinedInput: {
        styleOverrides: {
          root: {
            borderRadius: radius,
            '& .MuiOutlinedInput-notchedOutline': { borderColor: t.line },
            '&:hover .MuiOutlinedInput-notchedOutline': {
              borderColor: mode === 'dark' ? '#3B5A66' : '#B9C4CA',
            },
            '&.Mui-focused .MuiOutlinedInput-notchedOutline': {
              borderColor: t.soft,
              borderWidth: 1,
            },
            '&.Mui-focused': { boxShadow: `0 0 0 3px ${t.softTint}` },
            '&.Mui-error .MuiOutlinedInput-notchedOutline': {
              borderColor: t.bad,
            },
          },
        },
      },
      MuiFilledInput: {
        styleOverrides: {
          root: { borderRadius: `${radius}px ${radius}px 0 0` },
        },
      },
      MuiInputLabel: {
        styleOverrides: { root: { '&.Mui-focused': { color: t.softInk } } },
      },
      MuiSwitch: { defaultProps: { color: 'secondary' } },
      MuiCheckbox: { defaultProps: { color: 'secondary' } },
      MuiRadio: { defaultProps: { color: 'secondary' } },
      MuiLink: {
        styleOverrides: { root: { color: t.softInk, textUnderlineOffset: 3 } },
      },
      MuiTooltip: {
        styleOverrides: {
          tooltip: {
            backgroundColor: t.nav,
            fontSize: '0.75rem',
            borderRadius: radius,
          },
        },
      },
      MuiDivider: { styleOverrides: { root: { borderColor: t.line } } },
      MuiAlert: { styleOverrides: { root: { borderRadius: 4 } } },
    },
  }
}
