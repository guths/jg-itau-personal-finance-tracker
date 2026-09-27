import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react"

import { PRIVACY_STORAGE_KEY, PrivacyContext } from "@/lib/privacy"

function readStored(): boolean {
  try {
    return localStorage.getItem(PRIVACY_STORAGE_KEY) === "1"
  } catch {
    return false
  }
}

function store(hidden: boolean) {
  try {
    if (hidden) localStorage.setItem(PRIVACY_STORAGE_KEY, "1")
    else localStorage.removeItem(PRIVACY_STORAGE_KEY)
  } catch {
    // armazenamento indisponível: vale só nesta aba
  }
}

/** Modo privacidade (esconder valores), lembrado no navegador; atalho Shift+H. */
export function PrivacyProvider({ children }: { children: ReactNode }) {
  const [hidden, setHiddenState] = useState(readStored)

  const setHidden = useCallback((v: boolean) => {
    setHiddenState(v)
    store(v)
  }, [])
  const toggle = useCallback(() => setHidden(!hidden), [hidden, setHidden])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.defaultPrevented || !e.shiftKey || e.metaKey || e.ctrlKey || e.altKey || e.repeat) return
      if (e.key !== "H" && e.key !== "h") return
      const t = e.target instanceof HTMLElement ? e.target : null
      if (t?.closest('input, textarea, select, [contenteditable="true"]')) return
      e.preventDefault()
      toggle()
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [toggle])

  const value = useMemo(() => ({ hidden, setHidden, toggle }), [hidden, setHidden, toggle])
  return <PrivacyContext.Provider value={value}>{children}</PrivacyContext.Provider>
}
