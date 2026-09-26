import { AlertCircleIcon, RotateCwIcon } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"

interface Props {
  error: unknown
  onRetry?: () => void
  title?: string
}

/** Erro de carregamento dentro da tela (o toast já avisou; aqui a tela não quebra). */
export function QueryError({ error, onRetry, title = "Não foi possível carregar" }: Props) {
  const message = error instanceof Error ? error.message : "Algo deu errado"
  return (
    <Alert variant="destructive">
      <AlertCircleIcon />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>
        <p>{message}</p>
        {onRetry && (
          <Button variant="outline" size="sm" className="mt-2" onClick={onRetry}>
            <RotateCwIcon /> Tentar de novo
          </Button>
        )}
      </AlertDescription>
    </Alert>
  )
}
