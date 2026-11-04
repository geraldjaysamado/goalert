import React from 'react'
import { Box, Button, Divider, Link, Typography } from '@mui/material'
import { AlertStatus, SlackAlertTemplatePreview } from '../../../schema'

// Minimal Slack mrkdwn renderer for previews: *bold*, <url|label>, <url>,
// line breaks and the &lt; &gt; &amp; escapes. Everything is rendered as
// React text nodes (no raw HTML), and only http(s) links become anchors.
const tokenRx = /(<[^<>|\s]+\|[^<>]+>|<[^<>|\s]+>|\*[^*\n]+\*)/g

function decode(text: string): string {
  return text.replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&')
}

function renderLink(url: string, label: string, key: number): React.ReactNode {
  if (!/^https?:\/\//i.test(url)) return decode(label)
  return (
    <Link key={key} href={url} target='_blank' rel='noopener noreferrer'>
      {decode(label)}
    </Link>
  )
}

function renderLine(line: string): React.ReactNode[] {
  return line.split(tokenRx).map((part, i) => {
    if (part.startsWith('<') && part.endsWith('>')) {
      const [url, label] = part.slice(1, -1).split('|', 2)
      return renderLink(url, label ?? url, i)
    }
    if (part.length > 2 && part.startsWith('*') && part.endsWith('*')) {
      return <strong key={i}>{decode(part.slice(1, -1))}</strong>
    }
    return decode(part)
  })
}

export function SlackMrkdwn(props: { text: string }): React.JSX.Element {
  return (
    <React.Fragment>
      {props.text.split('\n').map((line, i) => (
        <div key={i} style={{ minHeight: '1.4em' }}>
          {renderLine(line)}
        </div>
      ))}
    </React.Fragment>
  )
}

const statusColor: Record<AlertStatus, string> = {
  StatusUnacknowledged: '#862421',
  StatusAcknowledged: '#867321',
  StatusClosed: '#218626',
}

const statusLabel: Record<AlertStatus, string> = {
  StatusUnacknowledged: 'Unacknowledged',
  StatusAcknowledged: 'Acknowledged by Preview User',
  StatusClosed: 'Closed by Alertmanager',
}

export default function SlackMessagePreview(props: {
  preview: SlackAlertTemplatePreview
  status: AlertStatus
}): React.JSX.Element {
  const { preview, status } = props
  return (
    <Box
      sx={{
        borderLeft: `4px solid ${preview.color || statusColor[status]}`,
        pl: 1.5,
        py: 0.5,
        typography: 'body2',
        lineHeight: 1.4,
        wordBreak: 'break-word',
      }}
      data-testid='slack-preview'
    >
      <Typography
        component='div'
        sx={{ fontWeight: 'bold', color: 'primary.main', mb: 0.5 }}
      >
        {preview.title}
      </Typography>
      {preview.text && <SlackMrkdwn text={preview.text} />}
      {preview.buttons.length > 0 && (
        <Box sx={{ display: 'flex', gap: 1, mt: 1, flexWrap: 'wrap' }}>
          {preview.buttons.map((b) => (
            <Button
              key={b.url + b.label}
              size='small'
              variant='outlined'
              href={b.url}
              target='_blank'
              rel='noopener noreferrer'
            >
              {b.label}
            </Button>
          ))}
        </Box>
      )}
      <Typography variant='caption' component='div' sx={{ mt: 1 }}>
        {statusLabel[status]}
      </Typography>
      {status !== 'StatusClosed' && (
        <React.Fragment>
          <Divider sx={{ my: 1 }} />
          <Box sx={{ display: 'flex', gap: 1 }}>
            {status === 'StatusUnacknowledged' && (
              <Button size='small' variant='outlined' disabled>
                Acknowledge
              </Button>
            )}
            <Button size='small' variant='outlined' disabled>
              Close
            </Button>
          </Box>
          <Typography variant='caption' color='text.secondary'>
            Acknowledge/Close buttons appear when Slack interactive messages are
            enabled; templates do not change them.
          </Typography>
        </React.Fragment>
      )}
    </Box>
  )
}
