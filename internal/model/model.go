package model

import "time"

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type Group struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	NodeCount   int       `json:"node_count"`
	CreatedAt   time.Time `json:"created_at"`
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
	UDP            bool              `json:"udp"`
	TLS            bool              `json:"tls"`
	Extra          map[string]string `json:"extra,omitempty"`
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

type State struct {
	User          User           `json:"user"`
	Groups        []Group        `json:"groups"`
	Nodes         []Node         `json:"nodes"`
	Subscriptions []Subscription `json:"subscriptions"`
}
