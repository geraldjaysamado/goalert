import React, { ReactNode } from 'react'
import { StyledEngineProvider } from '@mui/material/styles'
import { CacheProvider, EmotionCache } from '@emotion/react'
import { ThemeProvider } from './theme/themeConfig'

// StyleProviders sets up styling for the app. The nonce-enabled Emotion cache
// must wrap ThemeProvider: ThemeProvider renders global styles (e.g. the
// Playbook appearance's GlobalStyles), and with Content-Security-Policy
// enabled every injected <style> needs the CSP nonce.
export default function StyleProviders(props: {
  cache: EmotionCache
  children: ReactNode
}): React.JSX.Element {
  return (
    <StyledEngineProvider injectFirst>
      <CacheProvider value={props.cache}>
        <ThemeProvider>{props.children}</ThemeProvider>
      </CacheProvider>
    </StyledEngineProvider>
  )
}
