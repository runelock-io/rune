import { useState, useEffect, useCallback } from 'react'
import { Plus, Eye, RotateCcw, History, Trash2, Copy, Check, KeyRound, RefreshCw, Upload, Timer } from 'lucide-react'
import { api, type TokenRecord, type SecretVersion, type SecretMeta } from '@/lib/api'
import { toast } from 'sonner'
import { useTheme } from 'next-themes'
import CodeMirror from '@uiw/react-codemirror'
import { json } from '@codemirror/lang-json'
import { vscodeDark, vscodeLight } from '@uiw/codemirror-theme-vscode'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel,
  AlertDialogContent, AlertDialogDescription, AlertDialogFooter,
  AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import {
  Tooltip, TooltipContent, TooltipProvider, TooltipTrigger,
} from '@/components/ui/tooltip'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'

interface Props {
  token: string
  tokenInfo: TokenRecord | null
  sealed: boolean
}

interface ValueModal {
  key: string
  value: string
}

interface HistoryModal {
  key: string
  versions: SecretVersion[]
}

const TTL_PRESETS = ['1d', '7d', '14d', '30d', '90d']

function ttlBadge(expiresAt?: string): { label: string; cls: string } | null {
  if (!expiresAt) return null
  const ms = new Date(expiresAt).getTime() - Date.now()
  const rose  = 'bg-rose-100 text-rose-800 border-rose-200 dark:bg-rose-950 dark:text-rose-300 dark:border-rose-800'
  const amber = 'bg-amber-100 text-amber-800 border-amber-200 dark:bg-amber-950 dark:text-amber-300 dark:border-amber-800'
  const green = 'bg-emerald-100 text-emerald-800 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-300 dark:border-emerald-800'
  if (ms <= 0)          return { label: 'past due',                          cls: rose  }
  if (ms < 60_000)      return { label: `${Math.ceil(ms / 1000)}s left`,     cls: amber }
  if (ms < 3_600_000)   return { label: `${Math.floor(ms / 60_000)}m left`,  cls: amber }
  if (ms < 86_400_000)  return { label: `${Math.floor(ms / 3_600_000)}h left`, cls: amber }
  return                       { label: `${Math.floor(ms / 86_400_000)}d left`, cls: green }
}

function CopyButton({ text, className }: { text: string; className?: string }) {
  const [copied, setCopied] = useState(false)
  const copy = () => {
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  return (
    <Button variant="ghost" size="icon" className={cn("h-7 w-7 shrink-0", className)} onClick={copy}>
      {copied ? <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" /> : <Copy className="h-3.5 w-3.5" />}
    </Button>
  )
}

function VersionCopyButton({
  token, ns, secretKey, version,
}: { token: string; ns: string; secretKey: string; version: number }) {
  const [state, setState] = useState<'idle' | 'loading' | 'done'>('idle')

  const handleCopy = async () => {
    if (state === 'loading') return
    setState('loading')
    try {
      const { value } = await api.getSecretVersion(token, ns, secretKey, version)
      await navigator.clipboard.writeText(value)
      setState('done')
      setTimeout(() => setState('idle'), 2000)
    } catch (e) {
      toast.error('Failed to fetch value', { description: String(e) })
      setState('idle')
    }
  }

  return (
    <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={handleCopy} disabled={state === 'loading'}>
      {state === 'done'
        ? <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
        : state === 'loading'
        ? <RefreshCw className="h-3.5 w-3.5 animate-spin" />
        : <Copy className="h-3.5 w-3.5" />}
    </Button>
  )
}

function fmtDate(iso: string) {
  if (!iso) return '—'
  const d = new Date(iso)
  return isNaN(d.getTime()) ? iso : d.toLocaleString(undefined, {
    month: 'short', day: 'numeric', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

function parseEnvFile(text: string): Record<string, string> {
  const result: Record<string, string> = {}
  for (const line of text.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) continue
    const eq = trimmed.indexOf('=')
    if (eq === -1) continue
    const key = trimmed.slice(0, eq).trim()
    if (!key) continue
    let val = trimmed.slice(eq + 1).trim()
    if ((val.startsWith('"') && val.endsWith('"')) ||
        (val.startsWith("'") && val.endsWith("'"))) {
      val = val.slice(1, -1)
    }
    result[key] = val
  }
  return result
}

export function SecretsView({ token, tokenInfo, sealed }: Props) {
  const [namespaces, setNamespaces] = useState<string[]>([])
  const [ns, setNs] = useState('')
  const [metas, setMetas] = useState<SecretMeta[]>([])
  const [loading, setLoading] = useState(false)

  // Table mode state
  const [addOpen, setAddOpen] = useState(false)
  const [newKey, setNewKey] = useState('')
  const [newVal, setNewVal] = useState('')
  const [newTTL, setNewTTL] = useState('')
  const [putting, setPutting] = useState(false)

  // Set-TTL dialog
  const [ttlTarget, setTtlTarget] = useState<SecretMeta | null>(null)
  const [ttlInput, setTtlInput] = useState('')
  const [ttlSaving, setTtlSaving] = useState(false)
  const [valueModal, setValueModal] = useState<ValueModal | null>(null)
  const [historyModal, setHistoryModal] = useState<HistoryModal | null>(null)
  const [historyLoading, setHistoryLoading] = useState(false)

  // View mode
  const [mode, setMode] = useState<'table' | 'json'>('table')

  // JSON editor state
  const [jsonDraft, setJsonDraft] = useState('')
  const [jsonLoading, setJsonLoading] = useState(false)
  const [jsonSaving, setJsonSaving] = useState(false)
  const [jsonBase, setJsonBase] = useState<Record<string, string>>({})
  const [pendingDeletes, setPendingDeletes] = useState<string[]>([])
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)

  // Import state
  const [importOpen, setImportOpen] = useState(false)
  const [importData, setImportData] = useState<Record<string, string> | null>(null)
  const [importNs, setImportNs] = useState('')
  const [importing, setImporting] = useState(false)
  const [importProgress, setImportProgress] = useState({ done: 0, total: 0 })
  const [pasteContent, setPasteContent] = useState('')

  const { resolvedTheme } = useTheme()
  const isRoot = tokenInfo?.namespace === '*'

  function hasPermission(perm: string): boolean {
    if (tokenInfo?.read_only) return perm === 'read'
    if (isRoot) return true
    if (tokenInfo?.namespaces && tokenInfo.namespaces.length > 0) {
      const entry = tokenInfo.namespaces.find(np => np.namespace === ns)
      return entry?.permissions.includes(perm) ?? false
    }
    // Legacy single-namespace token: full access to its namespace
    return tokenInfo?.namespace === ns
  }

  const canWrite  = hasPermission('write')
  const canDelete = hasPermission('delete')
  const canRotate = hasPermission('rotate')

  const loadNamespaces = useCallback(async () => {
    if (isRoot) {
      const list = await api.listNamespaces(token).catch(() => ['default'])
      setNamespaces(list)
      if (!ns) setNs(list[0] ?? 'default')
    } else if (tokenInfo?.namespaces && tokenInfo.namespaces.length > 0) {
      const nsList = tokenInfo.namespaces.map(np => np.namespace)
      setNamespaces(nsList)
      setNs(prev => nsList.includes(prev) ? prev : nsList[0])
    } else {
      const n = tokenInfo?.namespace ?? 'default'
      setNamespaces([n])
      setNs(n)
    }
  }, [token, tokenInfo, isRoot, ns])

  const loadMetas = useCallback(async () => {
    if (!ns) return
    setLoading(true)
    const list = await api.listSecretMeta(token, ns).catch(() => [])
    setMetas(Array.isArray(list) ? list : [])
    setLoading(false)
  }, [token, ns])

  const loadJsonMode = useCallback(async () => {
    if (!ns) return
    setJsonLoading(true)
    try {
      const all = await api.getAllSecrets(token, ns)
      setJsonBase(all)
      setJsonDraft(JSON.stringify(all, null, 2))
    } catch (e) {
      toast.error('Failed to load secrets', { description: String(e) })
      setMode('table')
    }
    setJsonLoading(false)
  }, [token, ns])

  useEffect(() => { loadNamespaces() }, [loadNamespaces])
  useEffect(() => { if (ns) loadMetas() }, [loadMetas, ns])
  useEffect(() => { if (mode === 'json') loadJsonMode() }, [mode, loadJsonMode])

  async function handleGet(key: string) {
    const r = await api.getSecret(token, ns, key).catch(e => {
      toast.error('Failed to fetch value', { description: String(e) })
      return null
    })
    if (r) setValueModal({ key, value: r.value })
  }

  async function handleRotate(key: string) {
    const r = await api.rotateSecret(token, ns, key).catch(e => {
      toast.error('Rotate failed', { description: String(e) })
      return null
    })
    if (r) {
      toast.success(`Rotated "${key}"`)
      setValueModal({ key, value: r.value })
    }
  }

  async function handleHistory(key: string) {
    setHistoryLoading(true)
    setHistoryModal({ key, versions: [] })
    const versions = await api.listVersions(token, ns, key).catch(() => [])
    setHistoryModal({ key, versions: Array.isArray(versions) ? versions : [] })
    setHistoryLoading(false)
  }

  async function handleDelete(key: string) {
    await api.deleteSecret(token, ns, key)
      .then(() => { toast.success(`Deleted "${key}"`); loadMetas() })
      .catch(e => toast.error('Delete failed', { description: String(e) }))
  }

  async function handlePut() {
    if (!newKey.trim() || !newVal.trim()) return
    setPutting(true)
    try {
      await api.putSecret(token, ns, newKey.trim(), newVal.trim())
      if (newTTL.trim()) {
        await api.setSecretTTL(token, ns, newKey.trim(), newTTL.trim()).catch(() => {})
      }
      toast.success(`Saved "${newKey.trim()}"${newTTL ? ` · TTL ${newTTL}` : ''}`)
      setNewKey('')
      setNewVal('')
      setNewTTL('')
      setAddOpen(false)
      loadMetas()
    } catch (e) {
      toast.error('Save failed', { description: String(e) })
    }
    setPutting(false)
  }

  async function handleSetTTL() {
    if (!ttlTarget) return
    setTtlSaving(true)
    try {
      await api.setSecretTTL(token, ns, ttlTarget.key, ttlInput.trim())
      toast.success(ttlInput.trim()
        ? `TTL set to ${ttlInput.trim()} for "${ttlTarget.key}"`
        : `TTL cleared for "${ttlTarget.key}"`)
      setTtlTarget(null)
      setTtlInput('')
      loadMetas()
    } catch (e) {
      toast.error('Failed to set TTL', { description: String(e) })
    }
    setTtlSaving(false)
  }

  async function saveJsonDraft() {
    let parsed: Record<string, string>
    try {
      const raw = JSON.parse(jsonDraft)
      if (typeof raw !== 'object' || Array.isArray(raw) || raw === null) throw new Error()
      parsed = raw
    } catch {
      toast.error('Invalid JSON', { description: 'Must be a flat key → value object' })
      return
    }

    const toUpsert: [string, string][] = []
    const toDelete: string[] = []

    for (const [k, v] of Object.entries(parsed)) {
      if (!(k in jsonBase) || jsonBase[k] !== String(v)) {
        toUpsert.push([k, String(v)])
      }
    }
    for (const k of Object.keys(jsonBase)) {
      if (!(k in parsed)) toDelete.push(k)
    }

    if (toUpsert.length === 0 && toDelete.length === 0) {
      toast.success('No changes to save')
      return
    }

    // Apply adds / updates first
    setJsonSaving(true)
    const failed: string[] = []
    for (const [k, v] of toUpsert) {
      await api.putSecret(token, ns, k, v).catch(() => failed.push(k))
    }
    setJsonSaving(false)

    if (failed.length > 0) {
      toast.error(`${failed.length} save${failed.length !== 1 ? 's' : ''} failed`, { description: failed.join(', ') })
    }

    if (toDelete.length > 0) {
      setPendingDeletes(toDelete)
      setDeleteConfirmOpen(true)
    } else {
      const added = toUpsert.filter(([k]) => !(k in jsonBase)).length
      const updated = toUpsert.filter(([k]) => k in jsonBase).length
      const parts: string[] = []
      if (added) parts.push(`${added} added`)
      if (updated) parts.push(`${updated} updated`)
      toast.success('Saved — ' + parts.join(', '))
      loadMetas()
      loadJsonMode()
    }
  }

  async function handleConfirmDeletes() {
    setDeleteConfirmOpen(false)
    setJsonSaving(true)
    for (const k of pendingDeletes) {
      await api.deleteSecret(token, ns, k).catch(e =>
        toast.error(`Failed to delete "${k}"`, { description: String(e) })
      )
    }
    setJsonSaving(false)
    toast.success(`Deleted ${pendingDeletes.length} key${pendingDeletes.length !== 1 ? 's' : ''}`)
    setPendingDeletes([])
    loadMetas()
    loadJsonMode()
  }

  function handleCancelDeletes() {
    setDeleteConfirmOpen(false)
    setPendingDeletes([])
    toast.success('Saved (keys not deleted)')
    loadMetas()
    loadJsonMode()
  }

  function parseImportText(text: string): Record<string, string> | null {
    // Try JSON first
    try {
      const parsed = JSON.parse(text)
      if (typeof parsed === 'object' && !Array.isArray(parsed) && parsed !== null) {
        const data: Record<string, string> = {}
        for (const [k, v] of Object.entries(parsed)) data[k] = String(v)
        return data
      }
    } catch {}
    // Try .env
    const env = parseEnvFile(text)
    return Object.keys(env).length > 0 ? env : null
  }

  function handleImportFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = (ev) => {
      const text = ev.target?.result as string
      const name = file.name.toLowerCase()
      try {
        if (name.endsWith('.json')) {
          const parsed = JSON.parse(text)
          if (typeof parsed !== 'object' || Array.isArray(parsed) || parsed === null) {
            toast.error('Invalid format', { description: 'JSON must be a flat key-value object' })
            return
          }
          const data: Record<string, string> = {}
          for (const [k, v] of Object.entries(parsed)) data[k] = String(v)
          setImportData(data)
        } else {
          // .env, .txt, or any other format
          const env = parseEnvFile(text)
          if (Object.keys(env).length === 0) {
            toast.error('No secrets found', { description: 'Could not parse any key=value pairs from the file' })
            return
          }
          setImportData(env)
        }
      } catch {
        toast.error('Could not parse file')
      }
    }
    reader.readAsText(file)
  }

  function handlePasteChange(text: string) {
    setPasteContent(text)
    if (!text.trim()) { setImportData(null); return }
    const data = parseImportText(text.trim())
    if (data) setImportData(data)
    else setImportData(null)
  }

  async function handleImport() {
    if (!importData) return
    const entries = Object.entries(importData)
    setImporting(true)
    setImportProgress({ done: 0, total: entries.length })
    const failed: string[] = []
    for (const [key, value] of entries) {
      await api.putSecret(token, importNs, key, value).catch(() => failed.push(key))
      setImportProgress(p => ({ ...p, done: p.done + 1 }))
    }
    setImporting(false)
    if (failed.length === 0) {
      toast.success(`Imported ${entries.length} secret${entries.length !== 1 ? 's' : ''}`)
    } else {
      toast.error(`${failed.length} secret${failed.length !== 1 ? 's' : ''} failed to import`, {
        description: failed.join(', '),
      })
    }
    setImportOpen(false)
    setImportData(null)
    setPasteContent('')
    loadMetas()
  }

  function closeImportDialog() {
    setImportOpen(false)
    setImportData(null)
    setPasteContent('')
    setImportProgress({ done: 0, total: 0 })
  }

  if (sealed) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-center space-y-2">
          <p className="text-muted-foreground text-sm">Vault is sealed</p>
        </div>
      </div>
    )
  }

  return (
    <TooltipProvider delayDuration={400}>
      <div className="p-6 space-y-5">
        {/* Page header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold tracking-tight">Secrets</h1>
            <p className="text-sm text-muted-foreground mt-0.5">
              {loading
                ? 'Loading…'
                : `${metas.length} secret${metas.length !== 1 ? 's' : ''} in `}
              {!loading && <span className="font-mono text-foreground">{ns}</span>}
            </p>
          </div>
          <div className="flex items-center gap-2">
            {namespaces.length > 1 && (
              <Select value={ns} onValueChange={v => { setNs(v); setMode('table') }}>
                <SelectTrigger className="h-9 w-44 text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {namespaces.map(n => <SelectItem key={n} value={n} className="text-xs font-mono">{n}</SelectItem>)}
                </SelectContent>
              </Select>
            )}

            {/* View mode toggle */}
            <div className="flex items-center rounded-md border border-border overflow-hidden">
              <button
                className={cn(
                  'px-3 py-1.5 text-xs font-medium transition-colors',
                  mode === 'table'
                    ? 'bg-foreground text-background'
                    : 'bg-transparent text-muted-foreground hover:text-foreground'
                )}
                onClick={() => setMode('table')}
              >
                Table
              </button>
              <button
                className={cn(
                  'px-3 py-1.5 text-xs font-medium transition-colors border-l border-border',
                  mode === 'json'
                    ? 'bg-foreground text-background'
                    : 'bg-transparent text-muted-foreground hover:text-foreground'
                )}
                onClick={() => setMode('json')}
              >
                JSON
              </button>
            </div>

            <Button size="sm" variant="outline" onClick={() => { setImportNs(ns); setImportOpen(true) }} disabled={!canWrite}>
              <Upload className="h-4 w-4" />
              Import
            </Button>
            {mode === 'table' && (
              <Button size="sm" onClick={() => setAddOpen(true)} disabled={!canWrite}>
                <Plus className="h-4 w-4" />
                Add secret
              </Button>
            )}
          </div>
        </div>

        {/* TABLE MODE */}
        {mode === 'table' && (
          <div className="rounded-lg border border-border bg-card overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent border-b border-border">
                  <TableHead className="w-[35%] pl-4">Key</TableHead>
                  <TableHead className="w-[20%]">Namespace</TableHead>
                  <TableHead className="w-[20%]">TTL</TableHead>
                  <TableHead className="text-right pr-4">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground text-sm py-16">
                      <RefreshCw className="h-4 w-4 animate-spin mx-auto" />
                    </TableCell>
                  </TableRow>
                )}
                {!loading && metas.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="py-20">
                      <div className="flex flex-col items-center gap-3 text-center">
                        <div className="rounded-full bg-muted p-3">
                          <KeyRound className="h-5 w-5 text-muted-foreground" />
                        </div>
                        <div>
                          <p className="text-sm font-medium">No secrets yet</p>
                          <p className="text-xs text-muted-foreground mt-0.5">Add your first secret to get started</p>
                        </div>
                        <Button size="sm" variant="outline" onClick={() => setAddOpen(true)} disabled={!canWrite}>
                          <Plus className="h-4 w-4" />
                          Add secret
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )}
                {metas.map(meta => {
                  const badge = ttlBadge(meta.expires_at)
                  return (
                    <TableRow key={meta.key} className="group">
                      <TableCell className="pl-4">
                        <span className="font-mono text-sm">{meta.key}</span>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className="font-mono text-xs font-normal">
                          {ns}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {badge ? (
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <span className={cn(
                                'inline-flex items-center gap-1 text-[11px] font-medium px-2 py-0.5 rounded-full border cursor-default',
                                badge.cls,
                              )}>
                                <Timer className="h-3 w-3" />
                                {badge.label}
                              </span>
                            </TooltipTrigger>
                            <TooltipContent className="text-xs">
                              {meta.ttl && <div>Interval: {meta.ttl}</div>}
                              {meta.expires_at && <div>Next rotation: {fmtDate(meta.expires_at)}</div>}
                            </TooltipContent>
                          </Tooltip>
                        ) : (
                          <span className="text-xs text-muted-foreground">—</span>
                        )}
                      </TableCell>
                      <TableCell className="pr-4">
                        <div className="flex items-center justify-end gap-0.5">
                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8 opacity-60 group-hover:opacity-100 transition-opacity"
                                onClick={() => handleGet(meta.key)}
                              >
                                <Eye className="h-3.5 w-3.5" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>View value</TooltipContent>
                          </Tooltip>

                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8 opacity-60 group-hover:opacity-100 transition-opacity"
                                onClick={() => handleRotate(meta.key)}
                                disabled={!canRotate}
                              >
                                <RotateCcw className="h-3.5 w-3.5" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>{canRotate ? 'Rotate' : 'Requires rotate permission'}</TooltipContent>
                          </Tooltip>

                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8 opacity-60 group-hover:opacity-100 transition-opacity"
                                onClick={() => handleHistory(meta.key)}
                              >
                                <History className="h-3.5 w-3.5" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>Version history</TooltipContent>
                          </Tooltip>

                          <Tooltip>
                            <TooltipTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8 opacity-60 group-hover:opacity-100 transition-opacity"
                                onClick={() => { setTtlTarget(meta); setTtlInput(meta.ttl ?? '') }}
                                disabled={!canWrite}
                              >
                                <Timer className="h-3.5 w-3.5" />
                              </Button>
                            </TooltipTrigger>
                            <TooltipContent>{canWrite ? 'Set TTL' : 'Requires write permission'}</TooltipContent>
                          </Tooltip>

                          <Separator orientation="vertical" className="h-4 mx-1" />

                          <AlertDialog>
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <AlertDialogTrigger asChild>
                                  <Button
                                    variant="ghost"
                                    size="icon"
                                    className="h-8 w-8 opacity-60 group-hover:opacity-100 transition-opacity hover:text-destructive hover:bg-destructive/10"
                                    disabled={!canDelete}
                                  >
                                    <Trash2 className="h-3.5 w-3.5" />
                                  </Button>
                                </AlertDialogTrigger>
                              </TooltipTrigger>
                              <TooltipContent>{canDelete ? 'Delete' : 'Requires delete permission'}</TooltipContent>
                            </Tooltip>
                            <AlertDialogContent>
                              <AlertDialogHeader>
                                <AlertDialogTitle>Delete secret</AlertDialogTitle>
                                <AlertDialogDescription>
                                  Delete <code className="font-mono bg-muted px-1.5 py-0.5 rounded text-sm">{meta.key}</code>?
                                  {' '}All versions will be permanently removed.
                                </AlertDialogDescription>
                              </AlertDialogHeader>
                              <AlertDialogFooter>
                                <AlertDialogCancel>Cancel</AlertDialogCancel>
                                <AlertDialogAction
                                  onClick={() => handleDelete(meta.key)}
                                  className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                                >
                                  Delete
                                </AlertDialogAction>
                              </AlertDialogFooter>
                            </AlertDialogContent>
                          </AlertDialog>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        )}

        {/* JSON EDITOR MODE */}
        {mode === 'json' && (
          <div className="space-y-3">
            {jsonLoading ? (
              <div className="flex items-center justify-center rounded-lg border border-border bg-card h-64">
                <RefreshCw className="h-5 w-5 animate-spin text-muted-foreground" />
              </div>
            ) : (
              <>
                <div className="rounded-lg border border-border overflow-hidden bg-card">
                  <div className="flex items-center justify-between px-3 py-2 border-b border-border bg-muted/30">
                    <span className="text-xs text-muted-foreground font-mono">{ns}.json</span>
                    <div className="flex items-center gap-2">
                      {!canWrite && (
                        <Badge variant="outline" className="text-xs font-normal text-muted-foreground">read-only</Badge>
                      )}
                      <span className="text-xs text-muted-foreground">
                        {Object.keys(jsonBase).length} secret{Object.keys(jsonBase).length !== 1 ? 's' : ''}
                      </span>
                      <CopyButton text={jsonDraft} />
                    </div>
                  </div>
                  <CodeMirror
                    value={jsonDraft}
                    height="420px"
                    extensions={[json()]}
                    theme={resolvedTheme === 'dark' ? vscodeDark : vscodeLight}
                    onChange={value => setJsonDraft(value)}
                    editable={canWrite}
                    basicSetup={{
                      lineNumbers: true,
                      foldGutter: true,
                      highlightActiveLine: true,
                      bracketMatching: true,
                      autocompletion: canWrite,
                    }}
                  />
                </div>
                <div className="flex items-center justify-between">
                  <p className="text-xs text-muted-foreground">
                    {canWrite
                      ? 'Edit JSON directly. Save applies additions and updates; removed keys prompt for confirmation.'
                      : 'You have read-only access to this namespace.'}
                  </p>
                  <div className="flex gap-2">
                    <Button variant="outline" size="sm" onClick={loadJsonMode} disabled={jsonLoading}>
                      Refresh
                    </Button>
                    {(canWrite || canDelete) && (
                      <Button size="sm" onClick={saveJsonDraft} disabled={jsonSaving}>
                        {jsonSaving ? 'Saving…' : 'Save'}
                      </Button>
                    )}
                  </div>
                </div>
              </>
            )}
          </div>
        )}

        {/* Add secret dialog */}
        <Dialog open={addOpen} onOpenChange={open => { setAddOpen(open); if (!open) { setNewKey(''); setNewVal(''); setNewTTL('') } }}>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Add secret</DialogTitle>
              <DialogDescription>
                Store a new secret in <span className="font-mono font-medium">{ns}</span>.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-1">
              <div className="space-y-1.5">
                <Label htmlFor="secret-key" className="text-xs text-muted-foreground">Key</Label>
                <Input
                  id="secret-key"
                  placeholder="DATABASE_URL"
                  value={newKey}
                  onChange={e => setNewKey(e.target.value.replace(/\s/g, '_'))}
                  className={cn('font-mono text-sm', newKey && /\s/.test(newKey) && 'border-destructive focus-visible:ring-destructive')}
                  autoFocus
                />
                {newKey && /\s/.test(newKey) && (
                  <p className="text-[11px] text-destructive">Key cannot contain whitespace — spaces are converted to underscores.</p>
                )}
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="secret-val" className="text-xs text-muted-foreground">Value</Label>
                <Input
                  id="secret-val"
                  type="password"
                  placeholder="Secret value"
                  value={newVal}
                  onChange={e => setNewVal(e.target.value)}
                  onKeyDown={e => e.key === 'Enter' && handlePut()}
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground flex items-center gap-1.5">
                  <Timer className="h-3 w-3" />
                  Auto-rotation TTL <span className="text-muted-foreground/50">(optional)</span>
                </Label>
                <div className="flex flex-wrap gap-1.5 mb-1.5">
                  {TTL_PRESETS.map(p => (
                    <button
                      key={p}
                      type="button"
                      onClick={() => setNewTTL(newTTL === p ? '' : p)}
                      className={cn(
                        'text-xs px-2.5 py-1 rounded-full border font-medium transition-all',
                        newTTL === p
                          ? 'bg-foreground text-background border-foreground'
                          : 'border-border text-muted-foreground hover:border-muted-foreground/60 hover:text-foreground'
                      )}
                    >
                      {p}
                    </button>
                  ))}
                </div>
                <Input
                  placeholder="Custom (e.g. 12h, 45d)"
                  value={newTTL}
                  onChange={e => setNewTTL(e.target.value)}
                  className="font-mono text-sm h-8"
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setAddOpen(false)}>Cancel</Button>
              <Button onClick={handlePut} disabled={!newKey.trim() || /\s/.test(newKey) || !newVal.trim() || putting}>
                {putting ? 'Saving…' : 'Save secret'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Set TTL dialog */}
        <Dialog open={!!ttlTarget} onOpenChange={open => { if (!open) { setTtlTarget(null); setTtlInput('') } }}>
          <DialogContent className="sm:max-w-sm">
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <Timer className="h-4 w-4" />
                Auto-rotation TTL
              </DialogTitle>
              <DialogDescription>
                Secret will be automatically rotated after the TTL expires.
                {' '}Set empty to disable.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-3 py-1">
              <p className="text-xs font-mono text-muted-foreground">{ttlTarget?.key}</p>
              <div className="flex flex-wrap gap-1.5">
                {TTL_PRESETS.map(p => (
                  <button
                    key={p}
                    type="button"
                    onClick={() => setTtlInput(ttlInput === p ? '' : p)}
                    className={cn(
                      'text-xs px-2.5 py-1 rounded-full border font-medium transition-all',
                      ttlInput === p
                        ? 'bg-foreground text-background border-foreground'
                        : 'border-border text-muted-foreground hover:border-muted-foreground/60 hover:text-foreground'
                    )}
                  >
                    {p}
                  </button>
                ))}
              </div>
              <Input
                placeholder="Custom (e.g. 12h, 45d) or clear to disable"
                value={ttlInput}
                onChange={e => setTtlInput(e.target.value)}
                className="font-mono text-sm"
                autoFocus
              />
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => { setTtlTarget(null); setTtlInput('') }}>Cancel</Button>
              <Button onClick={handleSetTTL} disabled={ttlSaving}>
                {ttlSaving ? 'Saving…' : ttlInput.trim() ? 'Set TTL' : 'Clear TTL'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* View value modal */}
        <Dialog open={!!valueModal} onOpenChange={open => !open && setValueModal(null)}>
          <DialogContent className="sm:max-w-lg">
            <DialogHeader>
              <DialogTitle className="font-mono text-sm">{valueModal?.key}</DialogTitle>
              <DialogDescription>Current value</DialogDescription>
            </DialogHeader>
            <div className="flex items-start gap-2 rounded-lg bg-muted p-3">
              <code className="flex-1 font-mono text-xs leading-relaxed break-all select-all">
                {valueModal?.value}
              </code>
              <CopyButton text={valueModal?.value ?? ''} />
            </div>
          </DialogContent>
        </Dialog>

        {/* History modal */}
        <Dialog open={!!historyModal} onOpenChange={open => !open && setHistoryModal(null)}>
          <DialogContent className="sm:max-w-xl">
            <DialogHeader>
              <DialogTitle className="font-mono text-sm">{historyModal?.key}</DialogTitle>
              <DialogDescription>
                {historyLoading
                  ? 'Loading history…'
                  : `${historyModal?.versions.length ?? 0} version${historyModal?.versions.length !== 1 ? 's' : ''}`}
              </DialogDescription>
            </DialogHeader>

            {historyLoading ? (
              <div className="flex items-center justify-center py-10">
                <RefreshCw className="h-5 w-5 animate-spin text-muted-foreground" />
              </div>
            ) : (
              <div className="space-y-2 max-h-80 overflow-y-auto pr-1">
                {(historyModal?.versions ?? []).length === 0 ? (
                  <p className="text-sm text-muted-foreground text-center py-8">No history found</p>
                ) : (
                  [...(historyModal?.versions ?? [])].reverse().map(v => (
                    <div
                      key={v.version}
                      className="flex items-start gap-3 rounded-lg border border-border bg-card p-3"
                    >
                      <Badge variant="secondary" className="shrink-0 font-mono tabular-nums mt-0.5">
                        v{v.version}
                      </Badge>
                      <div className="flex-1 min-w-0 space-y-0.5">
                        <div className="text-xs text-muted-foreground">
                          <span className="text-foreground font-medium">Created</span>
                          {' '}{fmtDate(v.created_at)}
                        </div>
                        {v.rotated_at && (
                          <div className="text-xs text-muted-foreground">
                            <span className="text-foreground font-medium">Rotated</span>
                            {' '}{fmtDate(v.rotated_at)}
                          </div>
                        )}
                      </div>
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span>
                            <VersionCopyButton
                              token={token}
                              ns={ns}
                              secretKey={historyModal!.key}
                              version={v.version}
                            />
                          </span>
                        </TooltipTrigger>
                        <TooltipContent>Copy value</TooltipContent>
                      </Tooltip>
                    </div>
                  ))
                )}
              </div>
            )}
          </DialogContent>
        </Dialog>

        {/* Import dialog */}
        <Dialog open={importOpen} onOpenChange={open => { if (!open) closeImportDialog(); else setImportOpen(true) }}>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Import secrets</DialogTitle>
              <DialogDescription>
                Upload a <span className="font-mono">.env</span> or <span className="font-mono">.json</span> file, or paste content below.
              </DialogDescription>
            </DialogHeader>

            <div className="space-y-4 py-1">
              {/* File picker */}
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">File</Label>
                <input
                  type="file"
                  accept=".env,.json,.txt,text/plain,application/json"
                  onChange={handleImportFile}
                  className="block w-full text-sm text-muted-foreground file:mr-3 file:rounded-md file:border file:border-border file:bg-muted file:px-3 file:py-1.5 file:text-xs file:font-medium file:text-foreground hover:file:bg-accent cursor-pointer"
                />
              </div>

              {/* Paste area */}
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Or paste content</Label>
                <textarea
                  className="font-mono text-xs w-full h-24 rounded-md border bg-muted/40 p-2 resize-none focus:outline-none focus:ring-1 focus:ring-ring"
                  placeholder={"DATABASE_URL=postgres://...\nAPI_KEY=sk-..."}
                  value={pasteContent}
                  onChange={e => handlePasteChange(e.target.value)}
                  spellCheck={false}
                />
              </div>

              {/* Namespace */}
              <div className="space-y-1.5">
                <Label className="text-xs text-muted-foreground">Namespace</Label>
                {isRoot ? (
                  <Select value={importNs} onValueChange={setImportNs}>
                    <SelectTrigger className="h-9 text-xs font-mono">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {namespaces.map(n => (
                        <SelectItem key={n} value={n} className="text-xs font-mono">{n}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : (
                  <Badge variant="outline" className="font-mono text-xs font-normal">{ns}</Badge>
                )}
              </div>

              {/* Preview */}
              {importData && (
                <div className="rounded-lg border border-border bg-muted/40 overflow-hidden">
                  <div className="px-3 py-2 border-b border-border">
                    <p className="text-xs font-medium">
                      Found {Object.keys(importData).length} secret{Object.keys(importData).length !== 1 ? 's' : ''}
                    </p>
                  </div>
                  <div className="max-h-40 overflow-y-auto px-3 py-2 space-y-1">
                    {Object.keys(importData).map(k => (
                      <p key={k} className="text-xs font-mono text-muted-foreground truncate">{k}</p>
                    ))}
                  </div>
                </div>
              )}

              {/* Progress */}
              {importing && (
                <p className="text-xs text-muted-foreground">
                  Importing… {importProgress.done} / {importProgress.total}
                </p>
              )}
            </div>

            <DialogFooter>
              <Button variant="outline" onClick={closeImportDialog} disabled={importing}>Cancel</Button>
              <Button onClick={handleImport} disabled={!importData || importing}>
                {importing
                  ? `Importing… ${importProgress.done}/${importProgress.total}`
                  : importData
                  ? `Import ${Object.keys(importData).length} secret${Object.keys(importData).length !== 1 ? 's' : ''}`
                  : 'Import'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Delete confirmation for JSON save */}
        <AlertDialog open={deleteConfirmOpen} onOpenChange={open => { if (!open) handleCancelDeletes() }}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                Delete {pendingDeletes.length} key{pendingDeletes.length !== 1 ? 's' : ''}?
              </AlertDialogTitle>
              <AlertDialogDescription asChild>
                <div>
                  <p>The following keys were removed from the JSON and will be permanently deleted:</p>
                  <div className="mt-2 rounded-lg bg-muted p-2 max-h-32 overflow-y-auto space-y-0.5">
                    {pendingDeletes.map(k => (
                      <p key={k} className="font-mono text-xs text-muted-foreground">{k}</p>
                    ))}
                  </div>
                </div>
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel onClick={handleCancelDeletes}>Keep keys</AlertDialogCancel>
              <AlertDialogAction
                className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                onClick={handleConfirmDeletes}
              >
                Delete {pendingDeletes.length} key{pendingDeletes.length !== 1 ? 's' : ''}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </TooltipProvider>
  )
}
