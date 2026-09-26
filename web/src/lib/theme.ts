import { createContext, useContext } from "react"

export type Theme = "light" | "dark" | "system"

export interface ThemeState {
  theme: Theme
  resolvedTheme: "light" | "dark"
  setTheme: (t: Theme) => void
}

export const THEME_STORAGE_KEY = "theme"

export const ThemeContext = createContext<ThemeState | null>(null)

export function useTheme(): ThemeState {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error("useTheme precisa estar dentro de <ThemeProvider>")
  return ctx
}
