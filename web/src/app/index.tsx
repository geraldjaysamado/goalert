import React, { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Provider as ReduxProvider } from 'react-redux'
import { ApolloProvider } from '@apollo/client'
import { GOALERT_VERSION as version, nonce, pathPrefix } from './env'
import StyleProviders from './StyleProviders'
import { GraphQLClient } from './apollo'
import './styles'
// Playbook appearance fonts (SIL OFL 1.1), bundled locally. The browser only
// downloads a font file once text actually uses that face.
import '@fontsource/inter/latin-400.css'
import '@fontsource/inter/latin-500.css'
import '@fontsource/inter/latin-600.css'
import '@fontsource/inter/latin-700.css'
import '@fontsource/jetbrains-mono/latin-400.css'
import '@fontsource/jetbrains-mono/latin-500.css'
import App from './main/App'
import store from './reduxStore'
import { ConfigProvider } from './util/RequireConfig'
import { warn } from './util/debug'
import NewVersionCheck from './NewVersionCheck'
import { Provider as URQLProvider } from 'urql'
import { client as urqlClient } from './urql'
import { Router } from 'wouter'

import { Settings } from 'luxon'
import RequireAuth from './main/RequireAuth'
import Login from './main/components/Login'
import createCache from '@emotion/cache'

Settings.throwOnInvalid = true

declare module 'luxon' {
  interface TSSettings {
    throwOnInvalid: true
  }
}

// version check
if (
  document
    .querySelector('meta[http-equiv=x-goalert-version]')
    ?.getAttribute('content') !== version
) {
  warn(
    'app.js version does not match HTML version',
    'index.html=' +
      document
        .querySelector('meta[http-equiv=x-goalert-version]')
        ?.getAttribute('content'),
    'app.js=' + version,
  )
}

const rootElement = document.getElementById('app')
const root = createRoot(rootElement as HTMLElement)

const cache = createCache({
  key: 'mui',
  prepend: true,
  nonce,
  speedy: true,
})

root.render(
  <StrictMode>
    <StyleProviders cache={cache}>
      <ApolloProvider client={GraphQLClient}>
        <ReduxProvider store={store}>
          <Router base={pathPrefix}>
            <URQLProvider value={urqlClient}>
              <NewVersionCheck />
              <RequireAuth fallback={<Login />}>
                <ConfigProvider>
                  <App />
                </ConfigProvider>
              </RequireAuth>
            </URQLProvider>
          </Router>
        </ReduxProvider>
      </ApolloProvider>
    </StyleProviders>
  </StrictMode>,
)
