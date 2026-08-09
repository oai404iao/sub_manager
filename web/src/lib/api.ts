export type User = {
  id: number
  username: string
}

export type Group = {
  id: number
  name: string
  description: string
  child_group_ids: number[]
  node_count: number
  created_at: string
}

export type Node = {
  id: number
  name: string
  protocol: "vless" | "socks5"
  server: string
  port: number
  uuid?: string
  username?: string
  password?: string
  encryption?: string
  flow?: string
  network?: string
  security?: string
  sni?: string
  alpn?: string[]
  fingerprint?: string
  allow_insecure: boolean
  public_key?: string
  short_id?: string
  spider_x?: string
  host?: string
  path?: string
  service_name?: string
  authority?: string
  header_type?: string
  xhttp_mode?: string
  xhttp_extra?: Record<string, unknown>
  kcp_mtu?: number
  kcp_tti?: number
  grpc_multi_mode: boolean
  ech_config_list?: string
  pinned_peer_cert_sha256?: string
  verify_peer_cert_by_name?: string
  mldsa65_verify?: string
  final_mask?: Record<string, unknown>
  udp: boolean
  tls: boolean
  extra?: Record<string, string>
  xray_outbound?: Record<string, unknown>
  group_ids: number[]
  subscription_id?: number
  created_at: string
  updated_at: string
}

export type NodeExportFormat = "uri" | "base64"

export type NodeExportResult = {
  content: string
  count: number
}

export type NodeGroupUpdateMode = "add" | "remove" | "replace"

export type Subscription = {
  id: number
  name: string
  url: string
  group_id: number
  last_status: string
  last_error?: string
  last_synced_at?: string
  created_at: string
}

export type Share = {
  id: number
  kind: "node" | "group"
  target_id: number
  target_name: string
  url: string
  qr_url: string
  qr_uri_url?: string
  permanent: boolean
  expired: boolean
  expires_at: string | null
  created_at: string
}

export type State = {
  user: User
  groups: Group[]
  nodes: Node[]
  subscriptions: Subscription[]
  shares: Share[]
}

export class APIError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export async function api<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const response = await fetch(path, {
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
    ...options,
  })
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as {
      error?: string
    } | null
    throw new APIError(response.status, body?.error ?? `HTTP ${response.status}`)
  }
  if (response.status === 204) {
    return undefined as T
  }
  return (await response.json()) as T
}
