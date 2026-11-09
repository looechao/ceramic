package mastodon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const requestTimeout = 25 * time.Second

type Client struct {
	requestContext context.Context
	mu             sync.Mutex
	origin, path   string
	session        *Session
	pending        *pendingLogin
	loginError     error
	formatMessage  func(Message) string
	version        uint64
	client         *http.Client
	open           func(string) error
}

func New(path string, formatters ...func(Message) string) *Client {
	a := &Client{path: path, client: &http.Client{Timeout: requestTimeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, open: openBrowser}
	if len(formatters) > 0 {
		a.formatMessage = formatters[0]
	}
	b, err := os.ReadFile(path)
	if err == nil {
		var s Session
		if json.Unmarshal(b, &s) == nil && s.Token != "" {
			if _, err := NormalizeInstance(s.Instance); err == nil {
				a.session = &s
			}
		}
	}
	return a
}

func (a *Client) request(method, endpoint, token string, form url.Values, out any) error {
	_, err := a.requestHeaders(method, endpoint, token, form, out)
	return err
}

func (a *Client) requestHeaders(method, endpoint, token string, form url.Values, out any) (http.Header, error) {
	return a.requestExtra(method, endpoint, token, form, out, nil)
}

func (a *Client) requestExtra(method, endpoint, token string, form url.Values, out any, headers http.Header) (http.Header, error) {
	ctx := a.requestContext
	if ctx == nil {
		ctx = context.Background()
	}
	return a.requestExtraContext(ctx, method, endpoint, token, form, out, headers)
}

// ForSession creates a request client bound to the captured session
// and operation lifetime. It shares the HTTP transport, not mutable login state.
func (a *Client) ForSession(ctx context.Context, session *Session) *Client {
	return &Client{session: session, client: a.client, requestContext: ctx, formatMessage: a.formatMessage}
}

func (a *Client) requestExtraContext(ctx context.Context, method, endpoint, token string, form url.Values, out any, headers http.Header) (http.Header, error) {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	for key, values := range headers {
		req.Header[key] = values
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
		return nil, MessageConnectionFailed
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		switch res.StatusCode {
		case 401:
			return nil, MessageSessionExpired
		case 403:
			return nil, MessageAccessDenied
		case 429:
			return nil, MessageRateLimited
		}
		response := HTTPError{Status: res.StatusCode}
		if res.StatusCode == http.StatusUnprocessableEntity {
			var body struct {
				Error string `json:"error"`
			}
			if json.NewDecoder(io.LimitReader(res.Body, 8192)).Decode(&body) == nil {
				response.Detail = strings.TrimSpace(body.Error)
			}
		}
		return nil, response
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(out); err != nil {
		return nil, MessageInvalidResponse
	}
	return res.Header, nil
}

func (a *Client) Timeline(cursor string) ([]Status, error) {
	a.mu.Lock()
	s := a.session
	a.mu.Unlock()
	if s == nil {
		return nil, MessageLoginRequired
	}
	q := url.Values{"limit": {"30"}}
	if cursor != "" {
		q.Set("max_id", cursor)
	}
	var posts []Status
	if err := a.request("GET", s.Instance+"/api/v1/timelines/home?"+q.Encode(), s.Token, nil, &posts); err != nil {
		return nil, err
	}
	return posts, nil
}

// readAPI keeps all account and search requests on the authenticated instance.
func (a *Client) readAPI(path string, q url.Values, out any) error {
	a.mu.Lock()
	s := a.session
	a.mu.Unlock()
	if s == nil {
		return MessageLoginRequired
	}
	return a.request("GET", s.Instance+path+"?"+q.Encode(), s.Token, nil, out)
}

func (a *Client) Profile(id string) (Account, error) {
	var result Account
	err := a.readAPI("/api/v1/accounts/"+url.PathEscape(id), nil, &result)
	return result, err
}

func (a *Client) AccountPosts(id, cursor string) ([]Status, error) {
	q := url.Values{"limit": {"30"}, "exclude_replies": {"true"}}
	if cursor != "" {
		q.Set("max_id", cursor)
	}
	var result []Status
	err := a.readAPI("/api/v1/accounts/"+url.PathEscape(id)+"/statuses", q, &result)
	return result, err
}

// conversation reads the focal post and its server-provided context.
func (a *Client) Conversation(id string) ([]Status, error) {
	path := "/api/v1/statuses/" + url.PathEscape(id)
	var focal Status
	if err := a.readAPI(path, nil, &focal); err != nil {
		return nil, err
	}
	var context struct {
		Ancestors   []Status `json:"ancestors"`
		Descendants []Status `json:"descendants"`
	}
	if err := a.readAPI(path+"/context", nil, &context); err != nil {
		return nil, err
	}
	posts := append(context.Ancestors, focal)
	return append(posts, context.Descendants...), nil
}

// StreamUser discovers the streaming host without credentials, then opens the
// notifications SSE endpoint with bearer authentication. Cancellation covers
// both discovery and the long-lived connection.
func (a *Client) StreamUser(ctx context.Context, onEvent func(event string, data []byte)) error {
	a.mu.Lock()
	s := a.session
	a.mu.Unlock()
	if s == nil {
		return MessageLoginRequired
	}
	discovery, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Instance+"/api/v1/streaming", nil)
	if err != nil {
		return err
	}
	probe := *a.client
	probe.Timeout = requestTimeout
	probe.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := probe.Do(discovery)
	if err != nil {
		return err
	}
	response.Body.Close()
	endpoint := discovery.URL
	switch response.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		endpoint, err = response.Location()
		if err != nil {
			return err
		}
	case http.StatusNotFound:
		// No separate streaming host configured.
	default:
		return HTTPError{Status: response.StatusCode}
	}
	if endpoint.Host == "" || endpoint.User != nil ||
		(endpoint.Scheme != "https" && !(endpoint.Scheme == "http" && discovery.URL.Scheme == "http" && endpoint.Host == discovery.URL.Host)) {
		return MessageInvalidStreamURL
	}
	endpoint.Path, endpoint.RawPath = "/api/v1/streaming/user/notification", ""
	endpoint.RawQuery, endpoint.Fragment = "", ""
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+s.Token)
	req.Header.Set("User-Agent", "Ceramic/0.1")
	// The regular client times requests out after 25s, which would cut the
	// stream; a stream lives until the server or the app closes it.
	stream := probe
	stream.Timeout = 0
	res, err := stream.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return HTTPError{Status: res.StatusCode}
	}
	if !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		return MessageInvalidStream
	}
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	var event string
	var data []byte
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			if len(data) > 0 {
				onEvent(event, data)
			}
			event, data = "", nil
			continue
		}
		switch {
		case bytes.HasPrefix(line, []byte("event:")):
			event = strings.TrimSpace(string(line[len("event:"):]))
		case bytes.HasPrefix(line, []byte("data:")):
			if len(data) > 0 {
				data = append(data, '\n')
			}
			data = append(data, bytes.TrimPrefix(line[len("data:"):], []byte(" "))...)
			if len(data) > 1<<20 {
				return MessageStreamEventTooLarge
			}
		}
	}
	return scanner.Err()
}

// Notifications reads the account's notification feed, newest first.
func (a *Client) Notifications(cursor string) ([]Notification, error) {
	q := url.Values{"limit": {"30"}}
	if cursor != "" {
		q.Set("max_id", cursor)
	}
	var result []Notification
	err := a.readAPI("/api/v1/notifications", q, &result)
	return result, err
}

func (a *Client) Search(query, kind string, offset int) (SearchResult, error) {
	var result SearchResult
	q := url.Values{"q": {query}, "type": {kind}, "limit": {"20"}, "offset": {fmt.Sprint(offset)}}
	err := a.readAPI("/api/v2/search", q, &result)
	return result, err
}
