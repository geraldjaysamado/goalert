import React, { useEffect, useState } from 'react'
import { gql, useClient, useMutation, useQuery } from 'urql'
import {
  Alert,
  Button,
  FormControlLabel,
  Grid,
  MenuItem,
  Paper,
  Switch,
  TextField,
  Typography,
} from '@mui/material'
import FormDialog from '../../dialogs/FormDialog'
import { useErrorConsumer } from '../../util/ErrorConsumer'
import SlackMessagePreview from './SlackMessagePreview'
import {
  AlertStatus,
  Query,
  SlackAlertTemplatePreviewSample,
} from '../../../schema'

export interface SlackTemplateValue {
  name: string
  title: string
  text: string
}

const samplesQuery = gql`
  query slackAlertTemplatePreviewSamples {
    slackAlertTemplatePreviewSamples {
      id
      name
    }
  }
`

const previewQuery = gql`
  query previewSlackAlertTemplate($input: PreviewSlackAlertTemplateInput!) {
    previewSlackAlertTemplate(input: $input) {
      color
      title
      text
      truncated
      buttons {
        label
        url
      }
    }
  }
`

const createMutation = gql`
  mutation createSlackAlertTemplate($input: CreateSlackAlertTemplateInput!) {
    createSlackAlertTemplate(input: $input) {
      id
    }
  }
`

const latestQuery = gql`
  query slackAlertTemplateLatest {
    slackAlertTemplates {
      id
      name
      title
      text
      revision
    }
  }
`

const updateMutation = gql`
  mutation updateSlackAlertTemplate($input: UpdateSlackAlertTemplateInput!) {
    updateSlackAlertTemplate(input: $input)
  }
`

const ctx = { additionalTypenames: ['SlackAlertTemplate'] }
// --goalert-mono is set by the Playbook appearance; otherwise plain monospace.
const mono = {
  fontFamily: 'var(--goalert-mono, monospace)',
  fontSize: '0.85rem',
}

function useDebounced<T>(value: T, ms: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return debounced
}

export default function SlackTemplateEditDialog(props: {
  // templateID and revision are set when editing; omitted when creating.
  templateID?: string
  revision?: string
  initialValue: SlackTemplateValue
  dialogTitle: string
  onClose: () => void
}): React.JSX.Element {
  const [value, setValue] = useState<SlackTemplateValue>(props.initialValue)
  const [revision, setRevision] = useState(props.revision)
  const [reloadMsg, setReloadMsg] = useState('')
  const [conflictDismissed, setConflictDismissed] = useState(false)
  const client = useClient()
  const [sampleID, setSampleID] = useState('deployment')
  const [status, setStatus] = useState<AlertStatus>('StatusUnacknowledged')
  const [usePayload, setUsePayload] = useState(false)
  const [payload, setPayload] = useState('')

  const [createStatus, create] = useMutation(createMutation)
  const [updateStatus, update] = useMutation(updateMutation)
  const saving = createStatus.fetching || updateStatus.fetching

  const [{ data: samplesData }] = useQuery<
    Pick<Query, 'slackAlertTemplatePreviewSamples'>
  >({ query: samplesQuery })
  const samples: SlackAlertTemplatePreviewSample[] =
    samplesData?.slackAlertTemplatePreviewSamples ?? []

  const previewInput = useDebounced(
    {
      title: value.title,
      text: value.text,
      sampleID,
      status,
      alertmanagerPayload: usePayload ? payload : null,
    },
    400,
  )
  const [preview] = useQuery<Pick<Query, 'previewSlackAlertTemplate'>>({
    query: previewQuery,
    variables: { input: previewInput },
    pause:
      (!previewInput.title.trim() && !previewInput.text.trim()) ||
      (usePayload && !payload.trim()),
  })

  const errs = useErrorConsumer(createStatus.error).append(updateStatus.error)
  // Always consume the conflict error so it is not repeated as a generic error.
  const rawConflictErr = errs.getErrorByField('expectedRevision')
  const conflictErr = conflictDismissed ? undefined : rawConflictErr
  const goneErr = props.templateID ? errs.getErrorByField('id') : undefined
  const nameErr = errs.getErrorByField('name')
  const titleErr = errs.getErrorByField('title')
  const textErr = errs.getErrorByField('text')
  const previewErr = useErrorConsumer(preview.error)
  const previewFieldErr =
    previewErr.getErrorByField('title') ||
    previewErr.getErrorByField('text') ||
    previewErr.getErrorByField('alertmanagerPayload') ||
    previewErr.getErrorByField('sampleID')
  const previewOtherErr = previewErr.remaining().join('\n')

  // loadLatest replaces the form with the stored version after a conflict.
  async function loadLatest(): Promise<void> {
    const res = await client
      .query(latestQuery, {}, { requestPolicy: 'network-only' })
      .toPromise()
    const latest = res.data?.slackAlertTemplates?.find(
      (t: { id: string }) => t.id === props.templateID,
    )
    if (!latest) {
      setReloadMsg('This template no longer exists.')
      return
    }
    setValue({ name: latest.name, title: latest.title, text: latest.text })
    setRevision(latest.revision)
    setConflictDismissed(true)
    setReloadMsg(
      'Loaded the latest version. Re-apply your edits, then save again.',
    )
  }

  const form = (
    <Grid container spacing={2}>
      {(conflictErr || goneErr) && (
        <Grid item xs={12}>
          <Alert
            severity='warning'
            data-testid='slack-template-conflict'
            action={
              conflictErr && (
                <Button color='inherit' size='small' onClick={loadLatest}>
                  Load latest version
                </Button>
              )
            }
          >
            {conflictErr || goneErr}
            {conflictErr &&
              ' Loading replaces the text below, so copy anything you want to keep first.'}
          </Alert>
        </Grid>
      )}
      {reloadMsg && !conflictErr && (
        <Grid item xs={12}>
          <Alert severity='info' onClose={() => setReloadMsg('')}>
            {reloadMsg}
          </Alert>
        </Grid>
      )}
      <Grid item xs={12} md={6}>
        <TextField
          fullWidth
          label='Name'
          value={value.name}
          onChange={(e) => setValue({ ...value, name: e.target.value })}
          error={!!nameErr}
          helperText={
            nameErr || 'Shown in the service template dropdown. Must be unique.'
          }
          disabled={saving}
          required
        />
        <TextField
          fullWidth
          sx={{ mt: 2 }}
          label='Title template'
          value={value.title}
          onChange={(e) => setValue({ ...value, title: e.target.value })}
          error={!!titleErr}
          helperText={
            titleErr ||
            'Plain text (Slack links it to the GoAlert alert). Blank uses "Alert #ID: summary".'
          }
          multiline
          minRows={2}
          maxRows={6}
          InputProps={{ sx: mono }}
          disabled={saving}
        />
        <TextField
          fullWidth
          sx={{ mt: 2 }}
          label='Body template'
          value={value.text}
          onChange={(e) => setValue({ ...value, text: e.target.value })}
          error={!!textErr}
          helperText={
            textErr ||
            'Go template producing Slack mrkdwn. Data: .Alerts, .CommonLabels, .CommonAnnotations, .Meta, .Status, .GoAlertStatus, .ServiceName, .AlertURL.'
          }
          multiline
          minRows={12}
          maxRows={28}
          InputProps={{ sx: mono }}
          disabled={saving}
        />
      </Grid>
      <Grid item xs={12} md={6}>
        <Typography variant='subtitle1' gutterBottom>
          Preview (nothing is sent)
        </Typography>
        <Grid container spacing={1}>
          <Grid item xs={8}>
            <TextField
              select
              fullWidth
              size='small'
              label='Sample alert'
              value={samples.length ? sampleID : ''}
              onChange={(e) => setSampleID(e.target.value)}
              disabled={usePayload}
            >
              {samples.map((s) => (
                <MenuItem key={s.id} value={s.id}>
                  {s.name}
                </MenuItem>
              ))}
            </TextField>
          </Grid>
          <Grid item xs={4}>
            <TextField
              select
              fullWidth
              size='small'
              label='Status'
              value={status}
              onChange={(e) => setStatus(e.target.value as AlertStatus)}
            >
              <MenuItem value='StatusUnacknowledged'>Firing</MenuItem>
              <MenuItem value='StatusAcknowledged'>Acknowledged</MenuItem>
              <MenuItem value='StatusClosed'>Resolved</MenuItem>
            </TextField>
          </Grid>
          <Grid item xs={12}>
            <FormControlLabel
              control={
                <Switch
                  checked={usePayload}
                  onChange={(e) => setUsePayload(e.target.checked)}
                />
              }
              label='Use a real Alertmanager payload instead (optional)'
            />
            {usePayload && (
              <TextField
                fullWidth
                multiline
                minRows={4}
                maxRows={10}
                placeholder='Paste the Alertmanager webhook JSON body'
                value={payload}
                onChange={(e) => setPayload(e.target.value)}
                InputProps={{ sx: mono }}
              />
            )}
          </Grid>
        </Grid>
        <Paper variant='outlined' sx={{ p: 2, mt: 1 }}>
          {previewFieldErr || previewOtherErr ? (
            <Alert severity='error' sx={{ whiteSpace: 'pre-wrap' }}>
              {previewFieldErr || previewOtherErr}
            </Alert>
          ) : preview.data?.previewSlackAlertTemplate ? (
            <React.Fragment>
              {preview.data.previewSlackAlertTemplate.truncated && (
                <Alert severity='warning' sx={{ mb: 1 }}>
                  Even after listing fewer alerts, the body is longer than Slack
                  allows, so trailing lines were removed (shown below exactly as
                  sent). Shorten long fields or lines.
                </Alert>
              )}
              <SlackMessagePreview
                preview={preview.data.previewSlackAlertTemplate}
                status={status}
              />
            </React.Fragment>
          ) : (
            <Typography color='text.secondary'>
              {preview.fetching
                ? 'Rendering…'
                : 'Enter a title or body to see a preview.'}
            </Typography>
          )}
        </Paper>
      </Grid>
    </Grid>
  )

  return (
    <FormDialog
      title={props.dialogTitle}
      maxWidth='lg'
      loading={saving}
      errors={errs.remainingLegacyCallback()}
      onClose={props.onClose}
      primaryActionLabel='Save'
      onSubmit={async () => {
        setConflictDismissed(false)
        const res = props.templateID
          ? await update(
              {
                input: {
                  id: props.templateID,
                  expectedRevision: revision,
                  ...value,
                },
              },
              { additionalTypenames: ['SlackAlertTemplate'] },
            )
          : await create({ input: value }, ctx)
        if (res.error) return
        props.onClose()
      }}
      form={form}
    />
  )
}
