import { BotIcon, CalendarIcon, CreditCardIcon, MapPinIcon, TagIcon, UserIcon } from "lucide-react"
import { useState, type ReactNode } from "react"

import { Money } from "@/components/money"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { questionFacts } from "@/features/questions/queue"
import { formatDayMonth, plural } from "@/lib/format"
import type { Question } from "@/lib/types"

function Fact({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <span className="flex min-w-0 items-center gap-1.5 text-muted-foreground [&_svg]:size-3.5 [&_svg]:shrink-0">
      {icon}
      <span className="truncate">{children}</span>
    </span>
  )
}

const VISIBLE = 4

export function QuestionDetails({ question: q }: { question: Question }) {
  const [expanded, setExpanded] = useState(false)
  const facts = questionFacts(q)
  const title = q.alias || q.transactions[0]?.description || q.key
  const txs = expanded ? q.transactions : q.transactions.slice(0, VISIBLE)
  const bank = [...facts.bankCategories].join(", ")
  const cities = facts.cities.join(", ")

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <h2 className="text-xl font-semibold tracking-tight break-words">{title}</h2>
          {title !== q.key && <p className="font-mono text-xs break-all text-muted-foreground">{q.key}</p>}
          <div className="flex flex-wrap gap-1.5 pt-1">
            {q.person_transfer && (
              <Badge variant="secondary">
                <UserIcon /> PIX para pessoa
              </Badge>
            )}
            {q.per_transaction && <Badge variant="outline">pergunta por lançamento</Badge>}
            {q.status === "skipped" && <Badge variant="outline">pulada antes</Badge>}
          </div>
        </div>
        <div className="text-right">
          <Money cents={q.total_cents} signed className="text-2xl font-semibold tracking-tight" />
          <p className="text-xs text-muted-foreground">{plural(q.count, "lançamento", "lançamentos")}</p>
        </div>
      </div>

      <div className="grid gap-x-4 gap-y-1.5 text-sm sm:grid-cols-2">
        <Fact icon={<CalendarIcon />}>{facts.dates}</Fact>
        <Fact icon={<CreditCardIcon />}>{facts.accounts.join(", ") || "—"}</Fact>
        {bank && <Fact icon={<TagIcon />}>Banco: {bank}</Fact>}
        {cities && <Fact icon={<MapPinIcon />}>{cities}</Fact>}
      </div>

      {q.prompt && (
        <div className="flex gap-3 rounded-xl bg-muted/60 p-3 text-sm">
          <BotIcon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
          <p>{q.prompt}</p>
        </div>
      )}

      {q.transactions.length > 1 && (
        <div className="rounded-lg border text-sm">
          <ul className="divide-y">
            {txs.map((t) => (
              <li key={t.id} className="flex items-center gap-3 px-3 py-2">
                <span className="w-11 shrink-0 text-xs text-muted-foreground tabular-nums">{formatDayMonth(t.date)}</span>
                <span className="min-w-0 flex-1 truncate">{t.description}</span>
                <Money cents={t.amount_cents} signed className="text-sm" />
              </li>
            ))}
          </ul>
          {q.transactions.length > VISIBLE && (
            <Button variant="ghost" size="sm" className="w-full rounded-t-none border-t" onClick={() => setExpanded((v) => !v)}>
              {expanded ? "Mostrar menos" : `Ver todos os ${q.transactions.length}`}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
