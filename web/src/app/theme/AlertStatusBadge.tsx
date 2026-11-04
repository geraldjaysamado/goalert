import React from 'react'
import { useTheme } from '@mui/material/styles'
import { AlertStatus } from '../../schema'
import { playbookTokens } from './playbook'

const label: Record<AlertStatus, string> = {
  StatusUnacknowledged: 'Unacknowledged',
  StatusAcknowledged: 'Acknowledged',
  StatusClosed: 'Closed',
}

// AlertStatusBadge is the Playbook appearance's status pill. The status is
// always spelled out, so it does not rely on color: Unacknowledged is red,
// Acknowledged amber and Closed (resolved) green.
export default function AlertStatusBadge(props: {
  status: AlertStatus
  'data-cy'?: string
}): React.JSX.Element {
  const theme = useTheme()
  const t = playbookTokens(theme.palette.mode)
  const colors: Record<AlertStatus, [string, string]> = {
    StatusUnacknowledged: [t.badBg, t.bad],
    StatusAcknowledged: [t.warnBg, t.warn],
    StatusClosed: [t.okBg, t.ok],
  }
  const [bg, fg] = colors[props.status] ?? [t.neutralBg, t.neutral]
  return (
    <span
      data-cy={props['data-cy']}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        borderRadius: 999,
        padding: '2px 9px',
        fontSize: 12,
        fontWeight: 600,
        lineHeight: 1.5,
        whiteSpace: 'nowrap',
        background: bg,
        color: fg,
        verticalAlign: 'middle',
      }}
    >
      <span
        aria-hidden
        style={{ width: 6, height: 6, borderRadius: '50%', background: fg }}
      />
      {label[props.status] ?? props.status}
    </span>
  )
}
