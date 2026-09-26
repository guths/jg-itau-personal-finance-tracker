import { TriangleAlertIcon } from "lucide-react"
import { isRouteErrorResponse, useRouteError } from "react-router"

import { EmptyState } from "@/components/empty-state"
import { Button } from "@/components/ui/button"

/** Erro de renderização numa tela: mostra o problema sem derrubar o layout. */
export function RouteError() {
  const error = useRouteError()
  const message = isRouteErrorResponse(error)
    ? `${error.status} ${error.statusText}`
    : error instanceof Error
      ? error.message
      : "Erro desconhecido"
  return (
    <EmptyState icon={TriangleAlertIcon} title="Esta tela encontrou um erro" description={message}>
      <Button variant="outline" onClick={() => window.location.reload()}>
        Recarregar
      </Button>
    </EmptyState>
  )
}
