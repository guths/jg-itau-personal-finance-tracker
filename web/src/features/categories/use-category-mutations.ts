import { useMutation, useQueryClient } from "@tanstack/react-query"

import { api } from "@/lib/api"
import type { CategoryCreate, CategoryPatch } from "@/lib/types"

/** Criar/alterar categorias: nomes e cores aparecem em todas as telas, então recarrega tudo. */
export function useCategoryMutations() {
  const qc = useQueryClient()
  const refresh = () => qc.invalidateQueries()
  const create = useMutation({ mutationFn: (d: CategoryCreate) => api.createCategory(d), onSuccess: refresh })
  const update = useMutation({
    mutationFn: ({ id, data }: { id: number; data: CategoryPatch }) => api.updateCategory(id, data),
    onSuccess: refresh,
  })
  return { create, update }
}
