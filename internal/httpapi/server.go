package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"submanager/internal/auth"
	"submanager/internal/config"
	"submanager/internal/model"
	"submanager/internal/protocol"
	"submanager/internal/share"
	"submanager/internal/store"
	"submanager/internal/webassets"
)

const sessionCookie = "subman_session"

type Server struct {
	cfg    config.Config
	store  *store.Store
	signer share.Signer
	client *http.Client
}

func New(cfg config.Config, store *store.Store) *Server {
	server := &Server{
		cfg:    cfg,
		store:  store,
		signer: share.New(cfg.SigningKey),
	}
	server.client = safeHTTPClient()
	return server
}

func (s *Server) Handler() http.Handler {
	root := http.NewServeMux()
	root.HandleFunc("POST /api/auth/login", s.login)

	api := http.NewServeMux()
	api.HandleFunc("GET /api/auth/me", s.me)
	api.HandleFunc("POST /api/auth/logout", s.logout)
	api.HandleFunc("GET /api/state", s.state)
	api.HandleFunc("POST /api/groups", s.createGroup)
	api.HandleFunc("DELETE /api/groups/{id}", s.deleteGroup)
	api.HandleFunc("POST /api/nodes", s.saveNode)
	api.HandleFunc("PUT /api/nodes/{id}", s.saveNode)
	api.HandleFunc("GET /api/nodes/{id}/xray", s.exportXrayNode)
	api.HandleFunc("DELETE /api/nodes/{id}", s.deleteNode)
	api.HandleFunc("POST /api/nodes/import", s.importNodes)
	api.HandleFunc("POST /api/subscriptions", s.createSubscription)
	api.HandleFunc("POST /api/subscriptions/{id}/refresh", s.refreshSubscription)
	api.HandleFunc("DELETE /api/subscriptions/{id}", s.deleteSubscription)
	api.HandleFunc("POST /api/shares", s.createShare)
	api.HandleFunc("GET /api/qr", s.qr)
	root.Handle("/api/", s.requireAuth(api))

	root.HandleFunc("GET /s", s.publicShare)
	root.Handle("/", spaHandler())
	return s.recoverAndLog(root)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	user, ok, err := s.store.Authenticate(strings.TrimSpace(request.Username), request.Password)
	if err != nil {
		serverError(w, err)
		return
	}
	if !ok {
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	plain, digest, err := auth.NewToken()
	if err != nil {
		serverError(w, err)
		return
	}
	expires := time.Now().Add(7 * 24 * time.Hour)
	if err := s.store.CreateSession(user.ID, digest, expires); err != nil {
		serverError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    plain,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
	})
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_ = s.store.DeleteSession(auth.Digest(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, currentUser(r))
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	state, err := s.store.State(r.Context(), currentUser(r))
	if err != nil {
		serverError(w, err)
		return
	}
	for index := range state.Nodes {
		// Keep legacy/invalid nodes visible so administrators can repair them.
		if protocol.EnsureXrayOutbound(&state.Nodes[index]) == nil {
			_ = protocol.ApplyXrayOutbound(&state.Nodes[index])
		}
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	group, err := s.store.CreateGroup(r.Context(), request.Name, request.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, friendlyDBError(err))
		return
	}
	writeJSON(w, http.StatusCreated, group)
}

func (s *Server) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteGroup(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, "分组仍被订阅使用或不存在")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) saveNode(w http.ResponseWriter, r *http.Request) {
	var node model.Node
	if !decodeJSON(w, r, &node) {
		return
	}
	if r.Method == http.MethodPut {
		id, ok := pathID(w, r)
		if !ok {
			return
		}
		node.ID = id
	}
	if err := protocol.ApplyXrayOutbound(&node); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	normalizeNode(&node)
	if err := protocol.EnsureXrayOutbound(&node); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := protocol.Validate(node); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	saved, err := s.store.SaveNode(r.Context(), node)
	if err != nil {
		serverError(w, err)
		return
	}
	status := http.StatusCreated
	if r.Method == http.MethodPut {
		status = http.StatusOK
	}
	writeJSON(w, status, saved)
}

func (s *Server) exportXrayNode(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	node, err := s.store.Node(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "节点不存在")
		return
	}
	data, err := protocol.XrayOutboundJSON(node)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="vless-%d-xray.json"`, id))
	_, _ = w.Write(data)
}

func (s *Server) deleteNode(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteNode(r.Context(), id); err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) importNodes(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Content string `json:"content"`
		GroupID *int64 `json:"group_id"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	nodes, err := protocol.ParseText(request.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	for index := range nodes {
		if request.GroupID != nil && *request.GroupID > 0 {
			nodes[index].GroupIDs = []int64{*request.GroupID}
		}
	}
	count, err := s.store.SaveNodes(r.Context(), nodes)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int{"imported": count})
}

func (s *Server) createSubscription(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		GroupID int64  `json:"group_id"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		writeError(w, http.StatusBadRequest, "订阅名称不能为空")
		return
	}
	if request.GroupID == 0 {
		group, err := s.store.CreateGroup(r.Context(), request.Name, "由订阅自动创建")
		if err != nil {
			writeError(w, http.StatusBadRequest, friendlyDBError(err))
			return
		}
		request.GroupID = group.ID
	}
	subscription, err := s.store.CreateSubscription(r.Context(), request.Name, request.URL, request.GroupID)
	if err != nil {
		writeError(w, http.StatusBadRequest, friendlyDBError(err))
		return
	}
	count, err := s.syncSubscription(r.Context(), subscription)
	if err != nil {
		_ = s.store.UpdateSubscriptionStatus(r.Context(), subscription.ID, "error", err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.UpdateSubscriptionStatus(r.Context(), subscription.ID, "ok", "")
	writeJSON(w, http.StatusCreated, map[string]any{"subscription": subscription, "imported": count})
}

func (s *Server) refreshSubscription(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	subscription, err := s.store.Subscription(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "订阅不存在")
		return
	}
	count, err := s.syncSubscription(r.Context(), subscription)
	if err != nil {
		_ = s.store.UpdateSubscriptionStatus(r.Context(), id, "error", err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.store.UpdateSubscriptionStatus(r.Context(), id, "ok", "")
	writeJSON(w, http.StatusOK, map[string]int{"imported": count})
}

func (s *Server) deleteSubscription(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteSubscription(r.Context(), id); err != nil {
		serverError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) syncSubscription(ctx context.Context, subscription model.Subscription) (int, error) {
	content, err := s.fetchSubscription(ctx, subscription.URL)
	if err != nil {
		return 0, err
	}
	nodes, err := protocol.ParseText(content)
	if err != nil {
		return 0, err
	}
	for index := range nodes {
		nodes[index].GroupIDs = []int64{subscription.GroupID}
		nodes[index].SubscriptionID = &subscription.ID
	}
	return s.store.SaveNodes(ctx, nodes)
}

func (s *Server) createShare(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Kind         string `json:"kind"`
		ID           int64  `json:"id"`
		ExpiresHours int    `json:"expires_hours"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.Kind != "node" && request.Kind != "group" {
		writeError(w, http.StatusBadRequest, "kind 必须为 node 或 group")
		return
	}
	if request.ExpiresHours < 1 {
		request.ExpiresHours = 24 * 30
	}
	if request.ExpiresHours > 24*365 {
		request.ExpiresHours = 24 * 365
	}
	nodes, err := s.shareNodes(r.Context(), request.Kind, request.ID)
	if err != nil || len(nodes) == 0 {
		writeError(w, http.StatusNotFound, "没有可分享的节点")
		return
	}
	exp := time.Now().Add(time.Duration(request.ExpiresHours) * time.Hour).Unix()
	subscriptionURL := s.signedURL(request.Kind, request.ID, "subscription", exp)
	nodesURL := s.signedURL(request.Kind, request.ID, "nodes", exp)
	fullContent, err := protocol.FullContent(nodes)
	if err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"subscription_url":    subscriptionURL,
		"nodes_url":           nodesURL,
		"qr_subscription_url": "/api/qr?data=" + url.QueryEscape(subscriptionURL),
		"qr_nodes_url":        "/api/qr?data=" + url.QueryEscape(fullContent),
		"expires_at":          time.Unix(exp, 0).UTC(),
	})
}

func (s *Server) publicShare(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	kind := query.Get("kind")
	contentType := query.Get("content")
	id, err := strconv.ParseInt(query.Get("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	exp, err := strconv.ParseInt(query.Get("exp"), 10, 64)
	if err != nil || time.Now().Unix() > exp {
		writeError(w, http.StatusGone, "share URL expired")
		return
	}
	message := shareMessage(kind, id, contentType, exp)
	if !s.signer.Verify(message, query.Get("sig")) {
		writeError(w, http.StatusForbidden, "invalid signature")
		return
	}
	nodes, err := s.shareNodes(r.Context(), kind, id)
	if err != nil || len(nodes) == 0 {
		writeError(w, http.StatusNotFound, "nodes not found")
		return
	}
	var output string
	switch contentType {
	case "subscription":
		output, err = protocol.Subscription(nodes)
	case "nodes":
		output, err = protocol.FullContent(nodes)
	default:
		writeError(w, http.StatusBadRequest, "invalid content type")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=60")
	_, _ = io.WriteString(w, output)
}

func (s *Server) qr(w http.ResponseWriter, r *http.Request) {
	data := r.URL.Query().Get("data")
	if data == "" || len(data) > 16*1024 {
		writeError(w, http.StatusBadRequest, "二维码内容为空或过长")
		return
	}
	png, err := qrcode.Encode(data, qrcode.Medium, 384)
	if err != nil {
		writeError(w, http.StatusBadRequest, "二维码内容过长")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

func (s *Server) shareNodes(ctx context.Context, kind string, id int64) ([]model.Node, error) {
	switch kind {
	case "node":
		node, err := s.store.Node(ctx, id)
		if err != nil {
			return nil, err
		}
		return []model.Node{node}, nil
	case "group":
		return s.store.Nodes(ctx, id)
	default:
		return nil, errors.New("invalid share kind")
	}
}

func (s *Server) signedURL(kind string, id int64, content string, exp int64) string {
	message := shareMessage(kind, id, content, exp)
	values := url.Values{
		"kind":    []string{kind},
		"id":      []string{strconv.FormatInt(id, 10)},
		"content": []string{content},
		"exp":     []string{strconv.FormatInt(exp, 10)},
		"sig":     []string{s.signer.Sign(message)},
	}
	return strings.TrimRight(s.cfg.BaseURL, "/") + "/s?" + values.Encode()
}

func shareMessage(kind string, id int64, content string, exp int64) string {
	return fmt.Sprintf("kind=%s&id=%d&content=%s&exp=%d", kind, id, content, exp)
}

func (s *Server) fetchSubscription(ctx context.Context, rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return "", errors.New("订阅 URL 必须是有效的 HTTP/HTTPS 地址")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "SubManager/0.1")
	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("获取订阅失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("订阅返回 HTTP %d", response.StatusCode)
	}
	reader := io.LimitReader(response.Body, 8<<20)
	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		user, ok, err := s.store.SessionUser(auth.Digest(cookie.Value))
		if err != nil {
			serverError(w, err)
			return
		}
		if !ok {
			writeError(w, http.StatusUnauthorized, "登录已过期")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey{}, user)))
	})
}

func (s *Server) recoverAndLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic: %v", recovered)
				writeError(w, http.StatusInternalServerError, "服务器内部错误")
			}
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}()
		next.ServeHTTP(w, r)
	})
}

type userContextKey struct{}

func currentUser(r *http.Request) model.User {
	user, _ := r.Context().Value(userContextKey{}).(model.User)
	return user
}

func normalizeNode(node *model.Node) {
	node.Name = strings.TrimSpace(node.Name)
	node.Server = strings.TrimSpace(node.Server)
	node.Protocol = strings.ToLower(strings.TrimSpace(node.Protocol))
	if node.Protocol == "socks" {
		node.Protocol = "socks5"
	}
	if node.Protocol == "vless" {
		if node.Encryption == "" {
			node.Encryption = "none"
		}
		if node.Network == "" {
			node.Network = "raw"
		}
		if node.Security == "" {
			node.Security = "none"
		}
	}
	if node.Extra == nil {
		node.Extra = map[string]string{}
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "请求 JSON 无效: "+err.Error())
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("server error: %v", err)
	writeError(w, http.StatusInternalServerError, "服务器内部错误")
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "无效 ID")
		return 0, false
	}
	return id, true
}

func friendlyDBError(err error) string {
	if strings.Contains(strings.ToLower(err.Error()), "unique") {
		return "名称已存在"
	}
	return err.Error()
}

func safeHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
			if err != nil {
				return nil, err
			}
			for _, ip := range addresses {
				if isPublicIP(ip) {
					return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				}
			}
			return nil, errors.New("订阅地址解析到了私网或保留地址")
		},
		TLSHandshakeTimeout: 10 * time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if request.URL.Scheme != "http" && request.URL.Scheme != "https" {
			return errors.New("redirect scheme is not allowed")
		}
		return nil
	}
	return client
}

func isPublicIP(ip net.IP) bool {
	return ip != nil &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsUnspecified() &&
		!ip.IsMulticast()
}

func spaHandler() http.Handler {
	dist, err := fs.Sub(webassets.Dist, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if file, err := dist.Open(path); err == nil {
				_ = file.Close()
				files.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			http.Error(w, "frontend not built", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
}
