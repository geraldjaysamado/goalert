import React from 'react'
import { renderToString } from 'react-dom/server'
import createCache from '@emotion/cache'
import Box from '@mui/material/Box'

// Regression test: with Content-Security-Policy enabled, every <style> the app
// injects must carry the CSP nonce. ThemeProvider renders global styles (the
// Playbook appearance's GlobalStyles), so it must sit inside the nonce-enabled
// Emotion cache. This runs in Node (no DOM), where Emotion renders each
// <style> inline, so the nonce attribute can be checked directly.

jest.mock('./env', () => ({
  nonce: '',
  isCypress: false,
  pathPrefix: '',
  applicationName: 'GoAlert',
  GOALERT_VERSION: 'test',
}))

// ESM-only package (not transformed by this Jest setup); it only computes the
// Existing appearance's palette, which is irrelevant to nonce handling.
jest.mock('@material/material-color-utilities', () => {
  const c = 0xff607d8b
  const scheme = {
    primary: c,
    secondary: c,
    surfaceVariant: c,
    background: c,
    surface: c,
    error: c,
  }
  return {
    argbFromHex: () => c,
    hexFromArgb: () => '#607d8b',
    themeFromSourceColor: () => ({ schemes: { light: scheme, dark: scheme } }),
  }
})

const storage: Record<string, string> = {}
const g = globalThis as unknown as { window?: unknown }

beforeAll(() => {
  g.window = {
    localStorage: {
      getItem: (k: string) => (k in storage ? storage[k] : null),
      setItem: (k: string, v: string) => {
        storage[k] = String(v)
      },
    },
    matchMedia: () => ({
      matches: false,
      addEventListener: () => {},
      removeEventListener: () => {},
    }),
  }
})
afterAll(() => {
  delete g.window
})

function render(prefs: Record<string, string>): string[] {
  for (const k of Object.keys(storage)) delete storage[k]
  Object.assign(storage, prefs)
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const StyleProviders = require('./StyleProviders').default
  const cache = createCache({ key: 'mui', nonce: 'test-nonce' })
  const html = renderToString(
    React.createElement(
      StyleProviders,
      { cache },
      React.createElement(Box, { sx: { color: 'red' } }, 'content'),
    ),
  )
  return html.match(/<style[^>]*>/g) ?? []
}

describe('StyleProviders CSP nonce', () => {
  test.each([
    ['playbook', 'light'],
    ['playbook', 'dark'],
  ])(
    '%s %s: global and component styles carry the nonce',
    (appearance, theme) => {
      const tags = render({ appearance, theme })
      const global = tags.filter((t) => /data-emotion="[^"]*-global/.test(t))
      expect(global.length).toBeGreaterThan(0) // Playbook global styles rendered
      expect(tags.length).toBeGreaterThan(global.length) // plus component styles
      for (const tag of tags) expect(tag).toContain('nonce="test-nonce"')
    },
  )

  test('existing appearance: no global styles; component styles carry the nonce', () => {
    const tags = render({ appearance: 'existing', theme: 'light' })
    expect(tags.filter((t) => /-global/.test(t))).toHaveLength(0)
    expect(tags.length).toBeGreaterThan(0)
    for (const tag of tags) expect(tag).toContain('nonce="test-nonce"')
  })
})
