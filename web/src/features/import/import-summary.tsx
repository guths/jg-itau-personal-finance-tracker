import { ArrowRightIcon, MessageCircleQuestionIcon, SparklesIcon, TriangleAlertIcon } from "lucide-react"
import { Link } from "react-router"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { FileResultCard } from "@/features/import/file-result-card"
import { plural } from "@/lib/format"
import type { ImportResponse } from "@/lib/types"

export function ImportSummary({ data }: { data: ImportResponse }) {
  const { auto, questions } = data.categorization
  return (
    <section className="space-y-4" aria-label="Resultado da importação">
      <Card>
        <CardContent className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex flex-wrap gap-x-6 gap-y-2 text-sm">
            <span className="flex items-center gap-2">
              <SparklesIcon className="size-4 text-muted-foreground" />
              <span>
                <strong className="font-semibold tabular-nums">{auto}</strong>{" "}
                {auto === 1 ? "categorizado automaticamente" : "categorizados automaticamente"}
              </span>
            </span>
            <span className="flex items-center gap-2">
              <MessageCircleQuestionIcon className="size-4 text-muted-foreground" />
              <span>
                <strong className="font-semibold tabular-nums">{questions}</strong>{" "}
                {questions === 1 ? "pergunta para você" : "perguntas para você"}
              </span>
            </span>
          </div>
          {questions > 0 && (
            <Button asChild>
              <Link to="/perguntas">
                Responder {plural(questions, "pergunta", "perguntas")} <ArrowRightIcon />
              </Link>
            </Button>
          )}
        </CardContent>
      </Card>

      {data.warnings.length > 0 && (
        <Alert className="border-amber-500/40 text-amber-800 dark:text-amber-300">
          <TriangleAlertIcon />
          <AlertTitle>Avisos</AlertTitle>
          <AlertDescription>
            <ul className="list-disc space-y-1 pl-4">
              {data.warnings.map((w) => (
                <li key={w}>{w}</li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      )}

      <div className="grid gap-3 md:grid-cols-2">
        {data.results.map((r, i) => (
          <FileResultCard key={`${r.name}-${i}`} result={r} />
        ))}
      </div>
    </section>
  )
}
