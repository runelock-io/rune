import { useState, useEffect, useCallback } from 'react'
import { Plus, Trash2, X, Users } from 'lucide-react'
import { api, type UserAccount, type TokenRecord, type NamespacePermission, ALL_PERMS } from '@/lib/api'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
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

const PERM_CHIP: Record<string, string> = {
  read:   'bg-sky-100 text-sky-800 border-sky-200 dark:bg-sky-950 dark:text-sky-300 dark:border-sky-800',
  write:  'bg-emerald-100 text-emerald-800 border-emerald-200 dark:bg-emerald-950 dark:text-emerald-300 dark:border-emerald-800',
  delete: 'bg-rose-100 text-rose-800 border-rose-200 dark:bg-rose-950 dark:text-rose-300 dark:border-rose-800',
  rotate: 'bg-amber-100 text-amber-800 border-amber-200 dark:bg-amber-950 dark:text-amber-300 dark:border-amber-800',
}

const PERM_INACTIVE = 'bg-transparent text-muted-foreground/60 border-border hover:border-muted-foreground/60 hover:text-muted-foreground'

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

  return (
    <div className="rounded-lg border border-border bg-card p-4 space-y-3">
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
          onClick={() => onChange(index, ns, allSelected ? new Set() : new Set(ALL_PERMS))}
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

function AccessCell({ namespaces }: { namespaces?: NamespacePermission[] }) {
  if (!namespaces || namespaces.length === 0) return <span className="text-xs text-muted-foreground">—</span>
  return (
    <div className="flex flex-col gap-1.5">
      {namespaces.map(np => (
        <div key={np.namespace} className="flex flex-col gap-0.5">
          <span className="font-mono text-xs font-medium">{np.namespace}</span>
          <div className="flex flex-wrap gap-1">
            {np.permissions.map(p => (
              <span key={p} className={cn('text-[10px] px-1.5 py-px rounded-full border font-medium', PERM_CHIP[p])}>
                {PERM_LABEL[p]}
              </span>
            ))}
          </div>
        </div>
      ))}
    </div>
  )
}

export function UsersView({ token, tokenInfo }: Props) {
  const [users, setUsers] = useState<UserAccount[]>([])
  const [loading, setLoading] = useState(true)
  const [namespaces, setNamespaces] = useState<string[]>([])

  // Create dialog
  const [open, setOpen] = useState(false)
  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [nsEntries, setNsEntries] = useState<NsEntry[]>([])
  const [creating, setCreating] = useState(false)

  const isRoot = tokenInfo?.namespace === '*'

  const loadUsers = useCallback(async () => {
    try {
      const list = await api.listUsers(token)
      setUsers(list)
    } catch (e) {
      toast.error('Failed to load users', { description: String(e) })
    } finally {
      setLoading(false)
    }
  }, [token])

  useEffect(() => {
    loadUsers()
    api.listNamespaces(token).then(setNamespaces).catch(() => {})
  }, [token, loadUsers])

  function resetDialog() {
    setNewUsername('')
    setNewPassword('')
    setConfirmPassword('')
    setNsEntries(namespaces.length > 0 ? [makeEntry(namespaces[0])] : [])
  }

  function handleOpenChange(o: boolean) {
    setOpen(o)
    if (!o) resetDialog()
  }

  function handleNsChange(i: number, ns: string, perms: Set<string>) {
    setNsEntries(prev => prev.map((e, idx) => idx === i ? { namespace: ns, perms } : e))
  }

  function handleNsRemove(i: number) {
    setNsEntries(prev => prev.filter((_, idx) => idx !== i))
  }

  function addNsRow() {
    const unused = namespaces.find(n => !nsEntries.some(e => e.namespace === n))
    if (unused) setNsEntries(prev => [...prev, makeEntry(unused)])
  }

  const usedNamespaces = nsEntries.map(e => e.namespace)
  const canAddNs = usedNamespaces.length < namespaces.length
  const passwordsMatch = newPassword === confirmPassword
  const canCreate =
    newUsername.trim() !== '' &&
    !/\s/.test(newUsername) &&
    newPassword.trim().length >= 1 &&
    passwordsMatch &&
    nsEntries.length > 0 &&
    nsEntries.every(e => e.namespace && e.perms.size > 0)

  async function handleCreate() {
    if (!canCreate) return
    setCreating(true)
    try {
      const nsList: NamespacePermission[] = nsEntries.map(e => ({
        namespace: e.namespace,
        permissions: Array.from(e.perms),
      }))
      await api.createUser(token, newUsername.trim(), newPassword, nsList)
      toast.success(`User "${newUsername.trim()}" created`)
      setOpen(false)
      loadUsers()
    } catch (e) {
      toast.error('Failed to create user', { description: String(e) })
    } finally {
      setCreating(false)
    }
  }

  async function handleDelete(username: string) {
    try {
      await api.deleteUser(token, username)
      toast.success(`User "${username}" deleted`)
      loadUsers()
    } catch (e) {
      toast.error('Failed to delete user', { description: String(e) })
    }
  }

  if (!isRoot) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3 text-muted-foreground">
        <Users className="h-8 w-8 opacity-30" />
        <p className="text-sm">Root access required</p>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold">Users</h2>
          <p className="text-xs text-muted-foreground mt-0.5">Username + password accounts backed by session tokens</p>
        </div>
        <Button size="sm" className="gap-1.5" onClick={() => { resetDialog(); setOpen(true) }}>
          <Plus className="h-3.5 w-3.5" />
          Add user
        </Button>
      </div>

      {loading ? (
        <div className="flex justify-center py-12">
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-muted border-t-foreground" />
        </div>
      ) : users.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 gap-3 text-muted-foreground">
          <Users className="h-8 w-8 opacity-30" />
          <p className="text-sm">No user accounts yet</p>
          <p className="text-xs opacity-70">Create one so developers can log in with username + password</p>
        </div>
      ) : (
        <div className="rounded-lg border border-border overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="text-xs font-medium h-9">Username</TableHead>
                <TableHead className="text-xs font-medium h-9">Access</TableHead>
                <TableHead className="text-xs font-medium h-9">Created</TableHead>
                <TableHead className="text-xs font-medium h-9 w-14" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map(u => (
                <TableRow key={u.username}>
                  <TableCell className="font-mono text-sm font-medium">{u.username}</TableCell>
                  <TableCell><AccessCell namespaces={u.namespaces} /></TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {u.created_at ? new Date(u.created_at).toLocaleDateString() : '—'}
                  </TableCell>
                  <TableCell>
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button
                          variant="ghost" size="icon"
                          className="h-7 w-7 text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Delete user "{u.username}"?</AlertDialogTitle>
                          <AlertDialogDescription>
                            This will revoke their session token immediately. They will be signed out on their next request.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Cancel</AlertDialogCancel>
                          <AlertDialogAction
                            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                            onClick={() => handleDelete(u.username)}
                          >
                            Delete
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {/* Create user dialog */}
      <Dialog open={open} onOpenChange={handleOpenChange}>
        <DialogContent className="max-w-md max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="text-sm font-semibold">Add user</DialogTitle>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Username</Label>
              <Input
                placeholder="alice"
                value={newUsername}
                onChange={e => setNewUsername(e.target.value.replace(/\s/g, '_'))}
                className="font-mono text-sm"
                autoFocus
              />
              {newUsername && /\s/.test(newUsername) && (
                <p className="text-[11px] text-destructive/80">Username cannot contain spaces</p>
              )}
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Password</Label>
              <Input
                type="password"
                placeholder="••••••••"
                value={newPassword}
                onChange={e => setNewPassword(e.target.value)}
                className="text-sm"
              />
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs text-muted-foreground">Confirm password</Label>
              <Input
                type="password"
                placeholder="••••••••"
                value={confirmPassword}
                onChange={e => setConfirmPassword(e.target.value)}
                className="text-sm"
              />
              {confirmPassword && !passwordsMatch && (
                <p className="text-[11px] text-destructive/80">Passwords do not match</p>
              )}
            </div>

            <div className="space-y-2">
              <Label className="text-xs text-muted-foreground">Namespace access</Label>
              {nsEntries.map((e, i) => (
                <NsPermRow
                  key={i}
                  index={i}
                  ns={e.namespace}
                  perms={e.perms}
                  namespaces={namespaces}
                  usedNamespaces={usedNamespaces}
                  onChange={handleNsChange}
                  onRemove={handleNsRemove}
                  canRemove={nsEntries.length > 1}
                />
              ))}
              {canAddNs && (
                <Button variant="outline" size="sm" className="w-full gap-1.5 text-xs" onClick={addNsRow}>
                  <Plus className="h-3 w-3" />
                  Add namespace
                </Button>
              )}
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" size="sm" onClick={() => setOpen(false)}>Cancel</Button>
            <Button size="sm" disabled={!canCreate || creating} onClick={handleCreate}>
              {creating ? 'Creating…' : 'Create user'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
