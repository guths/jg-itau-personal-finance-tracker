import { CompassIcon } from "lucide-react"
import { Link } from "react-router"

import { EmptyState } from "@/components/empty-state"
import { Button } from "@/components/ui/button"

export function NotFoundPage() {
  return (
    <EmptyState icon={CompassIcon} title="Página não encontrada" description="Esse endereço não existe.">
      <Button asChild>
        <Link to="/">Ir para o dashboard</Link>
      </Button>
    </EmptyState>
  )
}
