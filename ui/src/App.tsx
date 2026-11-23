import { useState, useEffect, useCallback } from "react"
import { useTheme } from "next-themes"
import { Toaster } from "@/components/ui/sonner"
import { toast } from "sonner"
import { api, type TokenRecord } from "@/lib/api"
import { auth } from "@/lib/auth"
import { AppSidebar } from "@/components/app-sidebar"
import { SiteHeader } from "@/components/site-header"
import { LoginView } from "@/components/LoginView"
import { SecretsView } from "@/components/SecretsView"
import { NamespacesView } from "@/components/NamespacesView"
import { TokensView } from "@/components/TokensView"
import { IntegrationsView } from "@/components/IntegrationsView"
import { DashboardView } from "@/components/DashboardView"
import { KubernetesView } from "@/components/KubernetesView"
import { AuditView } from "@/components/AuditView"
import { UsersView } from "@/components/UsersView"
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"

export type View = "dashboard" | "secrets" | "namespaces" | "tokens" | "integrations" | "kubernetes" | "audit" | "users"

const ALL_VIEWS: View[] = ["dashboard", "secrets", "namespaces", "tokens", "integrations", "kubernetes", "audit", "users"]

function getInitialView(): View {
  const hash = window.location.hash.slice(1) as View
  return ALL_VIEWS.includes(hash) ? hash : "secrets"
}

export { toast }

export default function App() {
  const [view, setView] = useState<View>(getInitialView)
  const [token, setToken] = useState(auth.get)
  const [sealed, setSealed] = useState(true)
  const [statusLoaded, setStatusLoaded] = useState(false)
  const [tokenInfo, setTokenInfo] = useState<TokenRecord | null>(null)
  const { resolvedTheme } = useTheme()

  const pollStatus = useCallback(async () => {
    const s = await api.status().catch(() => "unknown")
    setSealed(s === "sealed")
    setStatusLoaded(true)
  }, [])

  useEffect(() => {
    pollStatus()
    const id = setInterval(pollStatus, 5000)
    return () => clearInterval(id)
  }, [pollStatus])

  useEffect(() => {
    if (!token) { setTokenInfo(null); return }
    api.tokenMe(token)
      .then(setTokenInfo)
      .catch(() => {
        // Token revoked or invalid — clear it so user re-enters
        auth.clear()
        setToken("")
        setTokenInfo(null)
      })
  }, [token])

  useEffect(() => {
    if (token && !sealed) window.location.hash = view
  }, [view, token, sealed])

  useEffect(() => {
    const onHash = () => {
      const h = window.location.hash.slice(1) as View
      if ((["secrets", "namespaces", "tokens"] as View[]).includes(h)) setView(h)
    }
    window.addEventListener("hashchange", onHash)
    return () => window.removeEventListener("hashchange", onHash)
  }, [])

  // Save a new token (config step — doesn't navigate anywhere)
  const handleSaveToken = (t: string) => {
    auth.set(t)
    setToken(t)
  }

  // Clear saved token so the user can enter a different one
  const handleClearToken = () => {
    auth.clear()
    setToken("")
    setTokenInfo(null)
  }

  // Logout: clear token, user will need to re-enter it next time
  const handleLogout = () => {
    auth.clear()
    setToken("")
    setTokenInfo(null)
  }

  // Seal: just locks the vault — token is kept, user only needs passphrase to re-enter
  const handleSeal = async () => {
    try {
      await api.seal(token)
      setSealed(true)
      toast.success("Vault sealed")
    } catch (e) {
      toast.error("Failed to seal vault", { description: String(e) })
    }
  }

  const toasterTheme = resolvedTheme === "light" ? "light" : "dark"

  // Wait for first status poll to avoid flashing the login screen on load
  if (!statusLoaded) {
    return (
      <div className="flex h-full w-full items-center justify-center bg-background">
        <div className="h-4 w-4 animate-spin rounded-full border-2 border-muted border-t-foreground" />
      </div>
    )
  }

  // Show login/unseal screen when no token is configured or vault is locked
  if (!token || sealed) {
    return (
      <>
        <LoginView
          savedToken={token}
          onSaveToken={handleSaveToken}
          onClearToken={handleClearToken}
          onSealed={setSealed}
          sealed={sealed}
        />
        <Toaster richColors theme={toasterTheme} position="bottom-right" />
      </>
    )
  }

  return (
    <SidebarProvider className="bg-sidebar">
      <AppSidebar
        view={view}
        onNavigate={setView}
        tokenInfo={tokenInfo}
        onLogout={handleLogout}
        onSeal={handleSeal}
      />
      <SidebarInset className="mt-2 mr-2 rounded-tl-xl rounded-tr-xl overflow-hidden">
        <SiteHeader view={view} />
        <div className="flex flex-1 flex-col overflow-auto">
          {view === "secrets"    && <SecretsView    token={token} tokenInfo={tokenInfo} sealed={false} />}
          {view === "namespaces" && <NamespacesView token={token} tokenInfo={tokenInfo} />}
          {view === "tokens"     && <TokensView     token={token} tokenInfo={tokenInfo} />}
          {view === "dashboard"     && <DashboardView token={token} />}
          {view === "integrations" && <IntegrationsView />}
          {view === "kubernetes"   && <KubernetesView   token={token} />}
          {view === "audit"       && <AuditView        token={token} />}
          {view === "users"       && <UsersView        token={token} tokenInfo={tokenInfo} />}
        </div>
      </SidebarInset>
      <Toaster richColors theme={toasterTheme} position="bottom-right" />
    </SidebarProvider>
  )
}
