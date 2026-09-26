import { ArrowRightIcon, CircleHelpIcon } from "lucide-react"
import { Link } from "react-router"

import { Money } from "@/components/money"
import { Button } from "@/components/ui/button"
import { plural } from "@/lib/format"
import type { DashboardSummary } from "@/lib/types"

export function UncategorizedAlert({ uncategorized: u }: { uncategorized: DashboardSummary["uncategorized"] }) {
  if (u.count === 0) return null
  return (
    <div
      role="status"
      className="flex flex-col gap-3 rounded-xl border border-amber-500/40 bg-amber-500/5 px-4 py-3 text-sm sm:flex-row sm:items-center"
    >
      <CircleHelpIcon className="hidden size-4 shrink-0 text-amber-600 sm:block dark:text-amber-400" />
      <div className="min-w-0 flex-1">
        <p className="font-medium">{plural(u.count, "lançamento sem categoria", "lançamentos sem categoria")}</p>
        <p className="text-muted-foreground">
          <Money cents={u.total} /> ainda não entram em nenhuma categoria deste mês.
        </p>
      </div>
      <Button size="sm" asChild className="self-start sm:self-auto">
        <Link to="/perguntas">
          Responder <ArrowRightIcon />
        </Link>
      </Button>
    </div>
  )
}
