import React, { useState } from 'react'
import { gql, useQuery } from 'urql'
import {
  Button,
  Card,
  CardActions,
  CardContent,
  Chip,
  Grid,
  Menu,
  MenuItem,
  Stack,
  Typography,
} from '@mui/material'
import { Add, ContentCopy, Delete, Edit } from '@mui/icons-material'
import Spinner from '../loading/components/Spinner'
import { GenericError } from '../error-pages'
import AppLink from '../util/AppLink'
import SlackTemplateEditDialog, {
  SlackTemplateValue,
} from './slack-templates/SlackTemplateEditDialog'
import SlackTemplateDeleteDialog from './slack-templates/SlackTemplateDeleteDialog'
import { Query, SlackAlertTemplate } from '../../schema'

const query = gql`
  query slackAlertTemplateLibrary {
    slackAlertTemplates {
      id
      name
      title
      text
      revision
      services {
        id
        name
      }
    }
    slackAlertTemplateExamples {
      key
      name
      description
      title
      text
    }
  }
`

const context = { additionalTypenames: ['SlackAlertTemplate'] }

type EditState = {
  templateID?: string
  revision?: string
  dialogTitle: string
  value: SlackTemplateValue
}

// uniqueName returns base, or base with a number appended, so that it does not
// clash (case-insensitively) with an existing template name.
function uniqueName(base: string, templates: SlackAlertTemplate[]): string {
  const taken = new Set(templates.map((t) => t.name.toLowerCase()))
  if (!taken.has(base.toLowerCase())) return base
  for (let i = 2; ; i++) {
    const name = `${base} (${i})`
    if (!taken.has(name.toLowerCase())) return name
  }
}

export default function AdminSlackTemplates(): React.JSX.Element {
  const [{ data, error }] = useQuery<
    Pick<Query, 'slackAlertTemplates' | 'slackAlertTemplateExamples'>
  >({ query, context })
  const [edit, setEdit] = useState<EditState | null>(null)
  const [deleteTemplate, setDeleteTemplate] =
    useState<SlackAlertTemplate | null>(null)
  const [exampleMenu, setExampleMenu] = useState<HTMLElement | null>(null)

  if (error) return <GenericError error={error.message} />
  if (!data) return <Spinner />

  const templates = data.slackAlertTemplates
  const examples = data.slackAlertTemplateExamples

  return (
    <Grid container spacing={2}>
      <Grid item xs={12}>
        <Card>
          <CardContent>
            <Typography variant='h6' component='h2' gutterBottom>
              Slack templates
            </Typography>
            <Typography color='text.secondary'>
              Named layouts for Slack alert messages. A template only affects a
              service after it is selected in that service&apos;s Edit dialog;
              every other service keeps the existing/default formatting. If an
              assigned template fails while sending, the alert is still sent
              using the default formatting and the error is logged.
            </Typography>
          </CardContent>
          <CardActions>
            <Button
              variant='contained'
              startIcon={<Add />}
              onClick={() =>
                setEdit({
                  dialogTitle: 'New Slack template',
                  value: {
                    name: uniqueName('New template', templates),
                    title: '{{ .Summary }}',
                    text: '',
                  },
                })
              }
            >
              New template
            </Button>
            {examples.length > 0 && (
              <Button onClick={(e) => setExampleMenu(e.currentTarget)}>
                Start from an example
              </Button>
            )}
            <Menu
              anchorEl={exampleMenu}
              open={!!exampleMenu}
              onClose={() => setExampleMenu(null)}
            >
              {examples.map((ex) => (
                <MenuItem
                  key={ex.key}
                  onClick={() => {
                    setExampleMenu(null)
                    setEdit({
                      dialogTitle: `New template from "${ex.name}"`,
                      value: {
                        name: uniqueName(ex.name, templates),
                        title: ex.title,
                        text: ex.text,
                      },
                    })
                  }}
                  sx={{ display: 'block', maxWidth: 480, whiteSpace: 'normal' }}
                >
                  <Typography>{ex.name}</Typography>
                  <Typography variant='caption' color='text.secondary'>
                    {ex.description}
                  </Typography>
                </MenuItem>
              ))}
            </Menu>
          </CardActions>
        </Card>
      </Grid>

      {templates.length === 0 && (
        <Grid item xs={12}>
          <Typography color='text.secondary' sx={{ p: 2 }}>
            No templates yet. Create one, or start from an example.
          </Typography>
        </Grid>
      )}

      {templates.map((t) => (
        <Grid item xs={12} key={t.id}>
          <Card>
            <CardContent>
              <Typography variant='subtitle1' component='h3'>
                {t.name}
              </Typography>
              <Stack
                direction='row'
                spacing={1}
                sx={{ mt: 1, flexWrap: 'wrap', alignItems: 'center' }}
                useFlexGap
              >
                {t.services.length === 0 ? (
                  <Typography variant='body2' color='text.secondary'>
                    Not used by any service
                  </Typography>
                ) : (
                  <React.Fragment>
                    <Typography variant='body2' color='text.secondary'>
                      Used by:
                    </Typography>
                    {t.services.map((s) => (
                      <Chip
                        key={s.id}
                        size='small'
                        label={s.name}
                        component={AppLink}
                        to={`/services/${s.id}`}
                        clickable
                      />
                    ))}
                  </React.Fragment>
                )}
              </Stack>
            </CardContent>
            <CardActions>
              <Button
                startIcon={<Edit />}
                onClick={() =>
                  setEdit({
                    templateID: t.id,
                    revision: t.revision,
                    dialogTitle: `Edit "${t.name}"`,
                    value: { name: t.name, title: t.title, text: t.text },
                  })
                }
              >
                Edit
              </Button>
              <Button
                startIcon={<ContentCopy />}
                onClick={() =>
                  setEdit({
                    dialogTitle: `Duplicate "${t.name}"`,
                    value: {
                      name: uniqueName(`Copy of ${t.name}`, templates),
                      title: t.title,
                      text: t.text,
                    },
                  })
                }
              >
                Duplicate
              </Button>
              <Button
                startIcon={<Delete />}
                color='error'
                onClick={() => setDeleteTemplate(t)}
              >
                Delete
              </Button>
            </CardActions>
          </Card>
        </Grid>
      ))}

      {edit && (
        <SlackTemplateEditDialog
          templateID={edit.templateID}
          revision={edit.revision}
          dialogTitle={edit.dialogTitle}
          initialValue={edit.value}
          onClose={() => setEdit(null)}
        />
      )}
      {deleteTemplate && (
        <SlackTemplateDeleteDialog
          template={deleteTemplate}
          onClose={() => setDeleteTemplate(null)}
        />
      )}
    </Grid>
  )
}
