import { useMutation, useQueryClient } from "@tanstack/react-query"

import { PageHeader } from "@/components/page-header"
import { Card, CardContent } from "@/components/ui/card"
import { Spinner } from "@/components/ui/spinner"
import { DocumentsTable } from "@/features/import/documents-table"
import { Dropzone } from "@/features/import/dropzone"
import { ImportSummary } from "@/features/import/import-summary"
import { api } from "@/lib/api"
import { plural } from "@/lib/format"
import { invalidateSpending, qk } from "@/lib/query"

export function ImportPage() {
  const qc = useQueryClient()
  const mutation = useMutation({
    mutationFn: api.importFiles,
    onSuccess: () => {
      void invalidateSpending(qc)
      void qc.invalidateQueries({ queryKey: qk.accounts })
    },
  })

  return (
    <div className="space-y-8">
      <PageHeader
        title="Importar"
        description="Faturas de cartão e extratos em PDF. Cada arquivo é conferido centavo a centavo com os totais impressos."
      />

      <Dropzone disabled={mutation.isPending} onSubmit={(files) => mutation.mutate(files)} />

      {mutation.isPending && (
        <Card>
          <CardContent className="flex items-center gap-4">
            <Spinner className="size-5" />
            <div className="space-y-0.5">
              <p className="font-medium">Importando e categorizando…</p>
              <p className="text-sm text-muted-foreground">
                {plural(mutation.variables.length, "arquivo", "arquivos")} — lotes grandes podem levar alguns minutos.
              </p>
            </div>
          </CardContent>
        </Card>
      )}

      {mutation.data && !mutation.isPending && <ImportSummary data={mutation.data} />}

      <section className="space-y-3">
        <h2 className="text-lg font-semibold tracking-tight">Documentos importados</h2>
        <DocumentsTable />
      </section>
    </div>
  )
}
