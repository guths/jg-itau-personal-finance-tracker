import { useEffect } from "react"

interface Handlers {
  onPick: (index: number) => void
  onConfirm: () => void
  onSkip: () => void
}

/**
 * Atalhos do chat: 1–3 escolhe a sugestão, Enter confirma, S pula. Ignorados enquanto se digita
 * ou com um popover/menu aberto; Enter num botão comum mantém o comportamento nativo.
 */
export function useAnswerShortcuts({ onPick, onConfirm, onSkip }: Handlers) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey || e.repeat) return
      const t = e.target instanceof HTMLElement ? e.target : null
      if (t?.closest('[role="dialog"], [role="menu"], [role="listbox"], [cmdk-root]')) return
      const typing = !!t?.closest('input, textarea, select, [contenteditable="true"]')
      if (e.key === "Enter") {
        if (typing) return
        if (t?.closest("button, a") && !t.closest("[data-answer-option]")) return
        e.preventDefault()
        onConfirm()
        return
      }
      if (typing) return
      if (e.key === "1" || e.key === "2" || e.key === "3") {
        e.preventDefault()
        onPick(Number(e.key) - 1)
      } else if (e.key === "s" || e.key === "S") {
        e.preventDefault()
        onSkip()
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [onPick, onConfirm, onSkip])
}
