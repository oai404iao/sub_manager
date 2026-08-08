import { useMemo, useState, type FormEvent } from "react"
import {
  BoxesIcon,
  CopyIcon,
  FileInputIcon,
  LinkIcon,
  LogOutIcon,
  MoreHorizontalIcon,
  PlusIcon,
  QrCodeIcon,
  RefreshCwIcon,
  ServerIcon,
  Share2Icon,
  Trash2Icon,
} from "lucide-react"

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
  FieldDescription,
  FieldGroup,
  FieldLabel,
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
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import {
  api,
  type Group,
  type Node,
  type ShareResult,
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
const networkItems = ["tcp", "ws", "grpc", "httpupgrade", "xhttp"].map(
  (value) => ({ label: value.toUpperCase(), value }),
)
const securityItems = ["none", "tls", "reality"].map((value) => ({
  label: value.toUpperCase(),
  value,
}))

const blankNode: Node = {
  id: 0,
  name: "",
  protocol: "vless",
  server: "",
  port: 443,
  encryption: "none",
  network: "tcp",
  security: "none",
  allow_insecure: false,
  udp: true,
  tls: false,
  group_ids: [],
  created_at: "",
  updated_at: "",
}

export function Dashboard({ state, onReload, onLogout }: DashboardProps) {
  const [groupFilter, setGroupFilter] = useState(0)
  const [error, setError] = useState("")
  const [groupOpen, setGroupOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [nodeOpen, setNodeOpen] = useState(false)
  const [shareOpen, setShareOpen] = useState(false)
  const [editingNode, setEditingNode] = useState<Node>(blankNode)
  const [shareTarget, setShareTarget] = useState<{
    kind: "node" | "group"
    id: number
    name: string
  } | null>(null)

  const visibleNodes = useMemo(
    () =>
      groupFilter === 0
        ? state.nodes
        : state.nodes.filter((node) => node.group_ids.includes(groupFilter)),
    [groupFilter, state.nodes],
  )

  async function run(action: () => Promise<unknown>) {
    setError("")
    try {
      await action()
      await onReload()
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "操作失败")
    }
  }

  function openNewNode() {
    setEditingNode({
      ...blankNode,
      group_ids: groupFilter ? [groupFilter] : [],
    })
    setNodeOpen(true)
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
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground">
              <BoxesIcon />
            </div>
            <div className="min-w-0">
              <h1 className="truncate font-heading font-medium">Sub Manager</h1>
              <p className="truncate text-xs text-muted-foreground">
                Xray / Mihomo 节点与订阅管理
              </p>
            </div>
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
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={() => setGroupOpen(true)}
                >
                  <PlusIcon />
                  <span className="sr-only">新建分组</span>
                </Button>
              </div>
              <CardDescription>按用途组织和分享节点。</CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-1">
              <Button
                variant={groupFilter === 0 ? "secondary" : "ghost"}
                className="justify-between"
                onClick={() => setGroupFilter(0)}
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
                    onClick={() => setGroupFilter(group.id)}
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
                              }),
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
            <CardContent className="grid grid-cols-2 gap-3 text-sm lg:grid-cols-1">
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
          <Tabs defaultValue="nodes">
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
                <Button onClick={openNewNode}>
                  <PlusIcon data-icon="inline-start" />
                  新建节点
                </Button>
              </div>
            </div>

            <TabsContent value="nodes" className="mt-4">
              <NodeTable
                nodes={visibleNodes}
                groups={state.groups}
                onEdit={(node) => {
                  setEditingNode({ ...node })
                  setNodeOpen(true)
                }}
                onDelete={(node) =>
                  run(() =>
                    api(`/api/nodes/${node.id}`, { method: "DELETE" }),
                  )
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
                    }),
                  )
                }
                onDelete={(item) =>
                  run(() =>
                    api(`/api/subscriptions/${item.id}`, {
                      method: "DELETE",
                    }),
                  )
                }
                onImport={() => setImportOpen(true)}
              />
            </TabsContent>
          </Tabs>
        </section>
      </main>

      <GroupDialog
        open={groupOpen}
        onOpenChange={setGroupOpen}
        onCreated={() => onReload()}
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
      <ShareDialog
        open={shareOpen}
        target={shareTarget}
        onOpenChange={setShareOpen}
      />
    </div>
  )
}

function NodeTable({
  nodes,
  groups,
  onEdit,
  onDelete,
  onShare,
  onCreate,
}: {
  nodes: Node[]
  groups: Group[]
  onEdit: (node: Node) => void
  onDelete: (node: Node) => void
  onShare: (node: Node) => void
  onCreate: () => void
}) {
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
        <CardTitle>节点列表</CardTitle>
        <CardDescription>点击行可修改协议和传输配置。</CardDescription>
      </CardHeader>
      <CardContent className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
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
                onClick={() => onEdit(node)}
              >
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

function SubscriptionList({
  subscriptions,
  groups,
  onRefresh,
  onDelete,
  onImport,
}: {
  subscriptions: Subscription[]
  groups: Group[]
  onRefresh: (item: Subscription) => void
  onDelete: (item: Subscription) => void
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
            <EmptyContent>
              <Button onClick={onImport}>添加订阅</Button>
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
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => onRefresh(item)}
              >
                <RefreshCwIcon data-icon="inline-start" />
                刷新
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => onDelete(item)}
              >
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

function GroupDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: () => Promise<void>
}) {
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [error, setError] = useState("")

  async function submit(event: FormEvent) {
    event.preventDefault()
    setError("")
    try {
      await api("/api/groups", {
        method: "POST",
        body: JSON.stringify({ name, description }),
      })
      setName("")
      setDescription("")
      onOpenChange(false)
      await onCreated()
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "创建失败")
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>新建分组</DialogTitle>
            <DialogDescription>节点可以同时属于多个分组。</DialogDescription>
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
          </FieldGroup>
          <DialogFooter>
            <Button type="submit">创建</Button>
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
      <DialogContent className="sm:max-w-xl">
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>导入节点或订阅</DialogTitle>
            <DialogDescription>
              支持 VLESS/SOCKS5 链接、Base64 订阅和 Mihomo YAML。
            </DialogDescription>
          </DialogHeader>
          <Tabs
            value={mode}
            onValueChange={(value) =>
              setMode(value as "raw" | "subscription")
            }
          >
            <TabsList>
              <TabsTrigger value="raw">粘贴内容</TabsTrigger>
              <TabsTrigger value="subscription">订阅 URL</TabsTrigger>
            </TabsList>
            <TabsContent value="raw" className="mt-4">
              <Field>
                <FieldLabel htmlFor="import-content">节点内容</FieldLabel>
                <Textarea
                  id="import-content"
                  className="min-h-40 font-mono text-xs"
                  placeholder="vless://...&#10;socks5://..."
                  value={content}
                  onChange={(event) => setContent(event.target.value)}
                />
              </Field>
            </TabsContent>
            <TabsContent value="subscription" className="mt-4">
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="subscription-name">订阅名称</FieldLabel>
                  <Input
                    id="subscription-name"
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="subscription-url">
                    订阅 URL
                  </FieldLabel>
                  <Input
                    id="subscription-url"
                    type="url"
                    placeholder="https://example.com/subscription"
                    value={subscriptionURL}
                    onChange={(event) =>
                      setSubscriptionURL(event.target.value)
                    }
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

  function update<K extends keyof Node>(key: K, value: Node[K]) {
    setDraft((current) => ({ ...current, [key]: value }))
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    setError("")
    try {
      await api(draft.id ? `/api/nodes/${draft.id}` : "/api/nodes", {
        method: draft.id ? "PUT" : "POST",
        body: JSON.stringify(draft),
      })
      onOpenChange(false)
      await onSaved()
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "保存失败")
    }
  }

  const groupItems = [
    { label: "不指定分组", value: "0" },
    ...groups.map((group) => ({
      label: group.name,
      value: String(group.id),
    })),
  ]
  const selectedGroup = draft.group_ids[0] ?? 0

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90svh] overflow-y-auto sm:max-w-2xl">
        <form className="flex flex-col gap-4" onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>{draft.id ? "编辑节点" : "新建节点"}</DialogTitle>
            <DialogDescription>
              首期提供 VLESS 与 SOCKS5 的常用配置项。
            </DialogDescription>
          </DialogHeader>
          <FieldGroup>
            <div className="grid gap-4 sm:grid-cols-2">
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
              <Field>
                <FieldLabel>分组</FieldLabel>
                <Select
                  items={groupItems}
                  value={String(selectedGroup)}
                  onValueChange={(value) =>
                    update(
                      "group_ids",
                      Number(value) > 0 ? [Number(value)] : [],
                    )
                  }
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
              </Field>
            </div>

            <Separator />

            {draft.protocol === "vless" ? (
              <div className="grid gap-4 sm:grid-cols-2">
                <Field className="sm:col-span-2">
                  <FieldLabel htmlFor="node-uuid">UUID</FieldLabel>
                  <Input
                    id="node-uuid"
                    value={draft.uuid ?? ""}
                    onChange={(event) => update("uuid", event.target.value)}
                  />
                </Field>
                <Field>
                  <FieldLabel>传输</FieldLabel>
                  <Select
                    items={networkItems}
                    value={draft.network ?? "tcp"}
                    onValueChange={(value) =>
                      update("network", value ?? undefined)
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
                  <FieldLabel>安全层</FieldLabel>
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
                <Field>
                  <FieldLabel htmlFor="node-flow">Flow</FieldLabel>
                  <Input
                    id="node-flow"
                    placeholder="xtls-rprx-vision"
                    value={draft.flow ?? ""}
                    onChange={(event) => update("flow", event.target.value)}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="node-sni">SNI</FieldLabel>
                  <Input
                    id="node-sni"
                    value={draft.sni ?? ""}
                    onChange={(event) => update("sni", event.target.value)}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="node-fingerprint">
                    客户端指纹
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
                {draft.security === "reality" ? (
                  <>
                    <Field>
                      <FieldLabel htmlFor="node-public-key">
                        REALITY Public Key
                      </FieldLabel>
                      <Input
                        id="node-public-key"
                        value={draft.public_key ?? ""}
                        onChange={(event) =>
                          update("public_key", event.target.value)
                        }
                      />
                    </Field>
                    <Field>
                      <FieldLabel htmlFor="node-short-id">
                        REALITY Short ID
                      </FieldLabel>
                      <Input
                        id="node-short-id"
                        value={draft.short_id ?? ""}
                        onChange={(event) =>
                          update("short_id", event.target.value)
                        }
                      />
                    </Field>
                  </>
                ) : null}
                {["ws", "httpupgrade", "xhttp"].includes(
                  draft.network ?? "",
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
                {draft.network === "grpc" ? (
                  <Field className="sm:col-span-2">
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
                ) : null}
              </div>
            ) : (
              <div className="grid gap-4 sm:grid-cols-2">
                <Field>
                  <FieldLabel htmlFor="node-username">用户名</FieldLabel>
                  <Input
                    id="node-username"
                    value={draft.username ?? ""}
                    onChange={(event) =>
                      update("username", event.target.value)
                    }
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="node-password">密码</FieldLabel>
                  <Input
                    id="node-password"
                    type="password"
                    value={draft.password ?? ""}
                    onChange={(event) =>
                      update("password", event.target.value)
                    }
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
              </div>
            )}
          </FieldGroup>
          {error ? (
            <Alert variant="destructive">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
          <DialogFooter>
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
  onOpenChange,
}: {
  open: boolean
  target: { kind: "node" | "group"; id: number; name: string } | null
  onOpenChange: (open: boolean) => void
}) {
  const [expiresHours, setExpiresHours] = useState(720)
  const [result, setResult] = useState<ShareResult | null>(null)
  const [error, setError] = useState("")

  async function generate() {
    if (!target) return
    setError("")
    try {
      const share = await api<ShareResult>("/api/shares", {
        method: "POST",
        body: JSON.stringify({
          kind: target.kind,
          id: target.id,
          expires_hours: expiresHours,
        }),
      })
      setResult(share)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "生成失败")
    }
  }

  async function copy(value: string) {
    await navigator.clipboard.writeText(value)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next)
        if (!next) {
          setResult(null)
          setError("")
        }
      }}
    >
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>分享 {target?.name}</DialogTitle>
          <DialogDescription>
            URL 参数包含到期时间，并使用 HMAC-SHA256 防篡改。
          </DialogDescription>
        </DialogHeader>
        <Field>
          <FieldLabel htmlFor="share-hours">有效小时数</FieldLabel>
          <Input
            id="share-hours"
            type="number"
            min={1}
            max={8760}
            value={expiresHours}
            onChange={(event) => setExpiresHours(Number(event.target.value))}
          />
        </Field>
        {!result ? (
          <Button onClick={generate}>
            <Share2Icon data-icon="inline-start" />
            生成分享
          </Button>
        ) : (
          <div className="flex flex-col gap-5">
            <ShareURL
              label="订阅 URL"
              value={result.subscription_url}
              onCopy={copy}
            />
            <ShareURL
              label="节点内容 URL"
              value={result.nodes_url}
              onCopy={copy}
            />
            <Separator />
            <div className="grid gap-4 sm:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <QrCodeIcon />
                    订阅 URL 二维码
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <img
                    className="mx-auto aspect-square w-full max-w-64 rounded-lg"
                    src={result.qr_subscription_url}
                    alt="订阅 URL 二维码"
                  />
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <QrCodeIcon />
                    完整节点二维码
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <img
                    className="mx-auto aspect-square w-full max-w-64 rounded-lg"
                    src={result.qr_nodes_url}
                    alt="完整节点二维码"
                  />
                </CardContent>
              </Card>
            </div>
          </div>
        )}
        {error ? (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function ShareURL({
  label,
  value,
  onCopy,
}: {
  label: string
  value: string
  onCopy: (value: string) => void
}) {
  return (
    <Field>
      <FieldLabel>{label}</FieldLabel>
      <div className="flex gap-2">
        <Input readOnly value={value} className="font-mono text-xs" />
        <Button variant="outline" size="icon" onClick={() => onCopy(value)}>
          <CopyIcon />
          <span className="sr-only">复制</span>
        </Button>
      </div>
    </Field>
  )
}
