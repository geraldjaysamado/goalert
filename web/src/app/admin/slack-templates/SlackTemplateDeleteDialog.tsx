import React from 'react'
import { gql, useMutation } from 'urql'
import { Typography } from '@mui/material'
import FormDialog from '../../dialogs/FormDialog'
import { useErrorConsumer } from '../../util/ErrorConsumer'
import AppLink from '../../util/AppLink'
import { SlackAlertTemplate } from '../../../schema'

const mutation = gql`
  mutation deleteSlackAlertTemplate($id: ID!) {
    deleteSlackAlertTemplate(id: $id)
  }
`

export default function SlackTemplateDeleteDialog(props: {
  template: SlackAlertTemplate
  onClose: () => void
}): React.JSX.Element {
  const { template } = props
  const [status, commit] = useMutation(mutation)
  const errs = useErrorConsumer(status.error)
  const inUse = template.services.length > 0

  if (inUse) {
    return (
      <FormDialog
        alert
        title={`"${template.name}" is in use`}
        onClose={props.onClose}
        form={
          <React.Fragment>
            <Typography gutterBottom>
              This template can&apos;t be deleted while services use it. Edit
              each service below and choose another template (or the default
              formatting), then delete it.
            </Typography>
            <ul>
              {template.services.map((s) => (
                <li key={s.id}>
                  <AppLink to={`/services/${s.id}`}>{s.name}</AppLink>
                </li>
              ))}
            </ul>
          </React.Fragment>
        }
      />
    )
  }

  return (
    <FormDialog
      confirm
      title={`Delete "${template.name}"?`}
      subTitle='No services use this template. This cannot be undone.'
      loading={status.fetching}
      errors={errs.remainingLegacyCallback()}
      onClose={props.onClose}
      onSubmit={async () => {
        const res = await commit(
          { id: template.id },
          { additionalTypenames: ['SlackAlertTemplate'] },
        )
        if (!res.error) props.onClose()
      }}
    />
  )
}
