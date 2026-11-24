import * as React from "react"
import { KeyRound, Layers, Coins, Lock, LockKeyhole, Plug, LayoutDashboard, Container, ScrollText, Users } from "lucide-react"
import runeIcon from "@/assets/rune-icon.png"

import { NavUser } from "@/components/nav-user"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarSeparator,
} from "@/components/ui/sidebar"
import type { TokenRecord } from "@/lib/api"
import type { View } from "@/App"
import { cn } from "@/lib/utils"

interface NavItem {
  id: View
  label: string
  icon: React.ElementType
  rootOnly: boolean
}

const NAV_ITEMS: NavItem[] = [
  { id: "secrets",    label: "Secrets",    icon: KeyRound,   rootOnly: false },
  { id: "namespaces", label: "Namespaces", icon: Layers,     rootOnly: true  },
  { id: "tokens",     label: "Tokens",     icon: Coins,      rootOnly: true  },
  { id: "users",      label: "Users",      icon: Users,      rootOnly: true  },
  { id: "audit",      label: "Audit Log",  icon: ScrollText, rootOnly: true  },
]

interface AppSidebarProps extends React.ComponentProps<typeof Sidebar> {
  view: View
  onNavigate: (v: View) => void
  tokenInfo: TokenRecord | null
  onLogout: () => void
  onSeal: () => void
}

export function AppSidebar({ view, onNavigate, tokenInfo, onLogout, onSeal, ...props }: AppSidebarProps) {
  const isRoot = tokenInfo?.namespace === "*"

  return (
    <Sidebar collapsible="icon" {...props}>
      {/* Logo */}
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" asChild className="pointer-events-none select-none">
              <div>
                <img src={runeIcon} alt="rune" className="size-8 rounded-lg object-contain" />
                <div className="grid flex-1 text-left text-sm leading-tight">
                  <span className="truncate font-semibold">Rune Vault</span>
                  <span className="truncate text-xs font-mono text-emerald-600 dark:text-emerald-400">
                    unsealed
                  </span>
                </div>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      {/* Nav */}
      <SidebarContent>
        {/* Dashboard — root only */}
        {isRoot && (
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu>
                <SidebarMenuItem>
                  <SidebarMenuButton
                    isActive={view === "dashboard"}
                    onClick={() => onNavigate("dashboard")}
                    tooltip="Dashboard"
                  >
                    <LayoutDashboard />
                    <span>Dashboard</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        )}

        <SidebarGroup>
          <SidebarGroupLabel>Vault</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {NAV_ITEMS.map(item => {
                const locked = item.rootOnly && !isRoot
                const Icon = item.icon
                return (
                  <SidebarMenuItem key={item.id}>
                    <SidebarMenuButton
                      isActive={view === item.id}
                      onClick={() => onNavigate(item.id)}
                      className={cn(locked && "opacity-40 cursor-default pointer-events-none")}
                      tooltip={locked ? "Root access required" : item.label}
                    >
                      <Icon />
                      <span>{item.label}</span>
                      {locked && <Lock className="ml-auto size-3 opacity-60" />}
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                )
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        {/* Integrations — root only */}
        {isRoot && (
          <>
            <SidebarSeparator />
            <SidebarGroup>
              <SidebarGroupLabel>Integrations</SidebarGroupLabel>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      isActive={view === "integrations"}
                      onClick={() => onNavigate("integrations")}
                      tooltip="Integrations"
                    >
                      <Plug />
                      <span>Integrations</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      isActive={view === "kubernetes"}
                      onClick={() => onNavigate("kubernetes")}
                      tooltip="Kubernetes"
                    >
                      <Container />
                      <span>Kubernetes</span>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </>
        )}

        {/* Seal vault — root only */}
        {isRoot && (
          <>
            <SidebarSeparator />
            <SidebarGroup>
              <SidebarGroupContent>
                <SidebarMenu>
                  <SidebarMenuItem>
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <SidebarMenuButton tooltip="Lock the vault until next passphrase entry">
                          <LockKeyhole />
                          <span>Seal vault</span>
                        </SidebarMenuButton>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Seal the vault?</AlertDialogTitle>
                          <AlertDialogDescription>
                            The vault will be locked. You can unseal it again at any time with your passphrase.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction onClick={onSeal}>
                            Seal vault
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </SidebarMenuItem>
                </SidebarMenu>
              </SidebarGroupContent>
            </SidebarGroup>
          </>
        )}
      </SidebarContent>

      {/* User footer */}
      <SidebarFooter>
        <NavUser tokenInfo={tokenInfo} onLogout={onLogout} onSeal={onSeal} isRoot={isRoot} />
      </SidebarFooter>
    </Sidebar>
  )
}
