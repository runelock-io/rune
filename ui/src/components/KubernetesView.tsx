import { useEffect, useState, useCallback } from "react"
import { toast } from "sonner"
import { api, type K8sMappingWithStatus } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { Plus, Trash2, RefreshCw, ChevronDown, ChevronRight, Terminal } from "lucide-react"

const SECRET_TYPES = [
  { value: "Opaque", label: "Opaque" },
  { value: "kubernetes.io/tls", label: "kubernetes.io/tls" },
  { value: "kubernetes.io/basic-auth", label: "kubernetes.io/basic-auth" },
  { value: "kubernetes.io/ssh-auth", label: "kubernetes.io/ssh-auth" },
  { value: "kubernetes.io/dockerconfigjson", label: "kubernetes.io/dockerconfigjson" },
]

const INTERVALS = [
  { value: "15s", label: "15 seconds" },
  { value: "30s", label: "30 seconds" },
  { value: "1m", label: "1 minute" },
  { value: "5m", label: "5 minutes" },
  { value: "15m", label: "15 minutes" },
]

function StatusBadge({ status }: { status?: string }) {
  if (!status) return <Badge variant="outline" className="text-xs">unknown</Badge>
  if (status === "synced")
    return <Badge className="text-xs bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border-emerald-500/20 hover:bg-emerald-500/15">synced</Badge>
  if (status === "error")
    return <Badge variant="destructive" className="text-xs">error</Badge>
  return <Badge variant="outline" className="text-xs text-yellow-600 dark:text-yellow-400 border-yellow-500/30">pending</Badge>
}

function formatTime(iso?: string) {
  if (!iso) return "—"
  const d = new Date(iso)
  return d.toLocaleString(undefined, { dateStyle: "short", timeStyle: "short" })
}

interface Props {
  token: string
}

export function KubernetesView({ token }: Props) {
  const [mappings, setMappings] = useState<K8sMappingWithStatus[]>([])
  const [loading, setLoading] = useState(true)
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [namespaces, setNamespaces] = useState<string[]>([])

  // Create dialog state
  const [open, setOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState("")
  const [runeNS, setRuneNS] = useState("")
  const [keys, setKeys] = useState("")
  const [targetSecret, setTargetSecret] = useState("")
  const [targetK8sNS, setTargetK8sNS] = useState("default")
  const [secretType, setSecretType] = useState("Opaque")
  const [interval, setInterval_] = useState("30s")

  const load = useCallback(async () => {
    try {
      const data = await api.listK8sMappings(token)
      setMappings(data ?? [])
    } catch (e) {
      toast.error("Failed to load K8s mappings", { description: String(e) })
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => {
    load()
    api.listNamespaces(token).then(setNamespaces).catch(() => {})
    const id = setInterval(load, 15000)
    return () => clearInterval(id)
  }, [load, token])

  function resetForm() {
    setName(""); setRuneNS(""); setKeys(""); setTargetSecret(""); setTargetK8sNS("default")
    setSecretType("Opaque"); setInterval_("30s")
  }

  async function handleCreate() {
    setCreating(true)
    try {
      await api.createK8sMapping(token, {
        name,
        rune_namespace: runeNS,
        keys: keys.trim() ? keys.split(",").map(k => k.trim()).filter(Boolean) : [],
        target_secret: targetSecret,
        target_k8s_ns: targetK8sNS,
        secret_type: secretType,
        refresh_interval: interval,
      })
      toast.success("Mapping created")
      setOpen(false)
      resetForm()
      load()
    } catch (e) {
      toast.error("Failed to create mapping", { description: String(e) })
    } finally {
      setCreating(false)
    }
  }

  async function handleDelete(id: string, name: string) {
    try {
      await api.deleteK8sMapping(token, id)
      toast.success(`Deleted mapping "${name}"`)
      load()
    } catch (e) {
      toast.error("Failed to delete mapping", { description: String(e) })
    }
  }

  function toggleExpand(id: string) {
    setExpanded(prev => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }

  const valid = name && runeNS && targetSecret && targetK8sNS

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Kubernetes Sync</h2>
          <p className="text-sm text-muted-foreground mt-0.5">
            Map Rune namespaces to native Kubernetes Secrets. The operator syncs them automatically.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="outline" size="icon" className="h-8 w-8" onClick={load}>
                  <RefreshCw className="h-3.5 w-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Refresh</TooltipContent>
            </Tooltip>
          </TooltipProvider>
          <Button size="sm" onClick={() => setOpen(true)}>
            <Plus className="h-4 w-4 mr-1" /> New Mapping
          </Button>
        </div>
      </div>

      {/* Operator install hint */}
      <div className="rounded-lg border border-dashed border-border bg-muted/30 p-4 flex items-start gap-3">
        <Terminal className="h-4 w-4 mt-0.5 text-muted-foreground shrink-0" />
        <div className="space-y-1">
          <p className="text-sm font-medium">Deploy the operator</p>
          <p className="text-xs text-muted-foreground">
            Set <code className="font-mono bg-muted px-1 rounded">RUNE_TOKEN</code> in the secret, then apply the manifest:
          </p>
          <code className="text-xs font-mono text-foreground block mt-1">
            kubectl apply -f https://github.com/runelock-io/rune/raw/main/config/operator/deploy.yaml
          </code>
        </div>
      </div>

      {/* Mappings table */}
      {loading ? (
        <div className="flex items-center justify-center py-16 text-muted-foreground text-sm">Loading…</div>
      ) : mappings.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 gap-2 text-muted-foreground">
          <p className="text-sm">No mappings yet.</p>
          <p className="text-xs">Create one to start syncing secrets to Kubernetes.</p>
        </div>
      ) : (
        <div className="rounded-lg border border-border overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40 hover:bg-muted/40">
                <TableHead className="w-6" />
                <TableHead>Name</TableHead>
                <TableHead>Rune Namespace</TableHead>
                <TableHead>K8s Secret</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Last Sync</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {mappings.map(m => (
                <>
                  <TableRow
                    key={m.id}
                    className="cursor-pointer"
                    onClick={() => toggleExpand(m.id)}
                  >
                    <TableCell className="text-muted-foreground">
                      {expanded.has(m.id)
                        ? <ChevronDown className="h-3.5 w-3.5" />
                        : <ChevronRight className="h-3.5 w-3.5" />}
                    </TableCell>
                    <TableCell className="font-medium">{m.name}</TableCell>
                    <TableCell>
                      <code className="text-xs font-mono bg-muted px-1.5 py-0.5 rounded">{m.rune_namespace}</code>
                    </TableCell>
                    <TableCell>
                      <span className="text-xs font-mono">
                        {m.target_k8s_ns}/{m.target_secret}
                      </span>
                    </TableCell>
                    <TableCell>
                      <code className="text-xs text-muted-foreground">{m.secret_type}</code>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {formatTime(m.sync_status?.last_sync)}
                    </TableCell>
                    <TableCell>
                      <StatusBadge status={m.sync_status?.status} />
                    </TableCell>
                    <TableCell onClick={e => e.stopPropagation()}>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button variant="ghost" size="icon" className="h-7 w-7 text-muted-foreground hover:text-destructive">
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>Delete mapping "{m.name}"?</AlertDialogTitle>
                            <AlertDialogDescription>
                              This will delete the mapping and the corresponding Kubernetes Secret{" "}
                              <code className="font-mono">{m.target_k8s_ns}/{m.target_secret}</code>.
                              This action cannot be undone.
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>Cancel</AlertDialogCancel>
                            <AlertDialogAction
                              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                              onClick={() => handleDelete(m.id, m.name)}
                            >
                              Delete
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </TableCell>
                  </TableRow>

                  {/* Expanded detail row */}
                  {expanded.has(m.id) && (
                    <TableRow key={`${m.id}-detail`} className="bg-muted/20 hover:bg-muted/20">
                      <TableCell />
                      <TableCell colSpan={7}>
                        <div className="py-2 space-y-2 text-xs text-muted-foreground">
                          <div className="grid grid-cols-2 gap-x-8 gap-y-1">
                            <span><span className="font-medium text-foreground">Refresh interval:</span> {m.refresh_interval}</span>
                            <span><span className="font-medium text-foreground">Keys:</span> {m.keys?.length ? m.keys.join(", ") : "all keys"}</span>
                            <span><span className="font-medium text-foreground">Secrets synced:</span> {m.sync_status?.secret_count ?? "—"}</span>
                            <span><span className="font-medium text-foreground">Created:</span> {formatTime(m.created_at)}</span>
                          </div>
                          {m.sync_status?.status === "error" && m.sync_status.message && (
                            <div className="mt-2 rounded bg-destructive/10 border border-destructive/20 px-3 py-2 font-mono text-destructive">
                              {m.sync_status.message}
                            </div>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  )}
                </>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Create mapping dialog */}
      <Dialog open={open} onOpenChange={v => { setOpen(v); if (!v) resetForm() }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>New K8s Mapping</DialogTitle>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>Name</Label>
              <Input placeholder="e.g. production-db" value={name} onChange={e => setName(e.target.value)} />
            </div>

            <div className="space-y-1.5">
              <Label>Rune Namespace</Label>
              <Select value={runeNS} onValueChange={setRuneNS}>
                <SelectTrigger>
                  <SelectValue placeholder="Select namespace…" />
                </SelectTrigger>
                <SelectContent>
                  {namespaces.map(ns => (
                    <SelectItem key={ns} value={ns}>{ns}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1.5">
              <Label>
                Keys{" "}
                <span className="text-muted-foreground font-normal">(optional — leave blank for all)</span>
              </Label>
              <Input
                placeholder="db/password, api/key"
                value={keys}
                onChange={e => setKeys(e.target.value)}
              />
              <p className="text-xs text-muted-foreground">Comma-separated list of Rune secret keys to include.</p>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>K8s Secret Name</Label>
                <Input placeholder="my-secret" value={targetSecret} onChange={e => setTargetSecret(e.target.value)} />
              </div>
              <div className="space-y-1.5">
                <Label>K8s Namespace</Label>
                <Input placeholder="default" value={targetK8sNS} onChange={e => setTargetK8sNS(e.target.value)} />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>Secret Type</Label>
                <Select value={secretType} onValueChange={setSecretType}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {SECRET_TYPES.map(t => (
                      <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>Refresh Interval</Label>
                <Select value={interval} onValueChange={setInterval_}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {INTERVALS.map(i => (
                      <SelectItem key={i.value} value={i.value}>{i.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => { setOpen(false); resetForm() }}>Cancel</Button>
            <Button onClick={handleCreate} disabled={!valid || creating}>
              {creating ? "Creating…" : "Create Mapping"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
