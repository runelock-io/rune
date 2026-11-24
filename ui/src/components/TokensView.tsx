import { useState, useEffect, useCallback } from 'react'
import k8sIcon  from '@/assets/k8s.png'
import javaIcon from '@/assets/java.png'
import jsIcon   from '@/assets/js.png'
import goIcon   from '@/assets/go.png'
import { Plus, Trash2, Lock, Copy, Check, KeySquare, Bot, X, ShieldCheck, ChevronDown, ChevronUp } from 'lucide-react'
import { api, type TokenRecord, type NamespacePermission, ALL_PERMS } from '@/lib/api'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader,
  AlertDialogTitle, AlertDialogTrigger,
} from '@/components/ui/alert-dialog'

interface Props {
  token: string
  tokenInfo: TokenRecord | null
}

const PERM_LABEL: Record<string, string> = {
  read: 'Read', write: 'Write', delete: 'Delete', rotate: 'Rotate',
}

// Pastel chips: solid light background + dark saturated text for clear contrast
const PERM_CHIP: Record<string, string> = {
  read:   'bg-sky-100 text-sky-800 border-sky-200',
  write:  'bg-emerald-100 text-emerald-800 border-emerald-200',
  delete: 'bg-rose-100 text-rose-800 border-rose-200',
  rotate: 'bg-amber-100 text-amber-800 border-amber-200',
}

const PERM_INACTIVE = 'bg-transparent text-muted-foreground/60 border-border hover:border-muted-foreground/60 hover:text-muted-foreground'

// ─── Small helpers ────────────────────────────────────────────────────────────

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  return (
    <Button variant="outline" size="sm" onClick={copy} className="gap-1.5">
      {copied ? <Check className="h-3.5 w-3.5 text-emerald-400" /> : <Copy className="h-3.5 w-3.5" />}
      {copied ? 'Copied' : 'Copy'}
    </Button>
  )
}

function PermChip({ perm, size = 'sm' }: { perm: string; size?: 'xs' | 'sm' }) {
  return (
    <span className={cn(
      'inline-flex items-center font-medium rounded-full border',
      size === 'xs' ? 'text-[10px] px-1.5 py-px' : 'text-xs px-2 py-0.5',
      PERM_CHIP[perm],
    )}>
      {PERM_LABEL[perm]}
    </span>
  )
}

// ─── Access column cell ───────────────────────────────────────────────────────

function AccessCell({ token: t }: { token: TokenRecord }) {
  if (t.namespace === '*') {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full border bg-violet-100 text-violet-800 border-violet-200">
        <ShieldCheck className="h-3 w-3" />
        root
      </span>
    )
  }

  const rows: NamespacePermission[] = t.namespaces && t.namespaces.length > 0
    ? t.namespaces
    : [{ namespace: t.namespace || '—', permissions: t.read_only ? ['read'] : ['read', 'write', 'delete', 'rotate'] }]

  // Single namespace — show inline
  if (rows.length === 1) {
    return (
      <div className="flex flex-col gap-1.5">
        <span className="font-mono text-xs font-medium text-foreground">{rows[0].namespace}</span>
        <div className="flex flex-wrap gap-1">
          {rows[0].permissions.map(p => <PermChip key={p} perm={p} size="xs" />)}
        </div>
      </div>
    )
  }

  // Multi-namespace — namespace pills with tooltip showing perms
  return (
    <TooltipProvider>
      <div className="flex flex-col gap-1.5">
        {rows.map(r => (
          <Tooltip key={r.namespace}>
            <TooltipTrigger asChild>
              <div className="flex items-center gap-1.5 cursor-default w-fit">
                <span className="font-mono text-xs font-medium text-foreground">{r.namespace}</span>
                <div className="flex flex-wrap gap-1">
                  {r.permissions.map(p => <PermChip key={p} perm={p} size="xs" />)}
                </div>
              </div>
            </TooltipTrigger>
            <TooltipContent side="right" className="text-xs font-mono">{r.namespace}</TooltipContent>
          </Tooltip>
        ))}
      </div>
    </TooltipProvider>
  )
}

// ─── Namespace-permission row in dialog ───────────────────────────────────────

type NsEntry = { namespace: string; perms: Set<string> }

function makeEntry(ns: string): NsEntry {
  return { namespace: ns, perms: new Set<string>(['read']) }
}

interface NsRowProps {
  index: number
  ns: string
  perms: Set<string>
  namespaces: string[]
  usedNamespaces: string[]
  onChange: (index: number, ns: string, perms: Set<string>) => void
  onRemove: (index: number) => void
  canRemove: boolean
}

function NsPermRow({ index, ns, perms, namespaces, usedNamespaces, onChange, onRemove, canRemove }: NsRowProps) {
  const available = namespaces.filter(n => n === ns || !usedNamespaces.includes(n))
  const allSelected = ALL_PERMS.every(p => perms.has(p))

  function toggle(perm: string) {
    const next = new Set(perms)
    next.has(perm) ? next.delete(perm) : next.add(perm)
    onChange(index, ns, next)
  }

  function toggleAll() {
    onChange(index, ns, allSelected ? new Set() : new Set(ALL_PERMS))
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4 space-y-3">
      {/* Row header: namespace select + all/none + remove */}
      <div className="flex items-center gap-2">
        <Select value={ns} onValueChange={v => onChange(index, v, perms)}>
          <SelectTrigger className="font-mono text-xs h-8 flex-1">
            <SelectValue placeholder="Select namespace…" />
          </SelectTrigger>
          <SelectContent>
            {available.map(n => (
              <SelectItem key={n} value={n} className="font-mono text-xs">{n}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <button
          type="button"
          onClick={toggleAll}
          className="text-[11px] text-muted-foreground hover:text-foreground transition-colors shrink-0 underline-offset-2 hover:underline"
        >
          {allSelected ? 'None' : 'All'}
        </button>

        {canRemove && (
          <Button
            variant="ghost" size="icon"
            className="h-7 w-7 shrink-0 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
            onClick={() => onRemove(index)}
          >
            <X className="h-3.5 w-3.5" />
          </Button>
        )}
      </div>

      {/* Permission toggle pills */}
      <div className="flex flex-wrap gap-2">
        {ALL_PERMS.map(p => {
          const active = perms.has(p)
          return (
            <button
              key={p}
              type="button"
              onClick={() => toggle(p)}
              className={cn(
                'text-xs font-medium px-3 py-1 rounded-full border transition-all duration-150 select-none',
                active ? PERM_CHIP[p] : PERM_INACTIVE,
              )}
            >
              {PERM_LABEL[p]}
            </button>
          )
        })}
      </div>

      {perms.size === 0 && (
        <p className="text-[11px] text-destructive/80">Select at least one permission</p>
      )}
    </div>
  )
}

// ─── Platform definitions ─────────────────────────────────────────────────────

const PLATFORMS = [
  {
    id: 'kubernetes',
    name: 'Kubernetes',
    desc: 'Operator — syncs all namespaces',
    icon: k8sIcon,
    placeholder: 'e.g. k8s-operator-prod',
  },
  {
    id: 'java',
    name: 'Java',
    desc: 'Spring Boot / Java SDK',
    icon: javaIcon,
    placeholder: 'e.g. spring-prod',
  },
  {
    id: 'nodejs',
    name: 'Node.js',
    desc: 'Express / NestJS / Next.js',
    icon: jsIcon,
    placeholder: 'e.g. node-worker',
  },
  {
    id: 'go',
    name: 'Go',
    desc: 'Go SDK / microservices',
    icon: goIcon,
    placeholder: 'e.g. go-service-prod',
  },
]

function getSetupSnippet(platform: string, token: string): string {
  switch (platform) {
    case 'kubernetes':
      return `# Update the operator credentials secret
kubectl create secret generic rune-operator-credentials \\
  --namespace rune-system \\
  --from-literal=RUNE_SERVER="http://host.docker.internal:2710" \\
  --from-literal=RUNE_TOKEN="${token}" \\
  --dry-run=client -o yaml | kubectl apply -f -

# Restart the operator to pick up the new token
kubectl rollout restart deploy/rune-operator -n rune-system`

    case 'java':
      return `# application.properties
rune.system-token=${token}
rune.server-url=http://localhost:2710

# Or as an environment variable
export RUNE_SYSTEM_TOKEN="${token}"
export RUNE_SERVER_URL="http://localhost:2710"`

    case 'nodejs':
      return `# .env
RUNE_SYSTEM_TOKEN="${token}"
RUNE_SERVER_URL="http://localhost:2710"

# Usage in code
const { RuneClient } = require('@runelock/node-sdk')
const rune = new RuneClient({
  serverUrl: process.env.RUNE_SERVER_URL,
  systemToken: process.env.RUNE_SYSTEM_TOKEN,
})`

    case 'go':
      return `# Set environment variables
export RUNE_SYSTEM_TOKEN="${token}"
export RUNE_SERVER_URL="http://localhost:2710"

// Usage in code
client := rune.NewClient(rune.Config{
    ServerURL:   os.Getenv("RUNE_SERVER_URL"),
    SystemToken: os.Getenv("RUNE_SYSTEM_TOKEN"),
})`

    default:
      return token
  }
}

// ─── Main view ────────────────────────────────────────────────────────────────

export function TokensView({ token, tokenInfo }: Props) {
  const [tokens, setTokens] = useState<TokenRecord[]>([])
  const [namespaces, setNamespaces] = useState<string[]>([])

  // Regular token dialog
  const [addOpen, setAddOpen] = useState(false)
  const [newName, setNewName] = useState('')
  const [nsEntries, setNsEntries] = useState<NsEntry[]>([])
  const [creating, setCreating] = useState(false)

  // System token dialog
  const [sysAddOpen, setSysAddOpen] = useState(false)
  const [sysPlatform, setSysPlatform] = useState<string | null>(null)
  const [sysNewName, setSysNewName] = useState('')
  const [sysNewNs, setSysNewNs] = useState('')
  const [sysCreating, setSysCreating] = useState(false)

  // Reveal modal — shared
  const [revealedToken, setRevealedToken] = useState<string | null>(null)
  const [revealedPlatform, setRevealedPlatform] = useState<string | null>(null)
  const [revealSnippetOpen, setRevealSnippetOpen] = useState(false)

  const isRoot = tokenInfo?.namespace === '*'

  const loadNamespaces = useCallback(async () => {
    const list = await api.listNamespaces(token).catch(() => [])
    const ns = Array.isArray(list) ? list : []
    setNamespaces(ns)
    setNsEntries([makeEntry(ns[0] ?? '')])
    setSysNewNs(prev => prev || ns[0] || '')
  }, [token])

  useEffect(() => { if (isRoot) loadNamespaces() }, [isRoot, loadNamespaces])

  const load = useCallback(async () => {
    const list = await api.listTokens(token).catch(() => [])
    setTokens(Array.isArray(list) ? list : [])
  }, [token])

  useEffect(() => { if (isRoot) load() }, [isRoot, load])

  if (!isRoot) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-center space-y-2">
          <Lock className="h-8 w-8 text-muted-foreground/40 mx-auto" />
          <p className="text-sm text-muted-foreground">Root access required</p>
        </div>
      </div>
    )
  }

  function resetAddDialog() {
    setNewName('')
    setNsEntries([makeEntry(namespaces[0] ?? '')])
  }

  function updateEntry(index: number, ns: string, perms: Set<string>) {
    setNsEntries(prev => prev.map((e, i) => i === index ? { namespace: ns, perms } : e))
  }

  function removeEntry(index: number) {
    setNsEntries(prev => prev.filter((_, i) => i !== index))
  }

  function addEntry() {
    const used = nsEntries.map(e => e.namespace)
    const next = namespaces.find(n => !used.includes(n)) ?? ''
    setNsEntries(prev => [...prev, makeEntry(next)])
  }

  const isCreateValid =
    newName.trim() !== '' &&
    nsEntries.length > 0 &&
    nsEntries.every(e => e.namespace !== '' && e.perms.size > 0)

  async function handleCreate() {
    if (!isCreateValid) return
    setCreating(true)
    const nps: NamespacePermission[] = nsEntries.map(e => ({
      namespace: e.namespace,
      permissions: Array.from(e.perms),
    }))
    await api.createToken(token, newName.trim(), nps)
      .then(r => {
        setRevealedToken(r.token)
        setRevealedPlatform(null)
        resetAddDialog()
        setAddOpen(false)
        load()
      })
      .catch(e => toast.error('Create failed', { description: String(e) }))
    setCreating(false)
  }

  async function handleCreateSystem() {
    const name = sysNewName.trim()
    const ns = sysPlatform === 'kubernetes' ? '*' : sysNewNs.trim()
    if (!name || !ns) return
    setSysCreating(true)
    await api.createSystemToken(token, name, ns, sysPlatform ?? '')
      .then(r => {
        setRevealedToken(r.token)
        setRevealedPlatform(sysPlatform)
        setRevealSnippetOpen(false)
        setSysNewName('')
        setSysPlatform(null)
        setSysNewNs(namespaces[0] ?? '')
        setSysAddOpen(false)
        load()
      })
      .catch(e => toast.error('Create failed', { description: String(e) }))
    setSysCreating(false)
  }

  async function handleRevoke(id: string) {
    await api.revokeToken(token, id)
      .then(() => { toast.success('Token revoked'); load() })
      .catch(e => toast.error('Revoke failed', { description: String(e) }))
  }

  function fmt(date: string) {
    return new Date(date).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
  }

  const usedNamespaces = nsEntries.map(e => e.namespace)
  const canAddMore = nsEntries.length < namespaces.length

  return (
    <div className="p-6 space-y-6">
      {/* Page header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Tokens</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            {tokens.length} token{tokens.length !== 1 ? 's' : ''}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button size="sm" variant="outline" onClick={() => setSysAddOpen(true)}>
            <Bot className="h-4 w-4" />
            System Token
          </Button>
          <Button size="sm" onClick={() => setAddOpen(true)}>
            <Plus className="h-4 w-4" />
            Add Token
          </Button>
        </div>
      </div>

      {/* Table */}
      <div className="rounded-lg border border-border bg-card overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="pl-4">Name</TableHead>
              <TableHead>Access</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Platform</TableHead>
              <TableHead>Created</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-14 text-right pr-4">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {tokens.length === 0 && (
              <TableRow>
                <TableCell colSpan={7} className="py-16">
                  <div className="flex flex-col items-center gap-3 text-center">
                    <div className="rounded-full bg-muted p-3">
                      <KeySquare className="h-5 w-5 text-muted-foreground" />
                    </div>
                    <div>
                      <p className="text-sm font-medium">No tokens yet</p>
                      <p className="text-xs text-muted-foreground mt-0.5">Create tokens to grant scoped namespace access</p>
                    </div>
                    <Button size="sm" variant="outline" onClick={() => setAddOpen(true)}>
                      <Plus className="h-4 w-4" />
                      Add Token
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            )}
            {tokens.map(t => (
              <TableRow key={t.id} className="align-top">
                <TableCell className="pl-4 pt-3.5 font-medium text-sm">{t.name}</TableCell>
                <TableCell className="pt-3"><AccessCell token={t} /></TableCell>
                <TableCell className="pt-3.5">
                  {t.system ? (
                    <span className="inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full border bg-sky-100 text-sky-800 border-sky-200">
                      <Bot className="h-3 w-3" />
                      system
                    </span>
                  ) : (
                    <span className="inline-flex items-center text-xs font-medium px-2 py-0.5 rounded-full border border-border text-muted-foreground">
                      regular
                    </span>
                  )}
                </TableCell>
                <TableCell className="pt-3">
                  {t.platform ? (() => {
                    const pl = PLATFORMS.find(p => p.id === t.platform)
                    return pl ? (
                      <span className="inline-flex items-center gap-1.5">
                        <img src={pl.icon} alt={pl.name} className="h-4 w-4 object-contain" />
                        <span className="text-xs text-muted-foreground">{pl.name}</span>
                      </span>
                    ) : (
                      <span className="text-xs text-muted-foreground">{t.platform}</span>
                    )
                  })() : (
                    <span className="text-xs text-muted-foreground/40">—</span>
                  )}
                </TableCell>
                <TableCell className="pt-3.5 text-sm text-muted-foreground">{fmt(t.created_at)}</TableCell>
                <TableCell className="pt-3.5">
                  <Badge variant={t.revoked ? 'destructive' : 'secondary'} className="text-xs">
                    {t.revoked ? 'revoked' : 'active'}
                  </Badge>
                </TableCell>
                <TableCell className="pt-2.5 text-right pr-4">
                  {!t.revoked && t.id !== 'root' && (
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground hover:text-destructive hover:bg-destructive/10">
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Revoke token</AlertDialogTitle>
                          <AlertDialogDescription>
                            Revoke <span className="font-semibold">{t.name}</span>? It will immediately lose all access and cannot be undone.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction
                            onClick={() => handleRevoke(t.id)}
                            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                          >
                            Revoke
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* ── Add token dialog ── */}
      <Dialog open={addOpen} onOpenChange={open => { setAddOpen(open); if (!open) resetAddDialog() }}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>Create Token</DialogTitle>
            <DialogDescription>
              Name the token and choose which namespaces it can access. Click a permission to toggle it.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-5 py-1">
            {/* Token name */}
            <div className="space-y-1.5">
              <Label htmlFor="token-name" className="text-sm">Token name</Label>
              <Input
                id="token-name"
                placeholder="e.g. ci-deploy, staging-reader…"
                value={newName}
                onChange={e => setNewName(e.target.value)}
                autoFocus
              />
            </div>

            {/* Namespace rows */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <Label className="text-sm">Namespace access</Label>
                <span className="text-xs text-muted-foreground">
                  {nsEntries.length} namespace{nsEntries.length !== 1 ? 's' : ''}
                </span>
              </div>

              <div className="space-y-2">
                {nsEntries.map((entry, i) => (
                  <NsPermRow
                    key={i}
                    index={i}
                    ns={entry.namespace}
                    perms={entry.perms}
                    namespaces={namespaces}
                    usedNamespaces={usedNamespaces.filter((_, idx) => idx !== i)}
                    onChange={updateEntry}
                    onRemove={removeEntry}
                    canRemove={nsEntries.length > 1}
                  />
                ))}
              </div>

              {canAddMore && (
                <button
                  type="button"
                  onClick={addEntry}
                  className="w-full rounded-lg border border-dashed border-border py-2.5 text-xs text-muted-foreground hover:border-muted-foreground/60 hover:text-foreground transition-colors flex items-center justify-center gap-1.5"
                >
                  <Plus className="h-3.5 w-3.5" />
                  Add another namespace
                </button>
              )}
            </div>

            {/* Live access summary */}
            {isCreateValid && (
              <div className="rounded-lg bg-muted/50 border border-border px-3.5 py-3 space-y-2">
                <p className="text-xs text-muted-foreground font-medium">Access summary</p>
                {nsEntries.map(e => (
                  <div key={e.namespace} className="flex items-center gap-2 flex-wrap">
                    <span className="font-mono text-xs text-foreground">{e.namespace}</span>
                    <span className="text-muted-foreground/40">→</span>
                    {Array.from(e.perms).map(p => <PermChip key={p} perm={p} size="xs" />)}
                  </div>
                ))}
              </div>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setAddOpen(false)}>Cancel</Button>
            <Button onClick={handleCreate} disabled={!isCreateValid || creating}>
              {creating ? 'Creating…' : 'Create Token'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── Add system token dialog ── */}
      <Dialog open={sysAddOpen} onOpenChange={open => {
        setSysAddOpen(open)
        if (!open) { setSysNewName(''); setSysPlatform(null); setSysNewNs(namespaces[0] ?? '') }
      }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Bot className="h-4 w-4 text-sky-400" />
              Create System Token
            </DialogTitle>
            <DialogDescription>
              Read-only token scoped to a platform. Choose your integration to get a tailored setup guide.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-5 py-2">
            {/* Platform picker */}
            <div className="space-y-2">
              <Label>Platform</Label>
              <div className="grid grid-cols-2 gap-2">
                {PLATFORMS.map(p => (
                  <button
                    key={p.id}
                    type="button"
                    onClick={() => setSysPlatform(p.id)}
                    className={cn(
                      'flex items-center gap-3 rounded-lg border p-3 text-left transition-all',
                      sysPlatform === p.id
                        ? 'border-primary bg-primary/5 ring-1 ring-primary'
                        : 'border-border hover:border-muted-foreground/40 hover:bg-muted/40'
                    )}
                  >
                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-muted/60">
                      <img src={p.icon} alt={p.name} className="h-5 w-5 object-contain" />
                    </div>
                    <div>
                      <p className="text-sm font-medium leading-none">{p.name}</p>
                      <p className="mt-1 text-xs text-muted-foreground">{p.desc}</p>
                    </div>
                  </button>
                ))}
              </div>
            </div>

            {sysPlatform && (
              <>
                <div className="space-y-1.5">
                  <Label>Token name</Label>
                  <Input
                    placeholder={PLATFORMS.find(p => p.id === sysPlatform)?.placeholder ?? 'e.g. prod-reader'}
                    value={sysNewName}
                    onChange={e => setSysNewName(e.target.value)}
                    autoFocus
                  />
                </div>

                {sysPlatform !== 'kubernetes' ? (
                  <div className="space-y-1.5">
                    <Label>Namespace</Label>
                    <Select value={sysNewNs} onValueChange={setSysNewNs}>
                      <SelectTrigger className="font-mono text-sm">
                        <SelectValue placeholder="Select namespace…" />
                      </SelectTrigger>
                      <SelectContent>
                        {namespaces.map(ns => (
                          <SelectItem key={ns} value={ns} className="font-mono text-xs">{ns}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                ) : (
                  <div className="rounded-lg border border-blue-500/20 bg-blue-500/5 px-3.5 py-3 space-y-1">
                    <p className="text-xs font-medium text-blue-400">Wildcard namespace · reads all namespaces</p>
                    <p className="text-xs text-muted-foreground">
                      The Kubernetes operator syncs secrets across namespaces and needs vault-wide read access.
                    </p>
                  </div>
                )}

                <div className="rounded-lg bg-sky-500/10 border border-sky-500/20 px-3.5 py-3 space-y-1">
                  <p className="text-xs font-medium text-sky-300">Read-only · system token</p>
                  <p className="text-xs text-muted-foreground">
                    {sysPlatform === 'kubernetes'
                      ? 'This token is used by the rune-operator to sync secrets into K8s. It cannot write or delete.'
                      : 'This token can only read secrets. It uses dual encryption and works even when the vault is sealed.'}
                  </p>
                </div>
              </>
            )}
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setSysAddOpen(false)}>Cancel</Button>
            <Button
              onClick={handleCreateSystem}
              disabled={!sysPlatform || !sysNewName.trim() || (sysPlatform !== 'kubernetes' && !sysNewNs.trim()) || sysCreating}
            >
              {sysCreating ? 'Creating…' : 'Create Token'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* ── Token reveal modal ── */}
      <Dialog open={!!revealedToken} onOpenChange={open => !open && setRevealedToken(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {revealedPlatform
                ? `${PLATFORMS.find(p => p.id === revealedPlatform)?.name ?? ''} token created`
                : 'Token created'}
            </DialogTitle>
            <DialogDescription>
              {revealedPlatform
                ? 'Copy the token — it will not be shown again.'
                : 'Save this token somewhere safe — it will not be shown again.'}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            {/* Token value */}
            <div className="space-y-2">
              <div className="rounded-lg bg-muted border border-border p-3">
                <code className="block font-mono text-xs break-all leading-relaxed select-all">
                  {revealedToken}
                </code>
              </div>
              <CopyButton text={revealedToken ?? ''} />
            </div>

            {/* Platform-specific setup snippet */}
            {revealedPlatform && revealedToken && (
              <div className="space-y-2">
                <button
                  type="button"
                  className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
                  onClick={() => setRevealSnippetOpen(v => !v)}
                >
                  {revealSnippetOpen ? <ChevronUp className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
                  Setup instructions
                </button>
                {revealSnippetOpen && (
                  <div className="rounded-lg border border-border bg-[#1e1e2e] p-4 space-y-1">
                    <div className="flex items-center gap-1.5 mb-2">
                      {(() => { const pl = PLATFORMS.find(p => p.id === revealedPlatform); return pl ? <img src={pl.icon} alt={pl.name} className="h-3.5 w-3.5 object-contain" /> : null })()}
                      <p className="text-[10px] text-muted-foreground font-mono">
                        {PLATFORMS.find(p => p.id === revealedPlatform)?.name}
                      </p>
                    </div>
                    <pre className="font-mono text-xs text-slate-200 whitespace-pre-wrap break-all leading-relaxed">
                      {getSetupSnippet(revealedPlatform, revealedToken)}
                    </pre>
                  </div>
                )}
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
