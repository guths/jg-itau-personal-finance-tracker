import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { formatMonthLong } from "@/lib/format"
import type { Month } from "@/lib/types"

const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1)

interface Props {
  months: Month[] // mais recente primeiro
  value: Month
  onChange: (m: Month) => void
}

export function MonthSelect({ months, value, onChange }: Props) {
  const i = months.indexOf(value)
  const older = i >= 0 ? months[i + 1] : undefined
  const newer = i > 0 ? months[i - 1] : undefined
  return (
    <div className="flex items-center gap-1">
      <Button variant="outline" size="icon" aria-label="Mês anterior" disabled={!older} onClick={() => older && onChange(older)}>
        <ChevronLeftIcon />
      </Button>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger className="w-48" aria-label="Mês">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {months.map((m) => (
            <SelectItem key={m} value={m}>
              {capitalize(formatMonthLong(m))}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Button variant="outline" size="icon" aria-label="Próximo mês" disabled={!newer} onClick={() => newer && onChange(newer)}>
        <ChevronRightIcon />
      </Button>
    </div>
  )
}
