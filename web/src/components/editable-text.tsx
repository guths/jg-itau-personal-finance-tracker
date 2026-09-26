import { CheckIcon, PencilIcon, XIcon } from "lucide-react"
import { useState } from "react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

interface Props {
  value: string
  onSave: (v: string) => void
  label: string
  placeholder?: string
  maxLength?: number
  allowEmpty?: boolean
  disabled?: boolean
  className?: string
}

/** Texto com edição no lugar: lápis → campo; Enter salva, Esc cancela. */
export function EditableText({ value, onSave, label, placeholder, maxLength, allowEmpty, disabled, className }: Props) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(value)

  function start() {
    setDraft(value)
    setEditing(true)
  }
  function save() {
    const v = draft.trim()
    setEditing(false)
    if (v !== value.trim() && (allowEmpty || v)) onSave(v)
  }

  if (editing) {
    return (
      <div className={cn("flex min-w-0 items-center gap-1", className)}>
        <Input
          autoFocus
          value={draft}
          maxLength={maxLength}
          placeholder={placeholder}
          aria-label={label}
          className="h-7"
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") save()
            if (e.key === "Escape") setEditing(false)
          }}
        />
        <Button size="icon-sm" variant="ghost" aria-label="Salvar" onClick={save}>
          <CheckIcon />
        </Button>
        <Button size="icon-sm" variant="ghost" aria-label="Cancelar" onClick={() => setEditing(false)}>
          <XIcon />
        </Button>
      </div>
    )
  }
  return (
    <div className={cn("group/edit flex min-w-0 items-center gap-1", className)}>
      <span className={cn("truncate", !value && "text-muted-foreground")}>{value || placeholder || "—"}</span>
      <Button
        size="icon-xs"
        variant="ghost"
        aria-label={label}
        disabled={disabled}
        className="shrink-0 opacity-60 group-hover/edit:opacity-100 focus-visible:opacity-100"
        onClick={start}
      >
        <PencilIcon />
      </Button>
    </div>
  )
}
