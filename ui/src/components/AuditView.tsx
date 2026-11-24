import { useState, useEffect, useCallback } from 'react'
import { ScrollText, RefreshCw } from 'lucide-react'
import { api, type AuditEvent } from '@/lib/api'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'

interface Props {
  token: string
}

// ─── Event category config ────────────────────────────────────────────────────

type Category = 'all' | 'secrets' | 'tokens' | 'namespaces' | 'vault' | 'k8s' | 'users'

const CATEGORIES: { id: Category; label: string }[] = [
  { id: 'all',        label: 'All' },
  { id: 'secrets',    label: 'Secrets' },
  { id: 'tokens',     label: 'Tokens' },
  { id: 'users',      label: 'Users' },
  { id: 'namespaces', label: 'Namespaces' },
  { id: 'vault',      label: 'Vault' },
  { id: 'k8s',        label: 'Kubernetes' },
]

const TYPE_CATEGORY: Record<string, Category> = {
  'secret.create':       'secrets',
  'secret.read':         'secrets',
  'secret.delete':       'secrets',
  'secret.rotate':       'secrets',
  'secret.list':         'secrets',
  'secret.ttl_set':      'secrets',
  'token.create':        'tokens',
  'token.create_system': 'tokens',
  'token.revoke':        'tokens',
  'namespace.create':    'namespaces',
  'namespace.delete':    'namespaces',
  'vault.seal':          'vault',
  'vault.unseal':        'vault',
  'k8s.mapping_create':  'k8s',
  'k8s.mapping_delete':  'k8s',
  'k8s.sync':            'k8s',
  'user.create':         'users',
  'user.delete':         'users',
  'user.login':          'users',
}

const TYPE_LABEL: Record<string, string> = {
  'secret.create':       'Create secret',
  'secret.read':         'Read secret',
  'secret.delete':       'Delete secret',
  'secret.rotate':       'Rotate secret',
  'secret.list':         'List secrets',
  'secret.ttl_set':      'Set TTL',
  'token.create':        'Create token',
  'token.create_system': 'Create system token',
  'token.revoke':        'Revoke token',
  'namespace.create':    'Create namespace',
  'namespace.delete':    'Delete namespace',
  'vault.seal':          'Seal vault',
  'vault.unseal':        'Unseal vault',
  'k8s.mapping_create':  'Create mapping',
  'k8s.mapping_delete':  'Delete mapping',
  'k8s.sync':            'Operator sync',
  'user.create':         'Create user',
  'user.delete':         'Delete user',
  'user.login':          'User login',
}

// Colour by action intent, not category
const TYPE_CHIP: Record<string, string> = {
  // Destructive — rose
  'secret.delete':       'bg-rose-100 text-rose-800 border-rose-200 dark:bg-rose-950 dark:text-rose-300 dark:border-rose-800',
  'token.revoke':        'bg-rose-100 text-rose-800 border-rose-200 dark:bg-rose-950 dark:text-rose-300 dark:border-rose-800',
  'namespace.delete':    'bg-rose-100 text-rose-800 border-rose-200 dark:bg-rose-950 dark:text-rose-300 dark:border-rose-800',
  'k8s.mapping_delete':  'bg-rose-100 text-rose-800 border-rose-200 dark:bg-rose-950 dark:text-rose-300 dark:border-rose-800',
  'vault.seal':          'bg-rose-100 text-rose-800 border-rose-200 dark:bg-rose-950 dark:text-rose-300 dark:border-rose-800',
  // Create / restore — emerald
  'secret.create':       'bg-emerald-100 text-emerald-800 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-300 dark:border-emerald-800',
  'token.create':        'bg-emerald-100 text-emerald-800 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-300 dark:border-emerald-800',
  'token.create_system': 'bg-emerald-100 text-emerald-800 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-300 dark:border-emerald-800',
  'namespace.create':    'bg-emerald-100 text-emerald-800 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-300 dark:border-emerald-800',
  'k8s.mapping_create':  'bg-emerald-100 text-emerald-800 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-300 dark:border-emerald-800',
  'vault.unseal':        'bg-emerald-100 text-emerald-800 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-300 dark:border-emerald-800',
  // Read / list — sky
  'secret.read':         'bg-sky-100 text-sky-800 border-sky-200 dark:bg-sky-950 dark:text-sky-300 dark:border-sky-800',
  'secret.list':         'bg-sky-100 text-sky-800 border-sky-200 dark:bg-sky-950 dark:text-sky-300 dark:border-sky-800',
  // Rotate / TTL — amber
  'secret.rotate':       'bg-amber-100 text-amber-800 border-amber-200 dark:bg-amber-950 dark:text-amber-300 dark:border-amber-800',
  'secret.ttl_set':      'bg-amber-100 text-amber-800 border-amber-200 dark:bg-amber-950 dark:text-amber-300 dark:border-amber-800',
  // Sync — indigo
  'k8s.sync':            'bg-indigo-100 text-indigo-800 border-indigo-200 dark:bg-indigo-950 dark:text-indigo-300 dark:border-indigo-800',
  // User — emerald create, rose delete, sky login
  'user.create':         'bg-emerald-100 text-emerald-800 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-300 dark:border-emerald-800',
  'user.delete':         'bg-rose-100 text-rose-800 border-rose-200 dark:bg-rose-950 dark:text-rose-300 dark:border-rose-800',
  'user.login':          'bg-sky-100 text-sky-800 border-sky-200 dark:bg-sky-950 dark:text-sky-300 dark:border-sky-800',
}

// ─── Relative time helper ─────────────────────────────────────────────────────

function relativeTime(ts: string): string {
  const diff = Math.floor((Date.now() - new Date(ts).getTime()) / 1000)
  if (diff < 60)  return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

function fullTime(ts: string): string {
  return new Date(ts).toLocaleString(undefined, {
    month: 'short', day: 'numeric', year: 'numeric',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

// ─── Action chip ─────────────────────────────────────────────────────────────

const FALLBACK_CHIP = 'bg-muted text-muted-foreground border-border'

function ActionChip({ type }: { type: string }) {
  const label = TYPE_LABEL[type] ?? type
  return (
    <span className={cn(
      'inline-flex items-center text-[11px] font-medium px-2 py-0.5 rounded-full border',
      TYPE_CHIP[type] ?? FALLBACK_CHIP,
    )}>
      {label}
    </span>
  )
}

// ─── Main view ────────────────────────────────────────────────────────────────

export function AuditView({ token }: Props) {
  const [events, setEvents] = useState<AuditEvent[]>([])
  const [category, setCategory] = useState<Category>('all')
  const [loading, setLoading] = useState(false)
  const [tick, setTick] = useState(0)

  const load = useCallback(async () => {
    setLoading(true)
    const list = await api.listAuditEvents(token, 500).catch(() => [])
    setEvents(Array.isArray(list) ? list : [])
    setLoading(false)
  }, [token])

  useEffect(() => { load() }, [load, tick])

  useEffect(() => {
    const id = setInterval(() => setTick(t => t + 1), 15000)
    return () => clearInterval(id)
  }, [])

  const filtered = category === 'all'
    ? events
    : events.filter(e => TYPE_CATEGORY[e.type] === category)

  return (
    <div className="p-6 space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Audit Log</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            {filtered.length} event{filtered.length !== 1 ? 's' : ''}
            {category !== 'all' ? ` · ${CATEGORIES.find(c => c.id === category)?.label}` : ''}
          </p>
        </div>
        <Button variant="ghost" size="icon" className="h-8 w-8" onClick={load} disabled={loading}>
          <RefreshCw className={cn('h-4 w-4', loading && 'animate-spin')} />
        </Button>
      </div>

      {/* Category filters */}
      <div className="flex flex-wrap gap-1.5">
        {CATEGORIES.map(c => (
          <button
            key={c.id}
            onClick={() => setCategory(c.id)}
            className={cn(
              'text-xs px-3 py-1 rounded-full border font-medium transition-all',
              category === c.id
                ? 'bg-foreground text-background border-foreground'
                : 'border-border text-muted-foreground hover:border-muted-foreground/60 hover:text-foreground'
            )}
          >
            {c.label}
          </button>
        ))}
      </div>

      {/* Table */}
      <div className="rounded-lg border border-border bg-card overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="pl-4 w-28">Time</TableHead>
              <TableHead className="w-32">Actor</TableHead>
              <TableHead>Action</TableHead>
              <TableHead>Resource</TableHead>
              <TableHead className="w-24">Status</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {filtered.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-16">
                  <div className="flex flex-col items-center gap-3 text-center">
                    <div className="rounded-full bg-muted p-3">
                      <ScrollText className="h-5 w-5 text-muted-foreground" />
                    </div>
                    <div>
                      <p className="text-sm font-medium">No events yet</p>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        Vault actions will appear here in real-time
                      </p>
                    </div>
                  </div>
                </TableCell>
              </TableRow>
            )}
            {filtered.map(e => (
              <TableRow key={e.id} className="align-middle">
                <TableCell className="pl-4 py-2.5">
                  <TooltipProvider>
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="text-xs text-muted-foreground tabular-nums cursor-default">
                          {relativeTime(e.timestamp)}
                        </span>
                      </TooltipTrigger>
                      <TooltipContent side="right" className="text-xs font-mono">
                        {fullTime(e.timestamp)}
                      </TooltipContent>
                    </Tooltip>
                  </TooltipProvider>
                </TableCell>
                <TableCell className="py-2.5">
                  <span className="text-xs font-medium font-mono truncate max-w-[120px] block">
                    {e.actor || '—'}
                  </span>
                </TableCell>
                <TableCell className="py-2.5">
                  <ActionChip type={e.type} />
                </TableCell>
                <TableCell className="py-2.5">
                  <span className="font-mono text-xs text-muted-foreground">{e.resource}</span>
                  {e.message && (
                    <p className="text-[11px] text-destructive/70 mt-0.5 truncate max-w-xs">{e.message}</p>
                  )}
                </TableCell>
                <TableCell className="py-2.5">
                  <Badge
                    className={cn(
                      'text-[11px] font-medium border',
                      e.status === 'success'
                        ? 'bg-emerald-100 text-emerald-800 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-300 dark:border-emerald-800'
                        : 'bg-rose-100 text-rose-800 border-rose-200 dark:bg-rose-950 dark:text-rose-300 dark:border-rose-800'
                    )}
                    variant="outline"
                  >
                    {e.status}
                  </Badge>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}
