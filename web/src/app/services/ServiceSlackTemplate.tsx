import React from 'react'
import { gql, useQuery } from 'urql'
import { MenuItem, TextField } from '@mui/material'
import { useSessionInfo } from '../util/RequireConfig'
import AppLink from '../util/AppLink'
import { Query, SlackAlertTemplate } from '../../schema'

const query = gql`
  query serviceSlackAlertTemplates {
    slackAlertTemplates {
      id
      name
      services {
        id
      }
    }
  }
`

const context = { additionalTypenames: ['SlackAlertTemplate'] }

export interface ServiceSlackTemplateState {
  // isAdmin is false when the current user cannot see or change templates.
  isAdmin: boolean
  ready: boolean
  templates: SlackAlertTemplate[]
  // currentID is the assigned template ID, or '' for existing/default formatting.
  currentID: string
}

// useServiceSlackTemplate loads the template library for admins only, so
// non-admin users never request admin-only data.
export function useServiceSlackTemplate(
  serviceID: string,
): ServiceSlackTemplateState {
  const { isAdmin, ready } = useSessionInfo()
  const [{ data }] = useQuery<Pick<Query, 'slackAlertTemplates'>>({
    query,
    context,
    pause: !ready || !isAdmin,
  })
  const templates = data?.slackAlertTemplates ?? []
  const current = templates.find((t) =>
    t.services.some((s) => s.id.toLowerCase() === serviceID.toLowerCase()),
  )
  return {
    isAdmin: ready && isAdmin,
    ready: !!data,
    templates,
    currentID: current?.id ?? '',
  }
}

export function ServiceSlackTemplateField(props: {
  state: ServiceSlackTemplateState
  value: string
  onChange: (templateID: string) => void
  disabled?: boolean
  error?: string
}): React.JSX.Element | null {
  const { state } = props
  if (!state.isAdmin) return null

  return (
    <TextField
      select
      fullWidth
      label='Slack template'
      value={state.ready ? props.value : ''}
      onChange={(e) => props.onChange(e.target.value)}
      disabled={props.disabled || !state.ready}
      error={!!props.error}
      helperText={
        props.error || (
          <React.Fragment>
            Changes only this service&apos;s Slack alert messages. Manage
            templates in{' '}
            <AppLink to='/admin/slack-templates'>
              Admin → Slack Templates
            </AppLink>
            .
          </React.Fragment>
        )
      }
      SelectProps={{ displayEmpty: true }}
      InputLabelProps={{ shrink: true }}
    >
      <MenuItem value=''>Use existing/default formatting</MenuItem>
      {state.templates.map((t) => (
        <MenuItem key={t.id} value={t.id}>
          {t.name}
        </MenuItem>
      ))}
    </TextField>
  )
}

// ServiceSlackTemplateInfo shows the assigned template on the service page
// (admins only; nothing is shown when the default formatting is used).
export function ServiceSlackTemplateInfo(props: {
  serviceID: string
}): React.JSX.Element | null {
  const state = useServiceSlackTemplate(props.serviceID)
  if (!state.isAdmin || !state.currentID) return null
  const t = state.templates.find((t) => t.id === state.currentID)
  return (
    <div>
      Slack template: <AppLink to='/admin/slack-templates'>{t?.name}</AppLink>
    </div>
  )
}
