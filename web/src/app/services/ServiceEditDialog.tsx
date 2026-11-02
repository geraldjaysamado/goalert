import React, { useState } from 'react'
import { gql, useQuery, useMutation } from 'urql'

import FormDialog from '../dialogs/FormDialog'
import ServiceForm from './ServiceForm'
import { Label } from '../../schema'
import { useErrorConsumer } from '../util/ErrorConsumer'
import { useConfigValue } from '../util/RequireConfig'
import {
  ServiceSlackTemplateField,
  useServiceSlackTemplate,
} from './ServiceSlackTemplate'

interface Value {
  name: string
  description: string
  escalationPolicyID?: string
  labels: Label[]
}

const query = gql`
  query service($id: ID!) {
    service(id: $id) {
      id
      name
      description
      labels {
        key
        value
      }
      ep: escalationPolicy {
        id
        name
      }
    }
  }
`
const mutation = gql`
  mutation updateService($input: UpdateServiceInput!) {
    updateService(input: $input)
  }
`
const setSlackTemplate = gql`
  mutation setServiceSlackAlertTemplate(
    $input: SetServiceSlackAlertTemplateInput!
  ) {
    setServiceSlackAlertTemplate(input: $input)
  }
`
const setLabel = gql`
  mutation setLabel($input: SetLabelInput!) {
    setLabel(input: $input)
  }
`

export default function ServiceEditDialog(props: {
  serviceID: string
  onClose: () => void
}): JSX.Element {
  const [{ data, error: dataError }] = useQuery({
    query,
    variables: { id: props.serviceID },
  })
  const [req] = useConfigValue('Services.RequiredLabels') as [string[]]
  const defaultValue = {
    name: data?.service?.name,
    description: data?.service?.description,
    escalationPolicyID: data?.service?.ep?.id,
    labels: (data?.service?.labels || []).filter((l: Label) =>
      req.includes(l.key),
    ),
  }
  const [value, setValue] = useState<Value>(defaultValue)

  const [saveStatus, save] = useMutation(mutation)
  const [saveLabelStatus, saveLabel] = useMutation(setLabel)

  // Slack template selection (admins only). undefined = not changed.
  const slackTemplate = useServiceSlackTemplate(props.serviceID)
  const [slackTemplateID, setSlackTemplateID] = useState<string | undefined>()
  const [saveSlackStatus, saveSlack] = useMutation(setSlackTemplate)

  const errs = useErrorConsumer(saveStatus.error)
    .append(saveLabelStatus.error)
    .append(saveSlackStatus.error)
  const slackTemplateErr =
    errs.getErrorByField('templateID') ||
    errs.getErrorByField(/^Slack\./) ||
    undefined

  return (
    <FormDialog
      title='Edit Service'
      loading={saveStatus.fetching || saveSlackStatus.fetching}
      errors={errs.remainingLegacyCallback()}
      onClose={props.onClose}
      onSubmit={async () => {
        const saveRes = await save(
          {
            input: {
              id: props.serviceID,
              name: value?.name || '',
              description: value?.description || '',
              escalationPolicyID: value?.escalationPolicyID || '',
            },
          },
          {
            additionalTypenames: ['Service'],
          },
        )
        if (saveRes.error) return

        for (const label of value?.labels || []) {
          const res = await saveLabel({
            input: {
              target: {
                type: 'service',
                id: props.serviceID,
              },
              key: label.key,
              value: label.value,
            },
          })
          if (res.error) return
        }

        if (
          slackTemplate.isAdmin &&
          slackTemplateID !== undefined &&
          slackTemplateID !== slackTemplate.currentID
        ) {
          const res = await saveSlack(
            {
              input: {
                serviceID: props.serviceID,
                templateID: slackTemplateID || null,
              },
            },
            { additionalTypenames: ['SlackAlertTemplate'] },
          )
          if (res.error) return
        }

        props.onClose()
      }}
      form={
        <React.Fragment>
          <ServiceForm
            epRequired
            nameError={errs.getErrorByField('Name')}
            descError={errs.getErrorByField('Description')}
            epError={errs.getErrorByField('EscalationPolicyID')}
            labelErrorKey={saveLabelStatus.operation?.variables?.input?.key}
            labelErrorMsg={errs.getErrorByField('Value')}
            disabled={Boolean(saveStatus.fetching || !data || dataError)}
            value={value}
            onChange={(value) => setValue(value)}
          />
          {slackTemplate.isAdmin && (
            <div style={{ marginTop: 16 }}>
              <ServiceSlackTemplateField
                state={slackTemplate}
                value={slackTemplateID ?? slackTemplate.currentID}
                onChange={setSlackTemplateID}
                disabled={Boolean(saveStatus.fetching || !data || dataError)}
                error={slackTemplateErr}
              />
            </div>
          )}
        </React.Fragment>
      }
    />
  )
}
