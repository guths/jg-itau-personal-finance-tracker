import { SearchIcon } from "lucide-react"
import { useEffect, useRef, useState } from "react"

import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

interface Props {
  value: string
  onChange: (v: string) => void
  placeholder?: string
  className?: string
  delay?: number
}

/** Campo de busca com atraso (não dispara uma consulta por tecla). Remonte com `key` para limpar. */
export function SearchInput({ value, onChange, placeholder = "Buscar…", className, delay = 300 }: Props) {
  const [text, setText] = useState(value)
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)
  useEffect(() => () => clearTimeout(timer.current), [])

  return (
    <div className={cn("relative", className)}>
      <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        type="search"
        value={text}
        placeholder={placeholder}
        className="pl-8"
        aria-label={placeholder}
        onChange={(e) => {
          const v = e.target.value
          setText(v)
          clearTimeout(timer.current)
          timer.current = setTimeout(() => onChange(v), delay)
        }}
      />
    </div>
  )
}
