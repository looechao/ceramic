package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const requestTimeout = 25 * time.Second
const scopes = "read:accounts read:statuses read:search"

type session struct {
	Instance string          `json:"instance"`
	Token    string          `json:"token"`
	Scopes   string          `json:"scopes,omitempty"`
	Account  json.RawMessage `json:"account"`
}
type pendingLogin struct {
	Instance, ClientID, Secret, State, Redirect string
	Expires                                     time.Time
	Version                                     uint64
}
type application struct {
	mu           sync.Mutex
	origin, path string
	session      *session
	pending      *pendingLogin
	loginError   string
	version      uint64
	client       *http.Client
	open         func(string) error
}

func randomKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func newApp(path string) *application {
	a := &application{path: path, client: &http.Client{Timeout: requestTimeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, open: openBrowser}
	b, err := os.ReadFile(path)
	if err == nil {
		var s session
		if json.Unmarshal(b, &s) == nil && s.Token != "" {
			if _, err := normalizeInstance(s.Instance); err == nil {
				a.session = &s
			}
		}
	}
	return a
}
func normalizeInstance(raw string) (string, error) {
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
func (a *application) request(method, endpoint, token string, form url.Values, out any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Ceramic/0.1")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.client.Do(req)
	if err != nil {
		return errors.New("无法连接实例，请检查域名和网络后重试")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		switch res.StatusCode {
		case 401:
			return errors.New("登录已失效，请退出后重新登录")
		case 403:
			return errors.New("实例拒绝访问，请检查账号权限")
		case 429:
			return errors.New("请求过于频繁，请稍后重试")
		}
		return fmt.Errorf("实例返回 HTTP %d，请稍后重试", res.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(out); err != nil {
		return errors.New("实例返回了无法识别的数据，请确认它支持 Mastodon API")
	}
	return nil
}

// The local HTTP listener only receives the OAuth redirect. All UI is native Go.
func (a *application) routes() http.Handler {
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
func (a *application) snapshot() (*session, bool, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pending != nil && time.Now().After(a.pending.Expires) {
		a.pending = nil
		a.loginError = "授权已超时，请重新连接"
	}
	return a.session, a.pending != nil, a.loginError
}
func (a *application) login(instance string) error {
	instance, err := normalizeInstance(instance)
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
	if err = a.request("POST", instance+"/api/v1/apps", "", url.Values{"client_name": {"Ceramic"}, "redirect_uris": {redirect}, "scopes": {scopes}}, &app); err != nil {
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
	authURL := instance + "/oauth/authorize?" + url.Values{"response_type": {"code"}, "client_id": {p.ClientID}, "redirect_uri": {redirect}, "scope": {scopes}, "state": {p.State}}.Encode()
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
func (a *application) callback(w http.ResponseWriter, r *http.Request) {
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
	}
	if err == nil {
		err = a.request("POST", p.Instance+"/oauth/token", "", url.Values{"grant_type": {"authorization_code"}, "client_id": {p.ClientID}, "client_secret": {p.Secret}, "redirect_uri": {p.Redirect}, "code": {r.URL.Query().Get("code")}}, &token)
	}
	if err == nil && token.AccessToken == "" {
		err = errors.New("实例未返回有效的登录令牌")
	}
	var account json.RawMessage
	if err == nil {
		err = a.request("GET", p.Instance+"/api/v1/accounts/verify_credentials", token.AccessToken, nil, &account)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if p.Version != a.version {
		http.Error(w, "授权已取消", 400)
		return
	}
	if err == nil {
		s := &session{Instance: p.Instance, Token: token.AccessToken, Account: account, Scopes: scopes}
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
func saveSession(path string, s *session) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.New("无法保存登录状态")
	}
	b, _ := json.Marshal(s)
	f, err := os.CreateTemp(filepath.Dir(path), ".session-*")
	if err != nil {
		return errors.New("无法保存登录状态")
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (a *application) logout() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := os.Remove(a.path); err != nil && !os.IsNotExist(err) {
		return errors.New("无法清除登录状态，请重试")
	}
	a.version++
	a.session = nil
	a.pending = nil
	a.loginError = ""
	return nil
}
func (a *application) timeline(cursor string) ([]status, error) {
	a.mu.Lock()
	s := a.session
	a.mu.Unlock()
	if s == nil {
		return nil, errors.New("请先登录")
	}
	q := url.Values{"limit": {"30"}}
	if cursor != "" {
		q.Set("max_id", cursor)
	}
	var posts []status
	if err := a.request("GET", s.Instance+"/api/v1/timelines/home?"+q.Encode(), s.Token, nil, &posts); err != nil {
		return nil, err
	}
	return posts, nil
}

// readAPI keeps all account and search requests on the authenticated instance.
func (a *application) readAPI(path string, q url.Values, out any) error {
	a.mu.Lock()
	s := a.session
	a.mu.Unlock()
	if s == nil {
		return errors.New("请先登录")
	}
	return a.request("GET", s.Instance+path+"?"+q.Encode(), s.Token, nil, out)
}
func (a *application) profile(id string) (account, error) {
	var result account
	err := a.readAPI("/api/v1/accounts/"+url.PathEscape(id), nil, &result)
	return result, err
}
func (a *application) accountPosts(id, cursor string) ([]status, error) {
	q := url.Values{"limit": {"30"}, "exclude_replies": {"true"}}
	if cursor != "" {
		q.Set("max_id", cursor)
	}
	var result []status
	err := a.readAPI("/api/v1/accounts/"+url.PathEscape(id)+"/statuses", q, &result)
	return result, err
}

type searchResult struct {
	Accounts []account `json:"accounts"`
	Statuses []status  `json:"statuses"`
}

func (a *application) search(query, kind string, offset int) (searchResult, error) {
	var result searchResult
	q := url.Values{"q": {query}, "type": {kind}, "limit": {"20"}, "offset": {fmt.Sprint(offset)}}
	err := a.readAPI("/api/v2/search", q, &result)
	return result, err
}
func (a *application) cancelLogin() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.version++
	a.pending = nil
	a.loginError = ""
}

// conversation reads the focal post and its server-provided context.
func (a *application) conversation(id string) ([]status, error) {
	path := "/api/v1/statuses/" + url.PathEscape(id)
	var focal status
	if err := a.readAPI(path, nil, &focal); err != nil {
		return nil, err
	}
	var context struct {
		Ancestors   []status `json:"ancestors"`
		Descendants []status `json:"descendants"`
	}
	if err := a.readAPI(path+"/context", nil, &context); err != nil {
		return nil, err
	}
	posts := append(context.Ancestors, focal)
	return append(posts, context.Descendants...), nil
}
