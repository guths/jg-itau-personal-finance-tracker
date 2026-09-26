import { useEffect, useMemo, useState, useSyncExternalStore, type ReactNode } from "react"

import { THEME_STORAGE_KEY, ThemeContext, type Theme } from "@/lib/theme"

const DARK_QUERY = "(prefers-color-scheme: dark)"

function subscribe(onChange: () => void) {
  const mql = window.matchMedia(DARK_QUERY)
  mql.addEventListener("change", onChange)
  return () => mql.removeEventListener("change", onChange)
}

function readStored(): Theme {
  try {
    const v = localStorage.getItem(THEME_STORAGE_KEY)
    return v === "light" || v === "dark" ? v : "system"
  } catch {
    return "system"
  }
}

/** Tema claro/escuro/sistema com a classe `.dark` do shadcn (o index.html aplica antes do React). */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(readStored)
  const systemDark = useSyncExternalStore(
    subscribe,
    () => window.matchMedia(DARK_QUERY).matches,
    () => false,
  )
  const resolvedTheme: "light" | "dark" = theme === "system" ? (systemDark ? "dark" : "light") : theme

  useEffect(() => {
    const root = document.documentElement
    root.classList.toggle("dark", resolvedTheme === "dark")
    root.style.colorScheme = resolvedTheme
  }, [resolvedTheme])

  const value = useMemo(
    () => ({
      theme,
      resolvedTheme,
      setTheme: (t: Theme) => {
        setThemeState(t)
        try {
          localStorage.setItem(THEME_STORAGE_KEY, t)
        } catch {
          // armazenamento indisponível: vale só nesta aba
        }
      },
    }),
    [theme, resolvedTheme],
  )

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}
