import { keepPreviousData, useQuery } from "@tanstack/react-query"
import { BarChart3Icon, UploadIcon } from "lucide-react"
import { Link, useSearchParams } from "react-router"

import { EmptyState } from "@/components/empty-state"
import { PageHeader } from "@/components/page-header"
import { QueryError } from "@/components/query-error"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { CategoryBreakdown } from "@/features/dashboard/category-breakdown"
import { DashboardSkeleton } from "@/features/dashboard/dashboard-skeleton"
import { EvolutionCard } from "@/features/dashboard/evolution-card"
import { KpiCards } from "@/features/dashboard/kpi-cards"
import { MonthSelect } from "@/features/dashboard/month-select"
import { RecurringCard } from "@/features/dashboard/recurring-card"
import { TopMerchantsCard } from "@/features/dashboard/top-merchants-card"
import { UncategorizedAlert } from "@/features/dashboard/uncategorized-alert"
import { UpcomingCard } from "@/features/dashboard/upcoming-card"
import { useMonths } from "@/hooks/queries"
import { api } from "@/lib/api"
import { formatMonthLong } from "@/lib/format"
import { qk } from "@/lib/query"
import { cn } from "@/lib/utils"

function ImportCta() {
  return (
    <Button asChild>
      <Link to="/importar">
        <UploadIcon /> Importar PDFs
      </Link>
    </Button>
  )
}

export function DashboardPage() {
  const [params, setParams] = useSearchParams()
  const months = useMonths()
  const list = months.data ?? []
  const wanted = params.get("mes")
  const month = wanted && list.includes(wanted) ? wanted : list[0]

  const dashboard = useQuery({
    queryKey: qk.dashboard(month),
    queryFn: () => api.dashboard(month),
    enabled: !!month,
    placeholderData: keepPreviousData,
  })

  const header = (
    <PageHeader
      title="Dashboard"
      description={month ? `Para onde foi o dinheiro em ${formatMonthLong(month)}.` : "Para onde vai o dinheiro, mês a mês."}
      actions={
        months.isPending ? (
          <Skeleton className="h-8 w-60" />
        ) : list.length > 0 && month ? (
          <MonthSelect months={list} value={month} onChange={(m) => setParams({ mes: m }, { replace: true })} />
        ) : null
      }
    />
  )

  if (months.error) {
    return (
      <>
        {header}
        <QueryError error={months.error} onRetry={() => months.refetch()} />
      </>
    )
  }
  if (!months.isPending && list.length === 0) {
    return (
      <>
        {header}
        <EmptyState
          icon={BarChart3Icon}
          title="Nada por aqui ainda"
          description="Importe as faturas do cartão e o extrato da conta para ver quanto você gasta por categoria."
        >
          <ImportCta />
        </EmptyState>
      </>
    )
  }

  const d = dashboard.data
  return (
    <>
      {header}
      {dashboard.error && !d ? (
        <QueryError error={dashboard.error} onRetry={() => dashboard.refetch()} />
      ) : !d || !month ? (
        <DashboardSkeleton />
      ) : (
        <div className={cn("space-y-6 transition-opacity", dashboard.isPlaceholderData && "opacity-60")}>
          <UncategorizedAlert uncategorized={d.summary.uncategorized} />
          <KpiCards summary={d.summary} />
          {d.by_category.length === 0 && d.summary.uncategorized.count === 0 ? (
            <EmptyState
              icon={BarChart3Icon}
              title={`Sem gastos em ${formatMonthLong(month)}`}
              description="Nenhum gasto neste mês de referência. Escolha outro mês ou importe mais PDFs."
            >
              <ImportCta />
            </EmptyState>
          ) : (
            <CategoryBreakdown month={month} items={d.by_category} summary={d.summary} />
          )}
          <RecurringCard items={d.recurring} fixed={d.summary.fixed} />
          <UpcomingCard upcoming={d.upcoming} />
          <div className="grid gap-6 xl:grid-cols-5">
            <div className="min-w-0 xl:col-span-3">
              <EvolutionCard months={d.evolution} />
            </div>
            <div className="min-w-0 xl:col-span-2">
              <TopMerchantsCard items={d.top_merchants} />
            </div>
          </div>
        </div>
      )}
    </>
  )
}
