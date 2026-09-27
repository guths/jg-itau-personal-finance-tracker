import { EyeIcon, EyeOffIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Kbd } from "@/components/ui/kbd"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { usePrivacy } from "@/lib/privacy"

/** Olho na barra superior: esconde/mostra todos os valores em R$ (Shift+H). */
export function PrivacyToggle() {
  const { hidden, toggle } = usePrivacy()
  const label = hidden ? "Mostrar valores" : "Esconder valores"
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={label} aria-pressed={hidden} onClick={toggle}>
          {hidden ? <EyeOffIcon /> : <EyeIcon />}
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        {label} <Kbd>Shift+H</Kbd>
      </TooltipContent>
    </Tooltip>
  )
}
