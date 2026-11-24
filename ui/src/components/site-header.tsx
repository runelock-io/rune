import { useTheme } from "next-themes"
import { Sun, Moon } from "lucide-react"
import { SidebarTrigger } from "@/components/ui/sidebar"
import { Separator } from "@/components/ui/separator"
import { Button } from "@/components/ui/button"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbList,
  BreadcrumbPage,
} from "@/components/ui/breadcrumb"
import type { View } from "@/App"

const VIEW_LABELS: Record<View, string> = {
  dashboard:    "Dashboard",
  secrets:      "Secrets",
  namespaces:   "Namespaces",
  tokens:       "Tokens",
  users:        "Users",
  integrations: "Integrations",
  kubernetes:   "Kubernetes",
  audit:        "Audit Log",
}

interface Props {
  view: View
}

export function SiteHeader({ view }: Props) {
  const { resolvedTheme, setTheme } = useTheme()

  return (
    <header className="flex h-12 shrink-0 items-center gap-2 border-b border-border px-4">
      <SidebarTrigger className="-ml-1" />
      <Separator orientation="vertical" className="mr-2 h-4" />
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbPage className="text-sm">{VIEW_LABELS[view]}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <div className="ml-auto">
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8"
          onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
          title="Toggle theme"
        >
          {resolvedTheme === "dark"
            ? <Sun className="h-4 w-4" />
            : <Moon className="h-4 w-4" />
          }
        </Button>
      </div>
    </header>
  )
}
