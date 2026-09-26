import { useQuery } from "@tanstack/react-query"
import { useMemo } from "react"

import { api } from "@/lib/api"
import { qk } from "@/lib/query"
import type { Category } from "@/lib/types"

export function useCategories() {
  return useQuery({ queryKey: qk.categories, queryFn: api.categories, staleTime: 5 * 60_000 })
}

export function useAccounts() {
  return useQuery({ queryKey: qk.accounts, queryFn: api.accounts, staleTime: 5 * 60_000 })
}

export function useMonths() {
  return useQuery({ queryKey: qk.months, queryFn: api.months })
}

/** Perguntas em aberto (pendentes primeiro, depois as puladas). */
export function useOpenQuestions() {
  return useQuery({ queryKey: qk.questions, queryFn: () => api.questions() })
}

export interface SubcategoryOption {
  id: number
  path: string
  categoryName: string
  name: string
  color: string
}

/** Subcategorias ativas como "Categoria › Subcategoria", na ordem da árvore. */
export function subcategoryOptions(categories: readonly Category[] | undefined): SubcategoryOption[] {
  const out: SubcategoryOption[] = []
  for (const c of categories ?? []) {
    if (c.archived) continue
    for (const s of c.subcategories) {
      if (s.archived) continue
      out.push({ id: s.id, path: `${c.name} › ${s.name}`, categoryName: c.name, name: s.name, color: c.color })
    }
  }
  return out
}

export function useSubcategoryOptions() {
  const { data } = useCategories()
  return useMemo(() => subcategoryOptions(data), [data])
}
