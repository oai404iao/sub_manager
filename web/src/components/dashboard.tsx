import { useMemo, useState, type FormEvent } from "react"
import {
  ArrowLeftIcon,
  BanIcon,
  Code2Icon,
  CopyIcon,
  DownloadIcon,
  FileInputIcon,
  HistoryIcon,
  LinkIcon,
  LogOutIcon,
  MoreHorizontalIcon,
  PencilIcon,
  PlusIcon,
  QrCodeIcon,
  RefreshCwIcon,
  ServerIcon,
  Share2Icon,
  Trash2Icon,
} from "lucide-react"

import { BrandLockup } from "@/components/brand-logo"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import {
  api,
  type Group,
  type Node,
  type NodeExportFormat,
  type NodeExportResult,
  type NodeGroupUpdateResult,
  type Share,
  type State,
  type Subscription,
} from "@/lib/api"

type DashboardProps = {
  state: State
  onReload: () => Promise<void>
  onLogout: () => void
}

const protocolItems = [
  { label: "VLESS", value: "vless" },
  { label: "SOCKS5", value: "socks5" },
]
const networkItems = [
  { label: "RAW", value: "raw" },
  { label: "XHTTP", value: "xhttp" },
  { label: "mKCP", value: "mkcp" },
  { label: "gRPC", value: "grpc" },
  { label: "WebSocket", value: "websocket" },
  { label: "HTTPUpgrade", value: "httpupgrade" },
  { label: "Hysteria", value: "hysteria" },
]
const securityItems = ["none", "tls", "reality"].map((value) => ({
  label: value.toUpperCase(),
  value,
}))
const flowItems = [
  { label: "无 Flow", value: "none" },
  { label: "xtls-rprx-vision", value: "xtls-rprx-vision" },
  {
    label: "xtls-rprx-vision-udp443",
    value: "xtls-rprx-vision-udp443",
  },
]
const xhttpModeItems = ["auto", "packet-up", "stream-up", "stream-one"].map(
  (value) => ({ label: value, value })
)
const grpcModeItems = [
  { label: "gun（默认）", value: "gun" },
  { label: "multi", value: "multi" },
]
const shareModeItems = [
  { label: "限时分享", value: "temporary" },
  { label: "永久分享", value: "permanent" },
]
const blankNode: Node = {
  id: 0,
  name: "",
  protocol: "vless",
  server: "",
  port: 443,
  encryption: "none",
  network: "raw",
  security: "tls",
  fingerprint: "chrome",
  allow_insecure: false,
  grpc_multi_mode: false,
  udp: true,
  tls: false,
  group_ids: [],
  created_at: "",
  updated_at: "",
}

function mergeNodeIntoXray(
  source: Record<string, unknown>,
  node: Node
): Record<string, unknown> {
  const outbound = structuredClone(source)
  outbound.protocol = "vless"
  outbound.tag = node.name

  const settings = recordValue(outbound.settings)
  delete settings.vnext
  settings.address = node.server
  settings.port = node.port
  settings.id = node.uuid ?? ""
  settings.encryption = node.encryption || "none"
  if (node.flow) settings.flow = node.flow
  else delete settings.flow
  outbound.settings = settings

  const stream = recordValue(outbound.streamSettings)
  stream.method = node.network || "raw"
  delete stream.network
  stream.security = node.security || "none"

  switch (node.network) {
    case "raw": {
      const raw = recordValue(stream.rawSettings)
      const header = recordValue(raw.header)
      setText(header, "type", node.header_type)
      raw.header = header
      stream.rawSettings = raw
      break
    }
    case "xhttp": {
      const xhttp = recordValue(stream.xhttpSettings)
      setText(xhttp, "host", node.host)
      setText(xhttp, "path", node.path)
      setText(xhttp, "mode", node.xhttp_mode)
      if (node.xhttp_extra) xhttp.extra = node.xhttp_extra
      stream.xhttpSettings = xhttp
      break
    }
    case "mkcp": {
      const kcp = recordValue(stream.kcpSettings)
      if (node.kcp_mtu) kcp.mtu = node.kcp_mtu
      if (node.kcp_tti) kcp.tti = node.kcp_tti
      delete kcp.header
      delete kcp.seed
      stream.kcpSettings = kcp
      break
    }
    case "grpc": {
      const grpc = recordValue(stream.grpcSettings)
      setText(grpc, "serviceName", node.service_name)
      setText(grpc, "authority", node.authority)
      grpc.multiMode = node.grpc_multi_mode
      stream.grpcSettings = grpc
      break
    }
    case "websocket": {
      const ws = recordValue(stream.wsSettings)
      setText(ws, "host", node.host)
      setText(ws, "path", node.path)
      stream.wsSettings = ws
      break
    }
    case "httpupgrade": {
      const upgrade = recordValue(stream.httpupgradeSettings)
      setText(upgrade, "host", node.host)
      setText(upgrade, "path", node.path)
      stream.httpupgradeSettings = upgrade
      break
    }
  }

  if (node.security === "tls") {
    const tls = recordValue(stream.tlsSettings)
    setText(tls, "serverName", node.sni)
    setText(tls, "fingerprint", node.fingerprint)
    if (node.alpn?.length) tls.alpn = node.alpn
    else delete tls.alpn
    setText(tls, "echConfigList", node.ech_config_list)
    setText(tls, "pinnedPeerCertSha256", node.pinned_peer_cert_sha256)
    setText(tls, "verifyPeerCertByName", node.verify_peer_cert_by_name)
    delete tls.allowInsecure
    stream.tlsSettings = tls
  } else if (node.security === "reality") {
    const reality = recordValue(stream.realitySettings)
    setText(reality, "serverName", node.sni)
    setText(reality, "fingerprint", node.fingerprint)
    setText(reality, "password", node.public_key)
    delete reality.publicKey
    setText(reality, "shortId", node.short_id)
    setText(reality, "spiderX", node.spider_x)
    setText(reality, "mldsa65Verify", node.mldsa65_verify)
    stream.realitySettings = reality
  }
  if (node.final_mask) stream.finalmask = node.final_mask
  outbound.streamSettings = stream
  return outbound
}

function recordValue(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {}
}

function setText(
  target: Record<string, unknown>,
  key: string,
  value: string | undefined
) {
  if (value) target[key] = value
  else delete target[key]
}

function nestedGroupIDs(groups: Group[], rootID: number) {
  const groupsByID = new Map(groups.map((group) => [group.id, group]))
  const result = new Set<number>()
  const pending = [rootID]
  while (pending.length > 0) {
    const groupID = pending.pop()
    if (!groupID || result.has(groupID)) continue
    result.add(groupID)
    const group = groupsByID.get(groupID)
    if (group) pending.push(...group.child_group_ids)
  }
  return result
}

async function copyText(value: string) {
  if (navigator.clipboard) {
    try {
      await navigator.clipboard.writeText(value)
      return
    } catch {
      // Fall back for self-hosted HTTP deployments without Clipboard API access.
    }
  }
  const textarea = document.createElement("textarea")
  textarea.value = value
  textarea.setAttribute("readonly", "")
  textarea.style.position = "fixed"
  textarea.style.opacity = "0"
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  const copied = document.execCommand("copy")
  textarea.remove()
  if (!copied) {
    throw new Error("复制失败，请检查浏览器剪贴板权限")
  }
}

export function Dashboard({ state, onReload, onLogout }: DashboardProps) {
  const [activeTab, setActiveTab] = useState<"nodes" | "subscriptions">("nodes")
  const [groupFilter, setGroupFilter] = useState(0)
  const [error, setError] = useState("")
  const [notice, setNotice] = useState("")
  const [groupOpen, setGroupOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [nodeOpen, setNodeOpen] = useState(false)
  const [subscriptionOpen, setSubscriptionOpen] = useState(false)
  const [shareOpen, setShareOpen] = useState(false)
  const [batchGroupOpen, setBatchGroupOpen] = useState(false)
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false)
  const [batchPending, setBatchPending] = useState(false)
  const [copyingFormat, setCopyingFormat] = useState<NodeExportFormat | null>(
    null
  )
  const [selectedNodeIDs, setSelectedNodeIDs] = useState<Set<number>>(new Set())
  const [editingGroup, setEditingGroup] = useState<Group | null>(null)
  const [editingNode, setEditingNode] = useState<Node>(blankNode)
  const [editingSubscription, setEditingSubscription] =
    useState<Subscription | null>(null)
  const [shareTarget, setShareTarget] = useState<{
    kind: "node" | "group"
    id: number
    name: string
  } | null>(null)

  const visibleNodes = useMemo(() => {
    if (groupFilter === 0) return state.nodes
    const includedGroupIDs = nestedGroupIDs(state.groups, groupFilter)
    return state.nodes.filter((node) =>
      node.group_ids.some((groupID) => includedGroupIDs.has(groupID))
    )
  }, [groupFilter, state.groups, state.nodes])
  const selectedNodes = useMemo(
    () => visibleNodes.filter((node) => selectedNodeIDs.has(node.id)),
    [selectedNodeIDs, visibleNodes]
  )
  const selectedHasManagedNodes = selectedNodes.some(
    (node) => node.subscription_id != null
  )
  const selectedSubscriptionCount = new Set(
    selectedNodes
      .map((node) => node.subscription_id)
      .filter((id): id is number => id != null)
  ).size

  async function run(
    action: () => Promise<unknown>,
    successMessage = ""
  ): Promise<boolean> {
    setError("")
    setNotice("")
    try {
      await action()
      await onReload()
      if (successMessage) setNotice(successMessage)
      return true
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "操作失败")
      return false
    }
  }

  function selectGroup(groupID: number) {
    setGroupFilter(groupID)
    setSelectedNodeIDs(new Set())
  }

  function openBatchGroups() {
    setError("")
    setNotice("")
    setBatchGroupOpen(true)
  }

  function openBatchDelete() {
    setError("")
    setNotice("")
    setBatchDeleteOpen(true)
  }

  async function copySelectedNodes(format: NodeExportFormat) {
    const ids = selectedNodes.map((node) => node.id)
    if (ids.length === 0) return
    setError("")
    setNotice("")
    setCopyingFormat(format)
    try {
      const result = await api<NodeExportResult>("/api/nodes/export", {
        method: "POST",
        body: JSON.stringify({ ids, format }),
      })
      await copyText(result.content)
      setNotice(
        `已复制 ${result.count} 个节点的${
          format === "uri" ? " URI" : " Base64"
        } 内容。`
      )
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "复制失败")
    } finally {
      setCopyingFormat(null)
    }
  }

  async function updateSelectedNodeGroups(groupIDs: number[]) {
    const ids = selectedNodes.map((node) => node.id)
    if (ids.length === 0) return false
    setBatchPending(true)
    setError("")
    setNotice("")
    try {
      const result = await api<NodeGroupUpdateResult>("/api/nodes/groups", {
        method: "PATCH",
        body: JSON.stringify({
          ids,
          group_ids: groupIDs,
          mode: "replace",
        }),
      })
      await onReload()
      setNotice(
        result.subscriptions > 0
          ? `已更新 ${result.updated} 个节点，并迁移 ${result.subscriptions} 条订阅。`
          : `已更新 ${result.updated} 个节点的分组。`
      )
      setSelectedNodeIDs(new Set())
      return true
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "修改分组失败")
      return false
    } finally {
      setBatchPending(false)
    }
  }

  async function deleteSelectedNodes() {
    const ids = selectedNodes.map((node) => node.id)
    if (ids.length === 0) return
    setBatchPending(true)
    try {
      const deleted = await run(
        () =>
          api("/api/nodes", {
            method: "DELETE",
            body: JSON.stringify({ ids }),
          }),
        `已删除 ${ids.length} 个节点。`
      )
      if (deleted) {
        setBatchDeleteOpen(false)
        setSelectedNodeIDs(new Set())
      }
    } finally {
      setBatchPending(false)
    }
  }

  function openNewNode() {
    setEditingNode({
      ...blankNode,
      group_ids: groupFilter ? [groupFilter] : [],
    })
    setNodeOpen(true)
  }

  function openNewGroup() {
    setEditingGroup(null)
    setGroupOpen(true)
  }

  function openEditGroup(group: Group) {
    setEditingGroup(group)
    setGroupOpen(true)
  }

  function openNewSubscription() {
    setEditingSubscription(null)
    setSubscriptionOpen(true)
  }

  function openEditSubscription(subscription: Subscription) {
    setEditingSubscription(subscription)
    setSubscriptionOpen(true)
  }

  function openShare(kind: "node" | "group", id: number, name: string) {
    setShareTarget({ kind, id, name })
    setShareOpen(true)
  }

  return (
    <div className="min-h-svh bg-muted/30">
      <header className="border-b bg-background">
        <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-3 sm:px-6">
          <div className="flex min-w-0 items-center gap-3">
            <BrandLockup compact heading />
          </div>
          <div className="flex items-center gap-2">
            <span className="hidden text-sm text-muted-foreground sm:inline">
              {state.user.username}
            </span>
            <Button variant="outline" size="sm" onClick={onLogout}>
              <LogOutIcon data-icon="inline-start" />
              退出
            </Button>
          </div>
        </div>
      </header>

      <main className="mx-auto grid max-w-7xl gap-6 px-4 py-6 sm:px-6 lg:grid-cols-[240px_1fr]">
        <aside className="flex flex-col gap-4">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between gap-2">
                <CardTitle>分组</CardTitle>
                <Button variant="ghost" size="icon-sm" onClick={openNewGroup}>
                  <PlusIcon />
                  <span className="sr-only">新建分组</span>
                </Button>
              </div>
              <CardDescription>按层级组织、筛选和分享节点。</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-1">
              <Button
                variant={groupFilter === 0 ? "secondary" : "ghost"}
                className="justify-between"
                onClick={() => selectGroup(0)}
              >
                <span className="flex items-center gap-2">
                  <ServerIcon data-icon="inline-start" />
                  全部节点
                </span>
                <Badge variant="outline">{state.nodes.length}</Badge>
              </Button>
              {state.groups.map((group) => (
                <div className="flex items-center gap-1" key={group.id}>
                  <Button
                    variant={groupFilter === group.id ? "secondary" : "ghost"}
                    className="min-w-0 flex-1 justify-between"
                    title={
                      group.child_group_ids.length > 0
                        ? `包含 ${group.child_group_ids.length} 个下级分组`
                        : undefined
                    }
                    onClick={() => selectGroup(group.id)}
                  >
                    <span className="truncate">{group.name}</span>
                    <Badge variant="outline">{group.node_count}</Badge>
                  </Button>
                  <DropdownMenu>
                    <DropdownMenuTrigger
                      render={<Button variant="ghost" size="icon-sm" />}
                    >
                      <MoreHorizontalIcon />
                      <span className="sr-only">分组操作</span>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuGroup>
                        <DropdownMenuItem onClick={() => openEditGroup(group)}>
                          <PencilIcon />
                          编辑分组
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() =>
                            openShare("group", group.id, group.name)
                          }
                        >
                          <Share2Icon />
                          分享分组
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          variant="destructive"
                          onClick={() =>
                            run(() =>
                              api(`/api/groups/${group.id}`, {
                                method: "DELETE",
                              })
                            )
                          }
                        >
                          <Trash2Icon />
                          删除分组
                        </DropdownMenuItem>
                      </DropdownMenuGroup>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>概览</CardTitle>
            </CardHeader>
            <CardContent className="grid grid-cols-3 gap-3 text-sm lg:grid-cols-1">
              <div>
                <p className="text-muted-foreground">节点</p>
                <p className="text-2xl font-medium">{state.nodes.length}</p>
              </div>
              <div>
                <p className="text-muted-foreground">订阅</p>
                <p className="text-2xl font-medium">
                  {state.subscriptions.length}
                </p>
              </div>
              <div>
                <p className="text-muted-foreground">分享</p>
                <p className="text-2xl font-medium">{state.shares.length}</p>
              </div>
            </CardContent>
          </Card>
        </aside>

        <section className="min-w-0">
          {error ? (
            <Alert variant="destructive" className="mb-4">
              <AlertTitle>操作失败</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
          {notice ? (
            <Alert className="mb-4">
              <AlertTitle>操作完成</AlertTitle>
              <AlertDescription>{notice}</AlertDescription>
            </Alert>
          ) : null}
          <Tabs
            value={activeTab}
            onValueChange={(value) => {
              const next = value as "nodes" | "subscriptions"
              setActiveTab(next)
              if (next !== "nodes") setSelectedNodeIDs(new Set())
            }}
          >
            <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
              <TabsList>
                <TabsTrigger value="nodes">节点</TabsTrigger>
                <TabsTrigger value="subscriptions">订阅</TabsTrigger>
              </TabsList>
              <div className="flex gap-2">
                <Button variant="outline" onClick={() => setImportOpen(true)}>
                  <FileInputIcon data-icon="inline-start" />
                  导入
                </Button>
                {activeTab === "nodes" ? (
                  <Button onClick={openNewNode}>
                    <PlusIcon data-icon="inline-start" />
                    新建节点
                  </Button>
                ) : (
                  <Button onClick={openNewSubscription}>
                    <PlusIcon data-icon="inline-start" />
                    新建订阅
                  </Button>
                )}
              </div>
            </div>

            <TabsContent value="nodes" className="mt-4">
              <NodeTable
                nodes={visibleNodes}
                groups={state.groups}
                selectedNodeIDs={selectedNodeIDs}
                selectedHasManagedNodes={selectedHasManagedNodes}
                copyingFormat={copyingFormat}
                onSelectionChange={setSelectedNodeIDs}
                onBatchGroups={openBatchGroups}
                onBatchDelete={openBatchDelete}
                onBatchCopy={copySelectedNodes}
                onEdit={(node) => {
                  setEditingNode({ ...node })
                  setNodeOpen(true)
                }}
                onDelete={(node) =>
                  run(() => api(`/api/nodes/${node.id}`, { method: "DELETE" }))
                }
                onShare={(node) => openShare("node", node.id, node.name)}
                onCreate={openNewNode}
              />
            </TabsContent>

            <TabsContent value="subscriptions" className="mt-4">
              <SubscriptionList
                subscriptions={state.subscriptions}
                groups={state.groups}
                onRefresh={(item) =>
                  run(() =>
                    api(`/api/subscriptions/${item.id}/refresh`, {
                      method: "POST",
                    })
                  )
                }
                onDelete={(item) =>
                  run(() =>
                    api(`/api/subscriptions/${item.id}`, {
                      method: "DELETE",
                    })
                  )
                }
                onEdit={openEditSubscription}
                onCreate={openNewSubscription}
                onImport={() => setImportOpen(true)}
              />
            </TabsContent>
          </Tabs>
        </section>
      </main>

      <GroupDialog
        key={`${editingGroup?.id ?? 0}-${groupOpen}`}
        open={groupOpen}
        group={editingGroup}
        groups={state.groups}
        onOpenChange={setGroupOpen}
        onSaved={() => onReload()}
      />
      <ImportDialog
        open={importOpen}
        groups={state.groups}
        onOpenChange={setImportOpen}
        onImported={() => onReload()}
      />
      <NodeDialog
        key={`${editingNode.id}-${editingNode.updated_at}-${nodeOpen}`}
        open={nodeOpen}
        node={editingNode}
        groups={state.groups}
        onOpenChange={setNodeOpen}
        onSaved={() => onReload()}
      />
      <SubscriptionDialog
        key={`${editingSubscription?.id ?? 0}-${subscriptionOpen}`}
        open={subscriptionOpen}
        subscription={editingSubscription}
        groups={state.groups}
        onOpenChange={setSubscriptionOpen}
        onSaved={() => onReload()}
      />
      <ShareDialog
        key={`share-${shareTarget?.kind ?? "none"}-${shareTarget?.id ?? 0}`}
        open={shareOpen}
        target={shareTarget}
        history={
          shareTarget
            ? state.shares.filter(
                (share) =>
                  share.kind === shareTarget.kind &&
                  share.target_id === shareTarget.id
              )
            : []
        }
        onOpenChange={setShareOpen}
        onGenerated={onReload}
      />
      <BatchGroupDialog
        key={`batch-groups-${batchGroupOpen}`}
        open={batchGroupOpen}
        groups={state.groups}
        selectedCount={selectedNodes.length}
        managedSubscriptionCount={selectedSubscriptionCount}
        error={error}
        pending={batchPending}
        onOpenChange={(open) => {
          setBatchGroupOpen(open)
          if (!open) setError("")
        }}
        onApply={updateSelectedNodeGroups}
      />
      <BatchDeleteDialog
        open={batchDeleteOpen}
        selectedCount={selectedNodes.length}
        error={error}
        pending={batchPending}
        onOpenChange={(open) => {
          setBatchDeleteOpen(open)
          if (!open) setError("")
        }}
        onConfirm={deleteSelectedNodes}
      />
    </div>
  )
}

function NodeTable({
  nodes,
  groups,
  selectedNodeIDs,
  selectedHasManagedNodes,
  copyingFormat,
  onSelectionChange,
  onBatchGroups,
  onBatchDelete,
  onBatchCopy,
  onEdit,
  onDelete,
  onShare,
  onCreate,
}: {
  nodes: Node[]
  groups: Group[]
  selectedNodeIDs: Set<number>
  selectedHasManagedNodes: boolean
  copyingFormat: NodeExportFormat | null
  onSelectionChange: (ids: Set<number>) => void
  onBatchGroups: () => void
  onBatchDelete: () => void
  onBatchCopy: (format: NodeExportFormat) => Promise<void>
  onEdit: (node: Node) => void
  onDelete: (node: Node) => void
  onShare: (node: Node) => void
  onCreate: () => void
}) {
  const selectedCount = nodes.filter((node) =>
    selectedNodeIDs.has(node.id)
  ).length
  const allSelected = selectedCount === nodes.length && nodes.length > 0
  const someSelected = selectedCount > 0 && !allSelected

  function selectAll(checked: boolean) {
    const next = new Set(selectedNodeIDs)
    for (const node of nodes) {
      if (checked) next.add(node.id)
      else next.delete(node.id)
    }
    onSelectionChange(next)
  }

  function selectNode(nodeID: number, checked: boolean) {
    const next = new Set(selectedNodeIDs)
    if (checked) next.add(nodeID)
    else next.delete(nodeID)
    onSelectionChange(next)
  }

  if (nodes.length === 0) {
    return (
      <Card>
        <CardContent>
          <Empty>
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <ServerIcon />
              </EmptyMedia>
              <EmptyTitle>还没有节点</EmptyTitle>
              <EmptyDescription>
                手动创建，或粘贴 VLESS / SOCKS5 链接导入。
              </EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button onClick={onCreate}>
                <PlusIcon data-icon="inline-start" />
                新建节点
              </Button>
            </EmptyContent>
          </Empty>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-col gap-3">
          <div>
            <CardTitle>节点列表</CardTitle>
            <CardDescription>
              点击行可修改协议和传输配置；勾选后可批量操作。
            </CardDescription>
          </div>
          {selectedCount > 0 ? (
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="secondary">已选择 {selectedCount} 个</Badge>
              <Button variant="outline" size="sm" onClick={onBatchGroups}>
                <PencilIcon data-icon="inline-start" />
                修改分组
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={copyingFormat !== null}
                onClick={() => onBatchCopy("uri")}
              >
                <LinkIcon data-icon="inline-start" />
                {copyingFormat === "uri" ? "复制中…" : "复制 URI"}
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={copyingFormat !== null}
                onClick={() => onBatchCopy("base64")}
              >
                <Code2Icon data-icon="inline-start" />
                {copyingFormat === "base64" ? "复制中…" : "复制 Base64"}
              </Button>
              <Button variant="destructive" size="sm" onClick={onBatchDelete}>
                <Trash2Icon data-icon="inline-start" />
                删除
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onSelectionChange(new Set())}
              >
                取消选择
              </Button>
              {selectedHasManagedNodes ? (
                <span className="text-xs text-muted-foreground">
                  修改托管节点时，整条订阅及其全部节点会一起迁移。
                </span>
              ) : null}
            </div>
          ) : null}
        </div>
      </CardHeader>
      <CardContent className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-10">
                <Checkbox
                  aria-label="选择全部可见节点"
                  checked={allSelected}
                  indeterminate={someSelected}
                  onCheckedChange={selectAll}
                />
              </TableHead>
              <TableHead>名称</TableHead>
              <TableHead>协议</TableHead>
              <TableHead>地址</TableHead>
              <TableHead>分组</TableHead>
              <TableHead className="w-12">
                <span className="sr-only">操作</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {nodes.map((node) => (
              <TableRow
                key={node.id}
                className="cursor-pointer"
                data-state={
                  selectedNodeIDs.has(node.id) ? "selected" : undefined
                }
                onClick={() => onEdit(node)}
              >
                <TableCell onClick={(event) => event.stopPropagation()}>
                  <Checkbox
                    aria-label={`选择节点 ${node.name}`}
                    checked={selectedNodeIDs.has(node.id)}
                    onCheckedChange={(checked) => selectNode(node.id, checked)}
                  />
                </TableCell>
                <TableCell className="font-medium">{node.name}</TableCell>
                <TableCell>
                  <Badge variant="secondary">
                    {node.protocol.toUpperCase()}
                  </Badge>
                </TableCell>
                <TableCell className="font-mono text-xs">
                  {node.server}:{node.port}
                </TableCell>
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {groups
                      .filter((group) => node.group_ids.includes(group.id))
                      .map((group) => (
                        <Badge variant="outline" key={group.id}>
                          {group.name}
                        </Badge>
                      ))}
                  </div>
                </TableCell>
                <TableCell onClick={(event) => event.stopPropagation()}>
                  <DropdownMenu>
                    <DropdownMenuTrigger
                      render={<Button variant="ghost" size="icon-sm" />}
                    >
                      <MoreHorizontalIcon />
                      <span className="sr-only">节点操作</span>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuGroup>
                        <DropdownMenuItem onClick={() => onEdit(node)}>
                          <PencilIcon />
                          编辑
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => onShare(node)}>
                          <Share2Icon />
                          分享
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          variant="destructive"
                          onClick={() => onDelete(node)}
                        >
                          <Trash2Icon />
                          删除
                        </DropdownMenuItem>
                      </DropdownMenuGroup>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

function BatchGroupDialog({
  open,
  groups,
  selectedCount,
  managedSubscriptionCount,
  error,
  pending,
  onOpenChange,
  onApply,
}: {
  open: boolean
  groups: Group[]
  selectedCount: number
  managedSubscriptionCount: number
  error: string
  pending: boolean
  onOpenChange: (open: boolean) => void
  onApply: (groupIDs: number[]) => Promise<boolean>
}) {
  const managed = managedSubscriptionCount > 0
  const [groupIDs, setGroupIDs] = useState<number[]>([])
  const [validationError, setValidationError] = useState("")
  const targetGroupItems = [
    { label: "请选择目标分组", value: "0" },
    ...groups.map((group) => ({
      label: group.name,
      value: String(group.id),
    })),
  ]

  function setGroup(groupID: number, checked: boolean) {
    setGroupIDs((current) =>
      checked
        ? [...new Set([...current, groupID])]
        : current.filter((candidate) => candidate !== groupID)
    )
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    setValidationError("")
    if (managed && groupIDs.length !== 1) {
      setValidationError("请选择一个订阅目标分组")
      return
    }
    if (await onApply(groupIDs)) {
      onOpenChange(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!pending) onOpenChange(next)
      }}
    >
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-xl">
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>
              {managed ? "迁移订阅与节点" : "批量设置所属组"}
            </DialogTitle>
            <DialogDescription>
              {managed
                ? `已选择 ${selectedCount} 个节点，涉及 ${managedSubscriptionCount} 条订阅。`
                : `将以勾选结果完整覆盖 ${selectedCount} 个节点的直接所属分组。`}
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            {managed ? (
              <>
                <Alert>
                  <AlertTitle>整条订阅迁移</AlertTitle>
                  <AlertDescription>
                    每条相关订阅的目标分组及其全部托管节点会一起迁移；
                    同时选中的手工节点会替换为同一分组。
                  </AlertDescription>
                </Alert>
                <Field>
                  <FieldLabel>目标分组</FieldLabel>
                  <Select
                    items={targetGroupItems}
                    value={String(groupIDs[0] ?? 0)}
                    disabled={pending}
                    onValueChange={(value) => {
                      const groupID = Number(value)
                      setGroupIDs(groupID > 0 ? [groupID] : [])
                      setValidationError("")
                    }}
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {targetGroupItems.map((item) => (
                          <SelectItem key={item.value} value={item.value}>
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </Field>
              </>
            ) : (
              <FieldSet>
                <FieldLegend variant="label">所属组（可多选）</FieldLegend>
                <FieldDescription>
                  保存后会完整覆盖当前归属；不勾选任何分组将清空全部归属。
                </FieldDescription>
                {groups.length > 0 ? (
                  <FieldGroup className="max-h-64 overflow-y-auto rounded-lg border p-3">
                    {groups.map((group) => {
                      const checkboxID = `batch-node-group-${group.id}`
                      return (
                        <Field key={group.id} orientation="horizontal">
                          <Checkbox
                            id={checkboxID}
                            checked={groupIDs.includes(group.id)}
                            disabled={pending}
                            onCheckedChange={(checked) =>
                              setGroup(group.id, checked)
                            }
                          />
                          <FieldLabel
                            htmlFor={checkboxID}
                            className="font-normal"
                          >
                            {group.name}
                          </FieldLabel>
                        </Field>
                      )
                    })}
                  </FieldGroup>
                ) : (
                  <FieldDescription>
                    暂无分组；应用后会清空节点的全部分组归属。
                  </FieldDescription>
                )}
              </FieldSet>
            )}
          </FieldGroup>
          {validationError || error ? (
            <Alert variant="destructive">
              <AlertDescription>{validationError || error}</AlertDescription>
            </Alert>
          ) : null}
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              disabled={pending}
              onClick={() => onOpenChange(false)}
            >
              取消
            </Button>
            <Button
              type="submit"
              disabled={
                pending ||
                selectedCount === 0 ||
                (managed && groups.length === 0)
              }
            >
              {pending ? "保存中…" : "应用修改"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function BatchDeleteDialog({
  open,
  selectedCount,
  error,
  pending,
  onOpenChange,
  onConfirm,
}: {
  open: boolean
  selectedCount: number
  error: string
  pending: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => Promise<void>
}) {
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!pending) onOpenChange(next)
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>批量删除节点</DialogTitle>
          <DialogDescription>
            确定删除已选择的 {selectedCount} 个节点吗？此操作无法撤销。
          </DialogDescription>
        </DialogHeader>
        {error ? (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}
        <DialogFooter>
          <Button
            variant="outline"
            disabled={pending}
            onClick={() => onOpenChange(false)}
          >
            取消
          </Button>
          <Button
            variant="destructive"
            disabled={pending || selectedCount === 0}
            onClick={onConfirm}
          >
            {pending ? "删除中…" : "确认删除"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function SubscriptionList({
  subscriptions,
  groups,
  onRefresh,
  onDelete,
  onEdit,
  onCreate,
  onImport,
}: {
  subscriptions: Subscription[]
  groups: Group[]
  onRefresh: (item: Subscription) => void
  onDelete: (item: Subscription) => void
  onEdit: (item: Subscription) => void
  onCreate: () => void
  onImport: () => void
}) {
  if (subscriptions.length === 0) {
    return (
      <Card>
        <CardContent>
          <Empty>
            <EmptyHeader>
              <EmptyMedia variant="icon">
                <LinkIcon />
              </EmptyMedia>
              <EmptyTitle>还没有订阅</EmptyTitle>
              <EmptyDescription>
                添加 HTTP/HTTPS 订阅并解析为分组与节点。
              </EmptyDescription>
            </EmptyHeader>
            <EmptyContent className="flex flex-wrap gap-2">
              <Button onClick={onCreate}>
                <PlusIcon data-icon="inline-start" />
                新建订阅
              </Button>
              <Button variant="outline" onClick={onImport}>
                <FileInputIcon data-icon="inline-start" />
                导入内容
              </Button>
            </EmptyContent>
          </Empty>
        </CardContent>
      </Card>
    )
  }
  return (
    <div className="grid gap-4 md:grid-cols-2">
      {subscriptions.map((item) => (
        <Card key={item.id}>
          <CardHeader>
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <CardTitle className="truncate">{item.name}</CardTitle>
                <CardDescription className="truncate">
                  {item.url}
                </CardDescription>
              </div>
              <Badge
                variant={item.last_status === "ok" ? "secondary" : "outline"}
              >
                {item.last_status}
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <div className="text-sm text-muted-foreground">
              分组：
              {groups.find((group) => group.id === item.group_id)?.name ??
                "未知"}
            </div>
            {item.last_error ? (
              <Alert variant="destructive">
                <AlertDescription>{item.last_error}</AlertDescription>
              </Alert>
            ) : null}
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" size="sm" onClick={() => onEdit(item)}>
                <PencilIcon data-icon="inline-start" />
                编辑
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => onRefresh(item)}
              >
                <RefreshCwIcon data-icon="inline-start" />
                刷新
              </Button>
              <Button variant="ghost" size="sm" onClick={() => onDelete(item)}>
                <Trash2Icon data-icon="inline-start" />
                删除
              </Button>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

function ShareHistoryList({
  shares,
  onView,
  onCopy,
  onManage,
}: {
  shares: Share[]
  onView: (share: Share) => void
  onCopy: (value: string) => void
  onManage: (share: Share, action: "revoke" | "delete") => void
}) {
  if (shares.length === 0) {
    return (
      <Empty>
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <HistoryIcon />
          </EmptyMedia>
          <EmptyTitle>还没有分享记录</EmptyTitle>
          <EmptyDescription>
            为当前对象生成分享后，会在这里保留历史。
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className="flex min-w-0 flex-col gap-3">
      {shares.map((share) => (
        <Card key={share.id} size="sm" className="min-w-0">
          <CardHeader>
            <div className="flex min-w-0 items-start justify-between gap-3">
              <div className="min-w-0">
                <CardTitle>
                  {share.permanent ? "永久分享" : "限时分享"}
                </CardTitle>
                <CardDescription>
                  创建于 {formatShareTime(share.created_at)}
                  {!share.permanent && share.expires_at
                    ? `，有效至 ${formatShareTime(share.expires_at)}`
                    : ""}
                  {share.revoked_at
                    ? `，撤销于 ${formatShareTime(share.revoked_at)}`
                    : ""}
                </CardDescription>
              </div>
              <div className="shrink-0">
                <Badge
                  variant={
                    share.revoked || share.expired
                      ? "destructive"
                      : share.permanent
                        ? "secondary"
                        : "outline"
                  }
                >
                  {share.revoked
                    ? "已撤销"
                    : share.expired
                      ? "已过期"
                      : share.permanent
                        ? "永久"
                        : "有效"}
                </Badge>
              </div>
            </div>
          </CardHeader>
          <CardContent className="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center">
            <Input
              aria-label="分享 URL"
              readOnly
              value={share.url}
              className="min-w-0 flex-1 font-mono text-xs"
            />
            <div className="flex shrink-0 justify-end gap-1">
              <Button
                variant="ghost"
                size="icon-sm"
                disabled={share.revoked || share.expired}
                onClick={() => onCopy(share.url)}
              >
                <CopyIcon />
                <span className="sr-only">复制分享 URL</span>
              </Button>
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={() => onView(share)}
              >
                <QrCodeIcon />
                <span className="sr-only">查看分享详情</span>
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={<Button variant="ghost" size="icon-sm" />}
                >
                  <MoreHorizontalIcon />
                  <span className="sr-only">管理分享记录</span>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuGroup>
                    {!share.revoked && !share.expired ? (
                      <DropdownMenuItem
                        variant="destructive"
                        onClick={() => onManage(share, "revoke")}
                      >
                        <BanIcon />
                        撤销分享
                      </DropdownMenuItem>
                    ) : null}
                    {!share.revoked && !share.expired ? (
                      <DropdownMenuSeparator />
                    ) : null}
                    <DropdownMenuItem
                      variant="destructive"
                      onClick={() => onManage(share, "delete")}
                    >
                      <Trash2Icon />
                      删除记录
                    </DropdownMenuItem>
                  </DropdownMenuGroup>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

function formatShareTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString("zh-CN", { hour12: false })
}

function GroupDialog({
  open,
  group,
  groups,
  onOpenChange,
  onSaved,
}: {
  open: boolean
  group: Group | null
  groups: Group[]
  onOpenChange: (open: boolean) => void
  onSaved: () => Promise<void>
}) {
  const [name, setName] = useState(group?.name ?? "")
  const [description, setDescription] = useState(group?.description ?? "")
  const [childGroupIDs, setChildGroupIDs] = useState(
    group?.child_group_ids ?? []
  )
  const [error, setError] = useState("")
  const unavailableGroupIDs = useMemo(() => {
    if (!group) return new Set<number>()
    return new Set(
      groups
        .filter((candidate) =>
          nestedGroupIDs(groups, candidate.id).has(group.id)
        )
        .map((candidate) => candidate.id)
    )
  }, [group, groups])
  const selectableGroups = groups.filter(
    (candidate) => candidate.id !== group?.id
  )

  function setChildGroup(groupID: number, checked: boolean) {
    setChildGroupIDs((current) =>
      checked
        ? [...new Set([...current, groupID])]
        : current.filter((candidate) => candidate !== groupID)
    )
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    setError("")
    try {
      await api(group ? `/api/groups/${group.id}` : "/api/groups", {
        method: group ? "PUT" : "POST",
        body: JSON.stringify({
          name,
          description,
          child_group_ids: childGroupIDs,
        }),
      })
      onOpenChange(false)
      await onSaved()
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "保存分组失败")
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-xl">
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>{group ? "编辑分组" : "新建分组"}</DialogTitle>
            <DialogDescription>
              节点可以属于多个分组，分组也可以递归包含下级分组。
            </DialogDescription>
          </DialogHeader>
          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="group-name">名称</FieldLabel>
              <Input
                id="group-name"
                required
                value={name}
                onChange={(event) => setName(event.target.value)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="group-description">说明</FieldLabel>
              <Textarea
                id="group-description"
                value={description}
                onChange={(event) => setDescription(event.target.value)}
              />
            </Field>
            <FieldSet>
              <FieldLegend variant="label">下级分组</FieldLegend>
              <FieldDescription>
                筛选或分享当前分组时，会递归聚合所有下级分组的节点。
              </FieldDescription>
              {selectableGroups.length > 0 ? (
                <FieldGroup className="max-h-56 overflow-y-auto rounded-lg border p-3">
                  {selectableGroups.map((candidate) => {
                    const disabled = unavailableGroupIDs.has(candidate.id)
                    const checkboxID = `group-child-${candidate.id}`
                    return (
                      <Field
                        key={candidate.id}
                        orientation="horizontal"
                        data-disabled={disabled || undefined}
                      >
                        <Checkbox
                          id={checkboxID}
                          checked={childGroupIDs.includes(candidate.id)}
                          disabled={disabled}
                          onCheckedChange={(checked) =>
                            setChildGroup(candidate.id, checked)
                          }
                        />
                        <FieldContent>
                          <FieldLabel
                            htmlFor={checkboxID}
                            className="font-normal"
                          >
                            {candidate.name}
                          </FieldLabel>
                          {disabled ? (
                            <FieldDescription>
                              选择后会形成循环层级。
                            </FieldDescription>
                          ) : candidate.child_group_ids.length > 0 ? (
                            <FieldDescription>
                              已包含 {candidate.child_group_ids.length}{" "}
                              个直接下级分组。
                            </FieldDescription>
                          ) : null}
                        </FieldContent>
                      </Field>
                    )
                  })}
                </FieldGroup>
              ) : (
                <FieldDescription>暂无其他分组可选。</FieldDescription>
              )}
            </FieldSet>
          </FieldGroup>
          <DialogFooter>
            <Button type="submit">{group ? "保存" : "创建"}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function ImportDialog({
  open,
  groups,
  onOpenChange,
  onImported,
}: {
  open: boolean
  groups: Group[]
  onOpenChange: (open: boolean) => void
  onImported: () => Promise<void>
}) {
  const [mode, setMode] = useState<"raw" | "subscription">("raw")
  const [content, setContent] = useState("")
  const [name, setName] = useState("")
  const [subscriptionURL, setSubscriptionURL] = useState("")
  const [groupID, setGroupID] = useState<number | null>(null)
  const [error, setError] = useState("")
  const groupItems = [
    { label: "不指定分组", value: "0" },
    ...groups.map((group) => ({
      label: group.name,
      value: String(group.id),
    })),
  ]

  async function submit(event: FormEvent) {
    event.preventDefault()
    setError("")
    try {
      if (mode === "raw") {
        await api("/api/nodes/import", {
          method: "POST",
          body: JSON.stringify({
            content,
            group_id: groupID && groupID > 0 ? groupID : null,
          }),
        })
        setContent("")
      } else {
        await api("/api/subscriptions", {
          method: "POST",
          body: JSON.stringify({
            name,
            url: subscriptionURL,
            group_id: groupID ?? 0,
          }),
        })
        setName("")
        setSubscriptionURL("")
      }
      onOpenChange(false)
      await onImported()
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "导入失败")
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90svh] overflow-x-hidden overflow-y-auto sm:max-w-xl">
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>导入节点或订阅</DialogTitle>
            <DialogDescription>
              支持 Mihomo/Xray YAML、VLESS/SOCKS5 URI，以及标准或 URL-safe
              Base64 订阅。
            </DialogDescription>
          </DialogHeader>
          <Tabs
            value={mode}
            onValueChange={(value) => setMode(value as "raw" | "subscription")}
          >
            <TabsList>
              <TabsTrigger value="raw">粘贴内容</TabsTrigger>
              <TabsTrigger value="subscription">订阅 URL</TabsTrigger>
            </TabsList>
            <TabsContent value="raw" className="mt-4">
              <Field>
                <FieldLabel htmlFor="import-content">
                  节点内容（YAML / URI / Base64）
                </FieldLabel>
                <Textarea
                  id="import-content"
                  className="max-h-[50svh] min-h-48 resize-y font-mono text-xs"
                  placeholder={
                    "vless://...\nsocks5://...\n\n或粘贴 YAML / Base64 订阅"
                  }
                  required
                  spellCheck={false}
                  wrap="soft"
                  value={content}
                  onChange={(event) => setContent(event.target.value)}
                />
                <FieldDescription>
                  可粘贴单条或多条 URI、Mihomo/Xray YAML、YAML URI
                  列表，以及带换行或 data URI 前缀的 Base64。
                </FieldDescription>
              </Field>
            </TabsContent>
            <TabsContent value="subscription" className="mt-4">
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="subscription-name">订阅名称</FieldLabel>
                  <Input
                    id="subscription-name"
                    required
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="subscription-url">订阅 URL</FieldLabel>
                  <Input
                    id="subscription-url"
                    type="url"
                    required
                    placeholder="https://example.com/subscription"
                    value={subscriptionURL}
                    onChange={(event) => setSubscriptionURL(event.target.value)}
                  />
                  <FieldDescription>
                    为防 SSRF，服务端拒绝私网和保留地址。
                  </FieldDescription>
                </Field>
              </FieldGroup>
            </TabsContent>
          </Tabs>
          <Field>
            <FieldLabel>目标分组</FieldLabel>
            <Select
              items={groupItems}
              value={String(groupID ?? 0)}
              onValueChange={(value) => setGroupID(Number(value))}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {groupItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>
                      {item.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            {mode === "subscription" && !groupID ? (
              <FieldDescription>
                未指定时会以订阅名称自动创建分组。
              </FieldDescription>
            ) : null}
          </Field>
          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
          <DialogFooter>
            <Button type="submit">开始导入</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function SubscriptionDialog({
  open,
  subscription,
  groups,
  onOpenChange,
  onSaved,
}: {
  open: boolean
  subscription: Subscription | null
  groups: Group[]
  onOpenChange: (open: boolean) => void
  onSaved: () => Promise<void>
}) {
  const [name, setName] = useState(subscription?.name ?? "")
  const [subscriptionURL, setSubscriptionURL] = useState(
    subscription?.url ?? ""
  )
  const [groupID, setGroupID] = useState(subscription?.group_id ?? 0)
  const [error, setError] = useState("")
  const groupItems = [
    ...(subscription ? [] : [{ label: "按订阅名称自动创建分组", value: "0" }]),
    ...groups.map((group) => ({
      label: group.name,
      value: String(group.id),
    })),
  ]

  async function submit(event: FormEvent) {
    event.preventDefault()
    setError("")
    try {
      await api(
        subscription
          ? `/api/subscriptions/${subscription.id}`
          : "/api/subscriptions",
        {
          method: subscription ? "PUT" : "POST",
          body: JSON.stringify({
            name,
            url: subscriptionURL,
            group_id: groupID,
          }),
        }
      )
      onOpenChange(false)
      await onSaved()
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "保存订阅失败")
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90svh] overflow-x-hidden overflow-y-auto sm:max-w-xl">
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>{subscription ? "编辑订阅" : "新建订阅"}</DialogTitle>
            <DialogDescription>
              保存后会立即拉取订阅，并替换该订阅上一次同步的节点。
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="managed-subscription-name">
                订阅名称
              </FieldLabel>
              <Input
                id="managed-subscription-name"
                required
                value={name}
                onChange={(event) => setName(event.target.value)}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="managed-subscription-url">
                订阅 URL
              </FieldLabel>
              <Input
                id="managed-subscription-url"
                type="url"
                required
                placeholder="https://example.com/subscription"
                value={subscriptionURL}
                onChange={(event) => setSubscriptionURL(event.target.value)}
              />
              <FieldDescription>
                为防 SSRF，服务端拒绝私网和保留地址。
              </FieldDescription>
            </Field>
            <Field>
              <FieldLabel>目标分组</FieldLabel>
              <Select
                items={groupItems}
                value={String(groupID)}
                onValueChange={(value) => setGroupID(Number(value))}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {groupItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
              {!subscription && groupID === 0 ? (
                <FieldDescription>
                  未指定时会以订阅名称自动创建分组。
                </FieldDescription>
              ) : null}
            </Field>
          </FieldGroup>
          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
          <DialogFooter>
            <Button type="submit">
              {subscription ? "保存并同步" : "创建并同步"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function NodeDialog({
  open,
  node,
  groups,
  onOpenChange,
  onSaved,
}: {
  open: boolean
  node: Node
  groups: Group[]
  onOpenChange: (open: boolean) => void
  onSaved: () => Promise<void>
}) {
  const [draft, setDraft] = useState<Node>(node)
  const [error, setError] = useState("")
  const [editorMode, setEditorMode] = useState<"basic" | "advanced">("basic")
  const [advancedJSON, setAdvancedJSON] = useState(() =>
    JSON.stringify(node.xray_outbound ?? mergeNodeIntoXray({}, node), null, 2)
  )

  function update<K extends keyof Node>(key: K, value: Node[K]) {
    setDraft((current) => ({ ...current, [key]: value }))
  }

  function setNodeGroup(groupID: number, checked: boolean) {
    setDraft((current) => ({
      ...current,
      group_ids: checked
        ? [...new Set([...current.group_ids, groupID])]
        : current.group_ids.filter((candidate) => candidate !== groupID),
    }))
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    setError("")
    try {
      let payload: Node = draft
      if (draft.protocol === "vless") {
        const parsed = JSON.parse(advancedJSON) as Record<string, unknown>
        payload = {
          ...draft,
          xray_outbound:
            editorMode === "advanced"
              ? parsed
              : mergeNodeIntoXray(parsed, draft),
        }
      } else {
        payload = { ...draft, xray_outbound: undefined }
      }
      const requestBody: Partial<Node> = { ...payload }
      delete requestBody.created_at
      delete requestBody.updated_at
      await api(payload.id ? `/api/nodes/${payload.id}` : "/api/nodes", {
        method: draft.id ? "PUT" : "POST",
        body: JSON.stringify(requestBody),
      })
      onOpenChange(false)
      await onSaved()
    } catch (caught) {
      setError(
        caught instanceof SyntaxError
          ? `Xray JSON 无效：${caught.message}`
          : caught instanceof Error
            ? caught.message
            : "保存失败"
      )
    }
  }

  const managedNode = draft.subscription_id != null
  const managedGroupItems = groups.map((group) => ({
    label: group.name,
    value: String(group.id),
  }))

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-3xl">
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>{draft.id ? "编辑节点" : "新建节点"}</DialogTitle>
            <DialogDescription>
              VLESS 对齐 Xray-core v26.7.28；完整字段可在高级 JSON 中编辑。
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <FieldGroup className="grid gap-4 sm:grid-cols-2">
              <Field>
                <FieldLabel htmlFor="node-name">名称</FieldLabel>
                <Input
                  id="node-name"
                  value={draft.name}
                  onChange={(event) => update("name", event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel>协议</FieldLabel>
                <Select
                  items={protocolItems}
                  value={draft.protocol}
                  onValueChange={(value) =>
                    update("protocol", value as Node["protocol"])
                  }
                >
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {protocolItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </Field>
              <Field>
                <FieldLabel htmlFor="node-server">服务器</FieldLabel>
                <Input
                  id="node-server"
                  value={draft.server}
                  onChange={(event) => update("server", event.target.value)}
                />
              </Field>
              <Field>
                <FieldLabel htmlFor="node-port">端口</FieldLabel>
                <Input
                  id="node-port"
                  type="number"
                  min={1}
                  max={65535}
                  value={draft.port}
                  onChange={(event) =>
                    update("port", Number(event.target.value))
                  }
                />
              </Field>
              {managedNode ? (
                <Field>
                  <FieldLabel>目标分组</FieldLabel>
                  <Select
                    items={managedGroupItems}
                    value={String(draft.group_ids[0] ?? 0)}
                    onValueChange={(value) =>
                      update("group_ids", [Number(value)])
                    }
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        {managedGroupItems.map((item) => (
                          <SelectItem key={item.value} value={item.value}>
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FieldDescription>
                    修改后会同步更新订阅目标分组，并迁移该订阅的全部节点。
                  </FieldDescription>
                </Field>
              ) : (
                <FieldSet className="sm:col-span-2">
                  <FieldLegend variant="label">所属组（可多选）</FieldLegend>
                  <FieldDescription>
                    保存后会以勾选结果完整覆盖节点当前的分组归属。
                  </FieldDescription>
                  {groups.length > 0 ? (
                    <FieldGroup className="max-h-56 overflow-y-auto rounded-lg border p-3">
                      {groups.map((group) => {
                        const checkboxID = `node-group-${draft.id || "new"}-${group.id}`
                        return (
                          <Field key={group.id} orientation="horizontal">
                            <Checkbox
                              id={checkboxID}
                              checked={draft.group_ids.includes(group.id)}
                              onCheckedChange={(checked) =>
                                setNodeGroup(group.id, checked)
                              }
                            />
                            <FieldLabel
                              htmlFor={checkboxID}
                              className="font-normal"
                            >
                              {group.name}
                            </FieldLabel>
                          </Field>
                        )
                      })}
                    </FieldGroup>
                  ) : (
                    <FieldDescription>
                      暂无分组；保存后节点将不属于任何分组。
                    </FieldDescription>
                  )}
                </FieldSet>
              )}
            </FieldGroup>

            <Separator />

            {draft.protocol === "vless" ? (
              <Tabs
                value={editorMode}
                onValueChange={(value) => {
                  const next = value as "basic" | "advanced"
                  if (next === "advanced" && editorMode === "basic") {
                    try {
                      const current = JSON.parse(advancedJSON) as Record<
                        string,
                        unknown
                      >
                      setAdvancedJSON(
                        JSON.stringify(
                          mergeNodeIntoXray(current, draft),
                          null,
                          2
                        )
                      )
                    } catch {
                      setAdvancedJSON(
                        JSON.stringify(mergeNodeIntoXray({}, draft), null, 2)
                      )
                    }
                  }
                  setEditorMode(next)
                }}
              >
                <TabsList>
                  <TabsTrigger value="basic">常用字段</TabsTrigger>
                  <TabsTrigger value="advanced">
                    <Code2Icon />
                    Xray 完整 JSON
                  </TabsTrigger>
                </TabsList>
                <TabsContent value="basic" className="mt-4">
                  <FieldGroup className="grid gap-4 sm:grid-cols-2">
                    <Field className="sm:col-span-2">
                      <FieldLabel htmlFor="node-uuid">ID / UUID</FieldLabel>
                      <Input
                        id="node-uuid"
                        value={draft.uuid ?? ""}
                        onChange={(event) => update("uuid", event.target.value)}
                      />
                      <FieldDescription>
                        可使用 UUID，或不超过 30 字节的自定义字符串；分享时按
                        Xray UUIDv5 规则映射。
                      </FieldDescription>
                    </Field>
                    <Field className="sm:col-span-2">
                      <FieldLabel htmlFor="node-encryption">
                        VLESS Encryption
                      </FieldLabel>
                      <Textarea
                        id="node-encryption"
                        className="min-h-20 font-mono text-xs"
                        value={draft.encryption ?? "none"}
                        onChange={(event) =>
                          update("encryption", event.target.value)
                        }
                      />
                      <FieldDescription>
                        必须填写；关闭时为 none。新加密格式以 mlkem768x25519plus
                        开头。
                      </FieldDescription>
                    </Field>
                    <Field>
                      <FieldLabel>Flow</FieldLabel>
                      <Select
                        items={flowItems}
                        value={draft.flow || "none"}
                        onValueChange={(value) =>
                          update("flow", value === "none" ? "" : (value ?? ""))
                        }
                      >
                        <SelectTrigger className="w-full">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {flowItems.map((item) => (
                              <SelectItem key={item.value} value={item.value}>
                                {item.label}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </Field>
                    <Field>
                      <FieldLabel>传输方法</FieldLabel>
                      <Select
                        items={networkItems}
                        value={draft.network ?? "raw"}
                        onValueChange={(value) =>
                          setDraft((current) => ({
                            ...current,
                            network: value ?? undefined,
                            security:
                              value === "hysteria" ? "tls" : current.security,
                          }))
                        }
                      >
                        <SelectTrigger className="w-full">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {networkItems.map((item) => (
                              <SelectItem key={item.value} value={item.value}>
                                {item.label}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </Field>
                    <Field>
                      <FieldLabel>传输安全</FieldLabel>
                      <Select
                        items={securityItems}
                        value={draft.security ?? "none"}
                        onValueChange={(value) =>
                          update("security", value ?? undefined)
                        }
                      >
                        <SelectTrigger className="w-full">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {securityItems.map((item) => (
                              <SelectItem key={item.value} value={item.value}>
                                {item.label}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </Field>
                    {draft.security === "none" &&
                    (draft.encryption ?? "none") === "none" ? (
                      <Alert variant="destructive" className="sm:col-span-2">
                        <AlertTitle>公开地址禁止明文 VLESS</AlertTitle>
                        <AlertDescription>
                          Xray-core v26.7.28 要求公开服务端使用 TLS、REALITY 或
                          VLESS Encryption。
                        </AlertDescription>
                      </Alert>
                    ) : null}
                    {draft.security !== "none" ? (
                      <>
                        <Field>
                          <FieldLabel htmlFor="node-sni">
                            Server Name / SNI
                          </FieldLabel>
                          <Input
                            id="node-sni"
                            value={draft.sni ?? ""}
                            onChange={(event) =>
                              update("sni", event.target.value)
                            }
                          />
                        </Field>
                        <Field>
                          <FieldLabel htmlFor="node-fingerprint">
                            uTLS 指纹
                          </FieldLabel>
                          <Input
                            id="node-fingerprint"
                            placeholder="chrome"
                            value={draft.fingerprint ?? ""}
                            onChange={(event) =>
                              update("fingerprint", event.target.value)
                            }
                          />
                        </Field>
                      </>
                    ) : null}
                    {draft.security === "tls" ? (
                      <>
                        <Field>
                          <FieldLabel htmlFor="node-alpn">ALPN</FieldLabel>
                          <Input
                            id="node-alpn"
                            placeholder="h2,http/1.1"
                            value={(draft.alpn ?? []).join(",")}
                            onChange={(event) =>
                              update(
                                "alpn",
                                event.target.value
                                  .split(",")
                                  .map((value) => value.trim())
                                  .filter(Boolean)
                              )
                            }
                          />
                        </Field>
                        <Field>
                          <FieldLabel htmlFor="node-verify-peer">
                            verifyPeerCertByName
                          </FieldLabel>
                          <Input
                            id="node-verify-peer"
                            value={draft.verify_peer_cert_by_name ?? ""}
                            onChange={(event) =>
                              update(
                                "verify_peer_cert_by_name",
                                event.target.value
                              )
                            }
                          />
                        </Field>
                        <Field className="sm:col-span-2">
                          <FieldLabel htmlFor="node-pinned-cert">
                            pinnedPeerCertSha256
                          </FieldLabel>
                          <Input
                            id="node-pinned-cert"
                            className="font-mono text-xs"
                            value={draft.pinned_peer_cert_sha256 ?? ""}
                            onChange={(event) =>
                              update(
                                "pinned_peer_cert_sha256",
                                event.target.value
                              )
                            }
                          />
                          <FieldDescription>
                            allowInsecure 已被移除；需要固定证书时使用 SHA-256。
                          </FieldDescription>
                        </Field>
                        <Field className="sm:col-span-2">
                          <FieldLabel htmlFor="node-ech">
                            ECH Config List / DNS
                          </FieldLabel>
                          <Input
                            id="node-ech"
                            value={draft.ech_config_list ?? ""}
                            onChange={(event) =>
                              update("ech_config_list", event.target.value)
                            }
                          />
                        </Field>
                      </>
                    ) : null}
                    {draft.security === "reality" ? (
                      <>
                        <Field className="sm:col-span-2">
                          <FieldLabel htmlFor="node-public-key">
                            REALITY password
                          </FieldLabel>
                          <Input
                            id="node-public-key"
                            className="font-mono text-xs"
                            value={draft.public_key ?? ""}
                            onChange={(event) =>
                              update("public_key", event.target.value)
                            }
                          />
                          <FieldDescription>
                            旧名 publicKey；最新 Xray 配置字段名为 password。
                          </FieldDescription>
                        </Field>
                        <Field>
                          <FieldLabel htmlFor="node-short-id">
                            REALITY shortId
                          </FieldLabel>
                          <Input
                            id="node-short-id"
                            value={draft.short_id ?? ""}
                            onChange={(event) =>
                              update("short_id", event.target.value)
                            }
                          />
                        </Field>
                        <Field>
                          <FieldLabel htmlFor="node-spider-x">
                            REALITY spiderX
                          </FieldLabel>
                          <Input
                            id="node-spider-x"
                            placeholder="/"
                            value={draft.spider_x ?? ""}
                            onChange={(event) =>
                              update("spider_x", event.target.value)
                            }
                          />
                        </Field>
                        <Field className="sm:col-span-2">
                          <FieldLabel htmlFor="node-mldsa">
                            REALITY mldsa65Verify
                          </FieldLabel>
                          <Textarea
                            id="node-mldsa"
                            className="min-h-20 font-mono text-xs"
                            value={draft.mldsa65_verify ?? ""}
                            onChange={(event) =>
                              update("mldsa65_verify", event.target.value)
                            }
                          />
                        </Field>
                        <Field
                          className="sm:col-span-2"
                          orientation="horizontal"
                        >
                          <Checkbox
                            id="node-mihomo-mlkem"
                            checked={
                              draft.extra?.["support-x25519mlkem768"] === "true"
                            }
                            onCheckedChange={(checked) =>
                              setDraft((current) => {
                                const extra = { ...current.extra }
                                if (checked) {
                                  extra["support-x25519mlkem768"] = "true"
                                } else {
                                  delete extra["support-x25519mlkem768"]
                                }
                                return { ...current, extra }
                              })
                            }
                          />
                          <FieldContent>
                            <FieldLabel htmlFor="node-mihomo-mlkem">
                              Mihomo X25519-MLKEM768
                            </FieldLabel>
                            <FieldDescription>
                              仅在 REALITY 分享链接中添加 Mihomo 专用参数；不是
                              Xray JSON 字段。
                            </FieldDescription>
                          </FieldContent>
                        </Field>
                      </>
                    ) : null}

                    {["websocket", "httpupgrade", "xhttp"].includes(
                      draft.network ?? ""
                    ) ? (
                      <>
                        <Field>
                          <FieldLabel htmlFor="node-host">Host</FieldLabel>
                          <Input
                            id="node-host"
                            value={draft.host ?? ""}
                            onChange={(event) =>
                              update("host", event.target.value)
                            }
                          />
                        </Field>
                        <Field>
                          <FieldLabel htmlFor="node-path">Path</FieldLabel>
                          <Input
                            id="node-path"
                            value={draft.path ?? ""}
                            onChange={(event) =>
                              update("path", event.target.value)
                            }
                          />
                        </Field>
                      </>
                    ) : null}
                    {draft.network === "xhttp" ? (
                      <Field>
                        <FieldLabel>XHTTP Mode</FieldLabel>
                        <Select
                          items={xhttpModeItems}
                          value={draft.xhttp_mode || "auto"}
                          onValueChange={(value) =>
                            update("xhttp_mode", value ?? "auto")
                          }
                        >
                          <SelectTrigger className="w-full">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {xhttpModeItems.map((item) => (
                                <SelectItem key={item.value} value={item.value}>
                                  {item.label}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                      </Field>
                    ) : null}
                    {draft.network === "mkcp" ? (
                      <>
                        <Field>
                          <FieldLabel htmlFor="node-kcp-mtu">
                            mKCP MTU
                          </FieldLabel>
                          <Input
                            id="node-kcp-mtu"
                            type="number"
                            value={draft.kcp_mtu ?? 1350}
                            onChange={(event) =>
                              update("kcp_mtu", Number(event.target.value))
                            }
                          />
                        </Field>
                        <Field>
                          <FieldLabel htmlFor="node-kcp-tti">
                            mKCP TTI
                          </FieldLabel>
                          <Input
                            id="node-kcp-tti"
                            type="number"
                            value={draft.kcp_tti ?? 50}
                            onChange={(event) =>
                              update("kcp_tti", Number(event.target.value))
                            }
                          />
                        </Field>
                      </>
                    ) : null}
                    {draft.network === "grpc" ? (
                      <>
                        <Field>
                          <FieldLabel htmlFor="node-service-name">
                            gRPC Service Name
                          </FieldLabel>
                          <Input
                            id="node-service-name"
                            value={draft.service_name ?? ""}
                            onChange={(event) =>
                              update("service_name", event.target.value)
                            }
                          />
                        </Field>
                        <Field>
                          <FieldLabel htmlFor="node-authority">
                            gRPC Authority
                          </FieldLabel>
                          <Input
                            id="node-authority"
                            value={draft.authority ?? ""}
                            onChange={(event) =>
                              update("authority", event.target.value)
                            }
                          />
                        </Field>
                        <Field>
                          <FieldLabel>gRPC Mode</FieldLabel>
                          <Select
                            items={grpcModeItems}
                            value={draft.grpc_multi_mode ? "multi" : "gun"}
                            onValueChange={(value) =>
                              update("grpc_multi_mode", value === "multi")
                            }
                          >
                            <SelectTrigger className="w-full">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent alignItemWithTrigger={false}>
                              <SelectGroup>
                                {grpcModeItems.map((item) => (
                                  <SelectItem
                                    key={item.value}
                                    value={item.value}
                                  >
                                    {item.label}
                                  </SelectItem>
                                ))}
                              </SelectGroup>
                            </SelectContent>
                          </Select>
                        </Field>
                      </>
                    ) : null}
                    {draft.network === "raw" ? (
                      <Field>
                        <FieldLabel htmlFor="node-header-type">
                          RAW Header Type
                        </FieldLabel>
                        <Input
                          id="node-header-type"
                          placeholder="none"
                          value={draft.header_type ?? ""}
                          onChange={(event) =>
                            update("header_type", event.target.value)
                          }
                        />
                      </Field>
                    ) : null}
                    {draft.network === "hysteria" ? (
                      <Alert className="sm:col-span-2">
                        <AlertTitle>Hysteria 传输需要 TLS</AlertTitle>
                        <AlertDescription>
                          auth、masquerade 与 QUIC 参数请在“Xray 完整
                          JSON”中配置。
                        </AlertDescription>
                      </Alert>
                    ) : null}
                  </FieldGroup>
                </TabsContent>
                <TabsContent value="advanced" className="mt-4">
                  <Field>
                    <FieldLabel htmlFor="xray-outbound-json">
                      Xray VLESS OutboundObject
                    </FieldLabel>
                    <Textarea
                      id="xray-outbound-json"
                      className="min-h-96 font-mono text-xs"
                      spellCheck={false}
                      value={advancedJSON}
                      onChange={(event) => setAdvancedJSON(event.target.value)}
                    />
                    <FieldDescription>
                      支持 v26.7.28 的完整 streamSettings、XHTTP、FinalMask、
                      Sockopt、Mux、ProxySettings 与 TLS/REALITY 高级字段。
                    </FieldDescription>
                  </Field>
                </TabsContent>
              </Tabs>
            ) : (
              <FieldGroup className="grid gap-4 sm:grid-cols-2">
                <Field>
                  <FieldLabel htmlFor="node-username">用户名</FieldLabel>
                  <Input
                    id="node-username"
                    value={draft.username ?? ""}
                    onChange={(event) => update("username", event.target.value)}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="node-password">密码</FieldLabel>
                  <Input
                    id="node-password"
                    type="password"
                    value={draft.password ?? ""}
                    onChange={(event) => update("password", event.target.value)}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="socks-sni">TLS SNI</FieldLabel>
                  <Input
                    id="socks-sni"
                    value={draft.sni ?? ""}
                    onChange={(event) => update("sni", event.target.value)}
                  />
                </Field>
              </FieldGroup>
            )}
          </FieldGroup>
          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
          <DialogFooter>
            {draft.id && draft.protocol === "vless" ? (
              <Button
                variant="outline"
                render={<a href={`/api/nodes/${draft.id}/xray`} />}
                nativeButton={false}
              >
                <DownloadIcon data-icon="inline-start" />
                下载 Xray JSON
              </Button>
            ) : null}
            <Button type="submit">保存节点</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function ShareDialog({
  open,
  target,
  history,
  onOpenChange,
  onGenerated,
}: {
  open: boolean
  target: { kind: "node" | "group"; id: number; name: string } | null
  history: Share[]
  onOpenChange: (open: boolean) => void
  onGenerated: () => Promise<void>
}) {
  const [activeSection, setActiveSection] = useState<"create" | "history">(
    "create"
  )
  const [shareMode, setShareMode] = useState<"temporary" | "permanent">(
    "temporary"
  )
  const [expiresHours, setExpiresHours] = useState(720)
  const [generatedShare, setGeneratedShare] = useState<Share | null>(null)
  const [historyShare, setHistoryShare] = useState<Share | null>(null)
  const [shareAction, setShareAction] = useState<{
    share: Share
    action: "revoke" | "delete"
  } | null>(null)
  const [managementPending, setManagementPending] = useState(false)
  const [managementError, setManagementError] = useState("")
  const [error, setError] = useState("")

  async function generate() {
    if (!target) return
    setError("")
    try {
      const permanent = shareMode === "permanent"
      const share = await api<Share>("/api/shares", {
        method: "POST",
        body: JSON.stringify({
          kind: target.kind,
          id: target.id,
          expires_hours: permanent ? 0 : expiresHours,
          permanent,
        }),
      })
      setGeneratedShare(share)
      await onGenerated()
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "生成失败")
    }
  }

  async function copy(value: string) {
    setError("")
    try {
      await copyText(value)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "复制失败")
    }
  }

  async function confirmShareAction() {
    if (!shareAction) return
    setManagementPending(true)
    setManagementError("")
    try {
      if (shareAction.action === "revoke") {
        await api(`/api/shares/${shareAction.share.id}/revoke`, {
          method: "POST",
        })
      } else {
        await api(`/api/shares/${shareAction.share.id}`, {
          method: "DELETE",
        })
      }
      await onGenerated()
      if (historyShare?.id === shareAction.share.id) {
        setHistoryShare(null)
      }
      setShareAction(null)
    } catch (caught) {
      setManagementError(
        caught instanceof Error ? caught.message : "管理分享记录失败"
      )
    } finally {
      setManagementPending(false)
    }
  }

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={(next) => {
          onOpenChange(next)
          if (!next) {
            setActiveSection("create")
            setShareMode("temporary")
            setExpiresHours(720)
            setGeneratedShare(null)
            setHistoryShare(null)
            setShareAction(null)
            setError("")
          }
        }}
      >
        <DialogContent className="max-h-[calc(100svh-2rem)] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>分享 {target?.name ?? ""}</DialogTitle>
            <DialogDescription>
              在这里生成分享，并查看当前节点或分组的历史记录。
            </DialogDescription>
          </DialogHeader>
          <Tabs
            value={activeSection}
            onValueChange={(value) => {
              if (value === "create" || value === "history") {
                setActiveSection(value)
                setError("")
              }
            }}
          >
            <TabsList>
              <TabsTrigger value="create">新建分享</TabsTrigger>
              <TabsTrigger value="history">
                分享历史（{history.length}）
              </TabsTrigger>
            </TabsList>
            <TabsContent value="create" className="mt-4">
              {generatedShare ? (
                <div className="flex flex-col gap-4">
                  <div className="flex justify-end">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => setGeneratedShare(null)}
                    >
                      <Share2Icon data-icon="inline-start" />
                      继续生成
                    </Button>
                  </div>
                  <ShareDetails share={generatedShare} onCopy={copy} />
                </div>
              ) : (
                <FieldGroup>
                  <Field>
                    <FieldLabel>分享类型</FieldLabel>
                    <Select
                      items={shareModeItems}
                      value={shareMode}
                      onValueChange={(value) => {
                        if (value === "temporary" || value === "permanent") {
                          setShareMode(value)
                        }
                      }}
                    >
                      <SelectTrigger className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          {shareModeItems.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                    <FieldDescription>
                      永久分享不会自动过期；修改签名密钥仍会使链接失效。
                    </FieldDescription>
                  </Field>
                  {shareMode === "temporary" ? (
                    <Field>
                      <FieldLabel htmlFor="share-hours">有效小时数</FieldLabel>
                      <Input
                        id="share-hours"
                        type="number"
                        min={1}
                        max={8760}
                        value={expiresHours}
                        onChange={(event) =>
                          setExpiresHours(Number(event.target.value))
                        }
                      />
                    </Field>
                  ) : null}
                  <Button onClick={generate}>
                    <Share2Icon data-icon="inline-start" />
                    生成分享
                  </Button>
                </FieldGroup>
              )}
            </TabsContent>
            <TabsContent value="history" className="mt-4">
              {historyShare ? (
                <div className="flex flex-col gap-4">
                  <Button
                    className="self-start"
                    variant="ghost"
                    size="sm"
                    onClick={() => setHistoryShare(null)}
                  >
                    <ArrowLeftIcon data-icon="inline-start" />
                    返回历史
                  </Button>
                  <ShareDetails share={historyShare} onCopy={copy} />
                </div>
              ) : (
                <ShareHistoryList
                  shares={history}
                  onView={setHistoryShare}
                  onCopy={copy}
                  onManage={(share, action) => {
                    setManagementError("")
                    setShareAction({ share, action })
                  }}
                />
              )}
            </TabsContent>
          </Tabs>
          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
        </DialogContent>
      </Dialog>
      <ShareManagementDialog
        action={shareAction}
        error={managementError}
        pending={managementPending}
        onOpenChange={(next) => {
          if (!next && !managementPending) {
            setShareAction(null)
            setManagementError("")
          }
        }}
        onConfirm={confirmShareAction}
      />
    </>
  )
}

function ShareManagementDialog({
  action,
  error,
  pending,
  onOpenChange,
  onConfirm,
}: {
  action: { share: Share; action: "revoke" | "delete" } | null
  error: string
  pending: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => Promise<void>
}) {
  const revoke = action?.action === "revoke"
  return (
    <Dialog open={action != null} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{revoke ? "撤销分享" : "删除分享记录"}</DialogTitle>
          <DialogDescription>
            {revoke
              ? `撤销“${action?.share.target_name ?? ""}”的分享后，URL 会立即失效，且无法恢复。`
              : `删除“${action?.share.target_name ?? ""}”的历史记录后，URL 也会失效，且无法恢复。`}
          </DialogDescription>
        </DialogHeader>
        {error ? (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}
        <DialogFooter>
          <Button
            variant="outline"
            disabled={pending}
            onClick={() => onOpenChange(false)}
          >
            取消
          </Button>
          <Button variant="destructive" disabled={pending} onClick={onConfirm}>
            {revoke ? (
              <BanIcon data-icon="inline-start" />
            ) : (
              <Trash2Icon data-icon="inline-start" />
            )}
            {pending ? "处理中…" : revoke ? "确认撤销" : "确认删除"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function ShareDetails({
  share,
  onCopy,
}: {
  share: Share
  onCopy: (value: string) => void
}) {
  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="outline">
          {share.kind === "node" ? "节点" : "分组"}
        </Badge>
        <Badge
          variant={
            share.revoked || share.expired
              ? "destructive"
              : share.permanent
                ? "secondary"
                : "outline"
          }
        >
          {share.revoked
            ? "已撤销"
            : share.expired
              ? "已过期"
              : share.permanent
                ? "永久有效"
                : `有效至 ${formatShareTime(share.expires_at ?? "")}`}
        </Badge>
        <span className="text-sm text-muted-foreground">
          {share.target_name} · 创建于 {formatShareTime(share.created_at)}
        </span>
      </div>
      {share.revoked ? (
        <Alert variant="destructive">
          <AlertTitle>分享已撤销</AlertTitle>
          <AlertDescription>
            此 URL 已无法访问，历史记录仅用于查看。
          </AlertDescription>
        </Alert>
      ) : null}
      <ShareURL
        label="分享 URL"
        value={share.url}
        disabled={share.revoked || share.expired}
        onCopy={onCopy}
      />
      <Separator />
      <div className="flex flex-wrap gap-4">
        <Card className="min-w-64 flex-1">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <QrCodeIcon />
              分享 URL 二维码
            </CardTitle>
          </CardHeader>
          <CardContent>
            <img
              className="mx-auto aspect-square w-full max-w-64 rounded-lg"
              src={share.qr_url}
              alt="分享 URL 二维码"
            />
          </CardContent>
        </Card>
        {share.qr_uri_url ? (
          <Card className="min-w-64 flex-1">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <QrCodeIcon />
                节点 URI 二维码
              </CardTitle>
            </CardHeader>
            <CardContent>
              <img
                className="mx-auto aspect-square w-full max-w-64 rounded-lg"
                src={share.qr_uri_url}
                alt="节点 URI 二维码"
              />
            </CardContent>
          </Card>
        ) : null}
      </div>
    </div>
  )
}

function ShareURL({
  label,
  value,
  disabled = false,
  onCopy,
}: {
  label: string
  value: string
  disabled?: boolean
  onCopy: (value: string) => void
}) {
  return (
    <Field className="min-w-0">
      <FieldLabel>{label}</FieldLabel>
      <div className="flex min-w-0 gap-2">
        <Input
          readOnly
          value={value}
          className="min-w-0 flex-1 font-mono text-xs"
        />
        <Button
          variant="outline"
          size="icon"
          disabled={disabled}
          onClick={() => onCopy(value)}
        >
          <CopyIcon />
          <span className="sr-only">复制</span>
        </Button>
      </div>
    </Field>
  )
}
