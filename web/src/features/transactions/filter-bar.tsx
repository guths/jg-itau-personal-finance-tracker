import { XIcon } from "lucide-react"
import { useId } from "react"

import { SearchInput } from "@/components/search-input"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { ALL_MONTHS, hasActiveFilters, type FilterState } from "@/features/transactions/filters"
import { useAccounts, useCategories } from "@/hooks/queries"
import { formatMonth } from "@/lib/format"
import { TX_KIND_LABEL } from "@/lib/labels"
import type { Month, TxKind } from "@/lib/types"

const ANY = "any"

interface Props {
  state: FilterState
  months: Month[]
  onChange: (patch: Partial<FilterState>) => void
  onClear: () => void
  searchKey: number
}

export function FilterBar({ state, months, onChange, onClear, searchKey }: Props) {
  const accounts = useAccounts().data ?? []
  const categories = (useCategories().data ?? []).filter((c) => !c.archived)
  const switchId = useId()
  const month = state.month ?? months[0] ?? ALL_MONTHS

  return (
    <div className="flex flex-col gap-3 rounded-xl border bg-card p-3">
      <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
        <Select value={month} onValueChange={(v) => onChange({ month: v })}>
          <SelectTrigger className="w-full" aria-label="Mês">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_MONTHS}>Todos os meses</SelectItem>
            {months.map((m) => (
              <SelectItem key={m} value={m}>
                {formatMonth(m)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={state.account ? String(state.account) : ANY} onValueChange={(v) => onChange({ account: v === ANY ? null : Number(v) })}>
          <SelectTrigger className="w-full" aria-label="Conta">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY}>Todas as contas</SelectItem>
            {accounts.map((a) => (
              <SelectItem key={a.id} value={String(a.id)}>
                {a.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={state.category ? String(state.category) : ANY} onValueChange={(v) => onChange({ category: v === ANY ? null : Number(v) })}>
          <SelectTrigger className="w-full" aria-label="Categoria">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY}>Todas as categorias</SelectItem>
            {categories.map((c) => (
              <SelectGroup key={c.id}>
                <SelectLabel>{c.name}</SelectLabel>
                <SelectItem value={String(c.id)}>{c.name} (toda)</SelectItem>
                {c.subcategories
                  .filter((s) => !s.archived)
                  .map((s) => (
                    <SelectItem key={s.id} value={String(s.id)} className="pl-6">
                      {c.name} › {s.name}
                    </SelectItem>
                  ))}
              </SelectGroup>
            ))}
          </SelectContent>
        </Select>

        <Select value={state.kind ?? ANY} onValueChange={(v) => onChange({ kind: v === ANY ? null : (v as TxKind) })}>
          <SelectTrigger className="w-full" aria-label="Tipo">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ANY}>Todos os tipos</SelectItem>
            {Object.entries(TX_KIND_LABEL).map(([k, label]) => (
              <SelectItem key={k} value={k}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <SearchInput
          key={searchKey}
          value={state.q}
          onChange={(q) => onChange({ q })}
          placeholder="Buscar descrição, chave ou apelido…"
          className="flex-1"
        />
        <div className="flex items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <Switch id={switchId} checked={state.uncategorized} onCheckedChange={(v) => onChange({ uncategorized: v })} />
            <Label htmlFor={switchId}>Só sem categoria</Label>
          </div>
          {hasActiveFilters(state) && (
            <Button variant="ghost" size="sm" onClick={onClear}>
              <XIcon /> Limpar
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}
