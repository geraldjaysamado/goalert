import React, { ReactNode } from 'react'
import Drawer from '@mui/material/Drawer'
import makeStyles from '@mui/styles/makeStyles'
import { SxProps, Theme } from '@mui/material/styles'

export const drawerWidth = '190px'

const useStyles = makeStyles((theme: Theme) => ({
  sidebarPaper: {
    width: drawerWidth,
    position: 'fixed',
    transition: theme.transitions.create('width', {
      easing: theme.transitions.easing.sharp,
      duration: theme.transitions.duration.enteringScreen,
    }),
  },
}))

interface WideSideBarProps {
  children: ReactNode
  // Optional overrides (used by the Playbook appearance); the existing
  // appearance passes neither and renders exactly as before.
  width?: string
  paperSx?: SxProps<Theme>
}

function WideSideBar(props: WideSideBarProps): React.JSX.Element {
  const classes = useStyles()

  return (
    <Drawer
      variant='permanent'
      classes={{
        paper: classes.sidebarPaper,
      }}
      PaperProps={
        props.width || props.paperSx
          ? {
              // inline width: the makeStyles class above would win over sx
              style: props.width ? { width: props.width } : undefined,
              sx: props.paperSx,
            }
          : undefined
      }
    >
      {props.children}
    </Drawer>
  )
}

export default WideSideBar
