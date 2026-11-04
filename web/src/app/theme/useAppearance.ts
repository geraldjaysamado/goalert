import { useContext } from 'react'
import { ThemeContext } from './themeConfig'
import { Appearance } from './playbook'

// useAppearance returns the selected console appearance (Existing/Playbook).
export function useAppearance(): Appearance {
  return useContext(ThemeContext).appearance
}
