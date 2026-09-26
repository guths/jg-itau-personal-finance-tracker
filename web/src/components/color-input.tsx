import { useEffect, useRef, useState } from "react"

import { cn } from "@/lib/utils"

interface Props {
  value: string
  onCommit: (hex: string) => void
  label: string
  className?: string
}

const valid = (v: string) => (/^#[0-9a-f]{6}$/i.test(v) ? v : "#8C8C8C")

/**
 * Bolinha de cor com o seletor nativo. Mostra a prévia enquanto arrasta e só grava quando o
 * seletor fecha (evento nativo "change"), para não disparar um PATCH por movimento.
 */
export function ColorInput({ value, onCommit, label, className }: Props) {
  const ref = useRef<HTMLInputElement>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const commit = useRef(onCommit)
  useEffect(() => {
    commit.current = onCommit
  }, [onCommit])

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const onChange = () => {
      setPreview(null)
      const hex = el.value.toUpperCase()
      if (hex !== value.toUpperCase()) commit.current(hex)
    }
    el.addEventListener("change", onChange)
    return () => el.removeEventListener("change", onChange)
  }, [value])

  const shown = preview ?? valid(value)
  return (
    <label
      className={cn("relative block size-5 shrink-0 cursor-pointer rounded-full ring-1 ring-foreground/15", className)}
      style={{ backgroundColor: shown }}
    >
      <span className="sr-only">{label}</span>
      <input
        ref={ref}
        type="color"
        value={shown}
        onChange={(e) => setPreview(e.target.value)}
        className="absolute inset-0 size-full cursor-pointer opacity-0"
      />
    </label>
  )
}
