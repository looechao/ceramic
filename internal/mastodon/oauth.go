package mastodon

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const Scopes = "read write follow push"

type pendingLogin struct {
	Instance, ClientID, Secret, State, Redirect string
	Expires                                     time.Time
	Version                                     uint64
}

func randomKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func NormalizeInstance(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("请输入实例域名，例如 mastodon.social，不要输入账号或页面地址")
	}
	return "https://" + u.Host, nil
}

func openBrowser(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return errors.New("链接无效")
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", raw).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", raw).Run()
	default:
		return exec.Command("xdg-open", raw).Run()
	}
}

// The local HTTP listener only receives the OAuth redirect. All UI is native Go.
func (a *Client) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /oauth/callback", a.callback)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin, _ := url.Parse(a.origin)
		if r.Host != origin.Host {
			http.Error(w, "Invalid host", 403)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		mux.ServeHTTP(w, r)
	})
}

// StartCallbackServer listens on a loopback port for the OAuth redirect and
// returns a function that stops it.
func (a *Client) StartCallbackServer() (func() error, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	a.origin = "http://" + listener.Addr().String()
	server := &http.Server{Handler: a.routes(), ReadHeaderTimeout: requestTimeout}
	go server.Serve(listener)
	return server.Close, nil
}

func (a *Client) Snapshot() (*Session, bool, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pending != nil && time.Now().After(a.pending.Expires) {
		a.pending = nil
		a.loginError = "授权已超时，请重新连接"
	}
	return a.session, a.pending != nil, a.loginError
}

func (a *Client) Login(instance string) error {
	instance, err := NormalizeInstance(instance)
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.version++
	version := a.version
	a.loginError = ""
	a.mu.Unlock()
	redirect := a.origin + "/oauth/callback"
	var app struct {
		ID     string `json:"client_id"`
		Secret string `json:"client_secret"`
	}
	if err = a.request("POST", instance+"/api/v1/apps", "", url.Values{"client_name": {"Ceramic"}, "redirect_uris": {redirect}, "scopes": {Scopes}}, &app); err != nil {
		return err
	}
	if app.ID == "" || app.Secret == "" {
		return errors.New("实例未返回有效的应用凭据")
	}
	p := &pendingLogin{Instance: instance, ClientID: app.ID, Secret: app.Secret, State: randomKey(), Redirect: redirect, Expires: time.Now().Add(10 * time.Minute)}
	a.mu.Lock()
	if version != a.version {
		a.mu.Unlock()
		return errors.New("授权已取消")
	}
	p.Version = version
	a.pending = p
	a.loginError = ""
	a.mu.Unlock()
	authURL := instance + "/oauth/authorize?" + url.Values{"response_type": {"code"}, "client_id": {p.ClientID}, "redirect_uri": {redirect}, "scope": {Scopes}, "state": {p.State}}.Encode()
	if err = a.open(authURL); err != nil {
		a.mu.Lock()
		if a.pending == p {
			a.pending = nil
		}
		a.mu.Unlock()
		return errors.New("无法打开浏览器，请检查系统默认浏览器")
	}
	return nil
}

func (a *Client) callback(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	p := a.pending
	if p == nil || time.Now().After(p.Expires) || subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(p.State)) != 1 {
		a.mu.Unlock()
		http.Error(w, "授权已过期或无效，请返回 Ceramic 重试。", 400)
		return
	}
	// Consume the state before exchanging the code, preventing callback replay.
	a.pending = nil
	a.mu.Unlock()
	var err error
	if r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
		err = errors.New("授权已取消，请重新连接")
	}
	var token struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
	}
	if err == nil {
		err = a.request("POST", p.Instance+"/oauth/token", "", url.Values{"grant_type": {"authorization_code"}, "client_id": {p.ClientID}, "client_secret": {p.Secret}, "redirect_uri": {p.Redirect}, "code": {r.URL.Query().Get("code")}}, &token)
	}
	if err == nil && token.AccessToken == "" {
		err = errors.New("实例未返回有效的登录令牌")
	}
	var me json.RawMessage
	if err == nil {
		err = a.request("GET", p.Instance+"/api/v1/accounts/verify_credentials", token.AccessToken, nil, &me)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if p.Version != a.version {
		http.Error(w, "授权已取消", 400)
		return
	}
	if err == nil {
		granted := token.Scope
		if granted == "" {
			granted = Scopes
		}
		s := &Session{Instance: p.Instance, Token: token.AccessToken, Account: me, Scopes: granted}
		err = saveSession(a.path, s)
		if err == nil {
			a.session = s
			a.loginError = ""
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if err != nil {
		a.loginError = err.Error()
		w.WriteHeader(400)
		fmt.Fprint(w, "登录未完成，请返回 Ceramic 查看提示。")
		return
	}
	fmt.Fprint(w, "登录成功。可以关闭这个页面，回到 Ceramic 阅读时间线。")
}

func (a *Client) CancelLogin() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.version++
	a.pending = nil
	a.loginError = ""
}
