import { CheckIcon, SkipForwardIcon } from "lucide-react"
import { useState } from "react"

import { Money } from "@/components/money"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export interface HistoryEntry {
  seq: number
  title: string
  totalCents: number
  answer: string | null // null = pulada
  remember?: boolean
}

const VISIBLE = 3

/** Respostas desta sessão, como um chat, acima da pergunta atual. */
export function ChatHistory({ entries }: { entries: HistoryEntry[] }) {
  const [all, setAll] = useState(false)
  if (entries.length === 0) return null
  const hidden = all ? 0 : Math.max(0, entries.length - VISIBLE)
  const shown = entries.slice(hidden)

  return (
    <div className="space-y-3" aria-label="Respostas desta sessão">
      {hidden > 0 && (
        <div className="flex justify-center">
          <Button variant="ghost" size="xs" onClick={() => setAll(true)}>
            Mostrar {hidden} {hidden === 1 ? "anterior" : "anteriores"}
          </Button>
        </div>
      )}
      {shown.map((e) => (
        <div key={e.seq} className="space-y-1.5 text-sm">
          <div className="flex">
            <div className="max-w-[85%] rounded-2xl rounded-bl-sm bg-muted px-3.5 py-2">
              <span className="font-medium">{e.title}</span>
              <span className="text-muted-foreground"> · </span>
              <Money cents={e.totalCents} className="text-muted-foreground" />
            </div>
          </div>
          <div className="flex justify-end">
            <div
              className={cn(
                "flex max-w-[85%] items-center gap-1.5 rounded-2xl rounded-br-sm px-3.5 py-2",
                e.answer ? "bg-primary text-primary-foreground" : "border border-dashed text-muted-foreground",
              )}
            >
              {e.answer ? <CheckIcon className="size-3.5 shrink-0" /> : <SkipForwardIcon className="size-3.5 shrink-0" />}
              <span>{e.answer ?? "Pulada — volta no fim"}</span>
              {e.answer && e.remember === false && <span className="opacity-70">(vai perguntar de novo)</span>}
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
