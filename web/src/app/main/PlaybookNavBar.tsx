import React from 'react'
import { useLocation } from 'wouter'
import Box from '@mui/material/Box'
import { useTheme } from '@mui/material/styles'
import {
  Group,
  Layers,
  Notifications,
  RotateRight,
  Today,
  VpnKey,
  Build,
  DeveloperBoard,
  OpenInNew,
} from '@mui/icons-material'
import { WizardHat as WizardIcon } from 'mdi-material-ui'
import AppLink from '../util/AppLink'
import RequireConfig from '../util/RequireConfig'
import { applicationName } from '../env'
import { playbookTokens } from '../theme/playbook'

// Playbook appearance navigation: the same routes and admin restrictions as
// NavBar, grouped under labels and styled like the OpsPlaybook console.

type ItemProps = {
  to: string
  title: string
  icon?: React.JSX.Element
  newTab?: boolean
  exact?: boolean
}

function NavItem(props: ItemProps): React.JSX.Element {
  const theme = useTheme()
  const t = playbookTokens(theme.palette.mode)
  const [path] = useLocation()
  const active = props.exact
    ? path === props.to
    : path === props.to || path.startsWith(props.to + '/')

  return (
    <AppLink
      to={props.to}
      newTab={props.newTab}
      underline='none'
      aria-current={active ? 'page' : undefined}
      data-cy={'nav-' + props.title.toLowerCase().replace(/\s+/g, '-')}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: '10px',
        padding: '8px',
        borderRadius: '3px',
        fontSize: '0.84375rem', // 13.5px
        fontWeight: 500,
        lineHeight: 1.3,
        color: active ? t.navInk : t.navMuted,
        backgroundColor: active ? t.nav2 : 'transparent',
        boxShadow: active ? `inset 3px 0 0 ${t.accent}` : 'none',
        '&:hover': {
          color: t.navInk,
          backgroundColor: active ? t.nav2 : 'rgba(255,255,255,0.06)',
          textDecoration: 'none',
        },
        '&:focus-visible': { outline: `2px solid ${t.soft}`, outlineOffset: 2 },
        '& svg': { fontSize: 18, flex: 'none' },
      }}
    >
      {props.icon ?? <Box component='span' sx={{ width: 18, flex: 'none' }} />}
      <Box component='span' sx={{ minWidth: 0, overflowWrap: 'anywhere' }}>
        {props.title}
      </Box>
      {props.newTab && (
        <OpenInNew sx={{ ml: 'auto', fontSize: '14px !important' }} />
      )}
    </AppLink>
  )
}

function NavGroup(props: {
  label: string
  children: React.ReactNode
}): React.JSX.Element {
  const theme = useTheme()
  const t = playbookTokens(theme.palette.mode)
  return (
    <Box
      component='section'
      aria-label={props.label}
      sx={{ display: 'flex', flexDirection: 'column', gap: '2px' }}
    >
      <Box
        sx={{
          color: t.navLabel,
          fontSize: '0.65625rem', // 10.5px
          fontWeight: 600,
          letterSpacing: '0.09em',
          textTransform: 'uppercase',
          padding: '0 8px 6px',
        }}
      >
        {props.label}
      </Box>
      {props.children}
    </Box>
  )
}

export default function PlaybookNavBar(): React.JSX.Element {
  const theme = useTheme()
  const t = playbookTokens(theme.palette.mode)
  const dev = process.env.NODE_ENV !== 'production'

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        gap: '22px',
        padding: '20px 14px 16px 16px',
        minHeight: '100%',
        color: t.navInk,
        fontFamily: theme.typography.fontFamily,
      }}
    >
      <AppLink
        to='/'
        underline='none'
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          padding: '2px 6px',
          color: t.navInk,
          fontWeight: 700,
          fontSize: '1.0625rem', // 17px
          letterSpacing: '-0.01em',
          '&:hover': { textDecoration: 'none', color: t.navInk },
        }}
      >
        <Box
          aria-hidden
          sx={{
            width: 22,
            height: 22,
            borderRadius: '3px',
            backgroundColor: t.accent,
            display: 'grid',
            placeItems: 'center',
            color: '#fff',
            '& svg': { fontSize: 15 },
          }}
        >
          <Notifications />
        </Box>
        {applicationName}
      </AppLink>

      <nav aria-label='Main' data-cy='nav-list'>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: '22px' }}>
          <NavGroup label='Alerting'>
            <NavItem to='/alerts' title='Alerts' icon={<Notifications />} />
            <NavItem to='/services' title='Services' icon={<VpnKey />} />
            <NavItem
              to='/escalation-policies'
              title='Escalation Policies'
              icon={<Layers />}
            />
          </NavGroup>
          <NavGroup label='On-call'>
            <NavItem to='/schedules' title='Schedules' icon={<Today />} />
            <NavItem to='/rotations' title='Rotations' icon={<RotateRight />} />
            <NavItem to='/users' title='Users' icon={<Group />} />
          </NavGroup>
          <RequireConfig isAdmin>
            <NavGroup label='Administration'>
              <NavItem to='/admin/config' title='Config' icon={<Build />} />
              <NavItem to='/admin/maintenance' title='Maintenance' />
              <NavItem to='/admin/limits' title='System Limits' />
              <NavItem to='/admin/toolbox' title='Toolbox' />
              <NavItem to='/admin/message-logs' title='Message Logs' />
              <NavItem to='/admin/alert-counts' title='Alert Counts' />
              <NavItem to='/admin/service-metrics' title='Service Metrics' />
              <NavItem to='/admin/switchover' title='Switchover' />
              <NavItem to='/admin/api-keys' title='API Keys' />
              <NavItem to='/admin/slack-templates' title='Slack Templates' />
              {dev && <NavItem newTab to='/admin/riverui' title='Job Queues' />}
            </NavGroup>
          </RequireConfig>
          <NavGroup label='Tools'>
            <NavItem to='/wizard' title='Wizard' icon={<WizardIcon />} />
            {dev && <NavItem to='/dev' title='Dev' icon={<DeveloperBoard />} />}
          </NavGroup>
        </Box>
      </nav>
    </Box>
  )
}
