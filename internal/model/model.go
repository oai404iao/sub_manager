package model

import "time"

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type Group struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	ChildGroupIDs []int64   `json:"child_group_ids"`
	NodeCount     int       `json:"node_count"`
	CreatedAt     time.Time `json:"created_at"`
}

type Node struct {
	ID             int64             `json:"id"`
	Name           string            `json:"name"`
	Protocol       string            `json:"protocol"`
	Server         string            `json:"server"`
	Port           int               `json:"port"`
	UUID           string            `json:"uuid,omitempty"`
	Username       string            `json:"username,omitempty"`
	Password       string            `json:"password,omitempty"`
	Encryption     string            `json:"encryption,omitempty"`
	Flow           string            `json:"flow,omitempty"`
	Network        string            `json:"network,omitempty"`
	Security       string            `json:"security,omitempty"`
	SNI            string            `json:"sni,omitempty"`
	ALPN           []string          `json:"alpn,omitempty"`
	Fingerprint    string            `json:"fingerprint,omitempty"`
	AllowInsecure  bool              `json:"allow_insecure"`
	PublicKey      string            `json:"public_key,omitempty"`
	ShortID        string            `json:"short_id,omitempty"`
	SpiderX        string            `json:"spider_x,omitempty"`
	Host           string            `json:"host,omitempty"`
	Path           string            `json:"path,omitempty"`
	ServiceName    string            `json:"service_name,omitempty"`
	Authority      string            `json:"authority,omitempty"`
	HeaderType     string            `json:"header_type,omitempty"`
	XHTTPMode      string            `json:"xhttp_mode,omitempty"`
	XHTTPExtra     map[string]any    `json:"xhttp_extra,omitempty"`
	KCPMTU         int               `json:"kcp_mtu,omitempty"`
	KCPTTI         int               `json:"kcp_tti,omitempty"`
	GRPCMultiMode  bool              `json:"grpc_multi_mode"`
	ECHConfigList  string            `json:"ech_config_list,omitempty"`
	PinnedPeerCert string            `json:"pinned_peer_cert_sha256,omitempty"`
	VerifyPeerName string            `json:"verify_peer_cert_by_name,omitempty"`
	MLDSA65Verify  string            `json:"mldsa65_verify,omitempty"`
	FinalMask      map[string]any    `json:"final_mask,omitempty"`
	UDP            bool              `json:"udp"`
	TLS            bool              `json:"tls"`
	Extra          map[string]string `json:"extra,omitempty"`
	XrayOutbound   map[string]any    `json:"xray_outbound,omitempty"`
	GroupIDs       []int64           `json:"group_ids"`
	SubscriptionID *int64            `json:"subscription_id,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type Subscription struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	URL          string     `json:"url"`
	GroupID      int64      `json:"group_id"`
	LastStatus   string     `json:"last_status"`
	LastError    string     `json:"last_error,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Share struct {
	ID         int64      `json:"id"`
	Kind       string     `json:"kind"`
	TargetID   int64      `json:"target_id"`
	TargetName string     `json:"target_name"`
	URL        string     `json:"url"`
	QRURL      string     `json:"qr_url,omitempty"`
	QRURIURL   string     `json:"qr_uri_url,omitempty"`
	TokenHash  string     `json:"-"`
	Permanent  bool       `json:"permanent"`
	Expired    bool       `json:"expired"`
	Revoked    bool       `json:"revoked"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at"`
	DeletedAt  *time.Time `json:"-"`
	CreatedAt  time.Time  `json:"created_at"`
}

type State struct {
	User          User           `json:"user"`
	Groups        []Group        `json:"groups"`
	Nodes         []Node         `json:"nodes"`
	Subscriptions []Subscription `json:"subscriptions"`
	Shares        []Share        `json:"shares"`
}
