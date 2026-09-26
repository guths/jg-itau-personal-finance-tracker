import {
  LayoutDashboardIcon,
  MessageCircleQuestionIcon,
  ReceiptTextIcon,
  TagsIcon,
  UploadIcon,
  WalletIcon,
  type LucideIcon,
} from "lucide-react"
import { NavLink, useLocation } from "react-router"

import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuBadge,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from "@/components/ui/sidebar"
import { useOpenQuestions } from "@/hooks/queries"

interface NavItem {
  to: string
  label: string
  icon: LucideIcon
  badge?: "questions"
}

const NAV: NavItem[] = [
  { to: "/", label: "Dashboard", icon: LayoutDashboardIcon },
  { to: "/importar", label: "Importar", icon: UploadIcon },
  { to: "/perguntas", label: "Perguntas", icon: MessageCircleQuestionIcon, badge: "questions" },
  { to: "/lancamentos", label: "Lançamentos", icon: ReceiptTextIcon },
  { to: "/categorias", label: "Categorias e regras", icon: TagsIcon },
]

function isActive(pathname: string, to: string) {
  return to === "/" ? pathname === "/" : pathname === to || pathname.startsWith(`${to}/`)
}

export function AppSidebar() {
  const { pathname } = useLocation()
  const { setOpenMobile } = useSidebar()
  const questions = useOpenQuestions()
  const open = questions.data?.length ?? 0

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild>
              <NavLink to="/" onClick={() => setOpenMobile(false)}>
                <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                  <WalletIcon className="size-4" />
                </div>
                <div className="grid flex-1 text-left leading-tight">
                  <span className="truncate font-semibold">JG Finanças</span>
                  <span className="truncate text-xs text-muted-foreground">Gastos pessoais</span>
                </div>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {NAV.map((item) => (
                <SidebarMenuItem key={item.to}>
                  <SidebarMenuButton asChild isActive={isActive(pathname, item.to)} tooltip={item.label}>
                    <NavLink to={item.to} onClick={() => setOpenMobile(false)}>
                      <item.icon />
                      <span>{item.label}</span>
                    </NavLink>
                  </SidebarMenuButton>
                  {item.badge === "questions" && open > 0 && (
                    <SidebarMenuBadge className="rounded-full bg-primary px-1.5 text-primary-foreground peer-data-active/menu-button:text-primary-foreground">
                      {open}
                    </SidebarMenuBadge>
                  )}
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarRail />
    </Sidebar>
  )
}
