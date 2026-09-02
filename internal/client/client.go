// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// apiBasePath is the fixed Application API prefix appended to the endpoint.
	apiBasePath = "api/application"

	defaultTimeout    = 60 * time.Second
	defaultMaxRetries = 3
	defaultUserAgent  = "terraform-provider-penguin"
	backoffBase       = 500 * time.Millisecond
	maxRetryBackoff   = 30 * time.Second
	maxListPages      = 1000
)

// Config configures a Client.
type Config struct {
	// Endpoint is the panel base URL, e.g. https://panel.example.com. The
	// /api/application prefix is appended automatically.
	Endpoint string
	// Token is the application API token (papp_...). Required.
	Token string
	// UserAgent overrides the default User-Agent header.
	UserAgent string
	// Insecure skips TLS certificate verification. Ignored when HTTPClient is set.
	Insecure bool
	// Timeout bounds each individual HTTP request (0 → default). Ignored when
	// HTTPClient is set.
	Timeout time.Duration
	// MaxRetries caps retry attempts for 429/5xx/transient failures. 0 → default,
	// a negative value disables retries.
	MaxRetries int
	// HTTPClient, when set, is used verbatim (Insecure and Timeout are ignored).
	HTTPClient *http.Client
}

// Client talks to the Penguin / Pelican Application API.
type Client struct {
	baseURL    *url.URL
	token      string
	userAgent  string
	httpClient *http.Client
	maxRetries int
}

// New builds a Client from cfg, validating the endpoint and token. The endpoint
// gains an https:// scheme when none is given and the /api/application prefix is
// appended once.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, errors.New("endpoint is required")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("api token is required")
	}

	raw := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing endpoint %q: %w", cfg.Endpoint, err)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("endpoint %q has no host", cfg.Endpoint)
	}
	base := u.JoinPath(apiBasePath)

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = defaultTimeout
		}
		transport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, errors.New("unexpected default transport type")
		}
		transport = transport.Clone()
		if cfg.Insecure {
			// Opt-in only, via the provider's `insecure` attribute.
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		}
		httpClient = &http.Client{Timeout: timeout, Transport: transport}
	}

	retries := cfg.MaxRetries
	switch {
	case retries == 0:
		retries = defaultMaxRetries
	case retries < 0:
		retries = 0
	}

	userAgent := defaultUserAgent
	if strings.TrimSpace(cfg.UserAgent) != "" {
		userAgent = cfg.UserAgent
	}

	return &Client{
		baseURL:    base,
		token:      cfg.Token,
		userAgent:  userAgent,
		httpClient: httpClient,
		maxRetries: retries,
	}, nil
}

// Do issues a single request and returns the raw response body on 2xx, an
// *APIError on a non-2xx response, or a transport error. apiPath is relative to
// /api/application (leading slashes are trimmed).
func (c *Client) Do(ctx context.Context, method, apiPath string, query url.Values, body []byte) ([]byte, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		req, err := c.newRequest(ctx, method, apiPath, query, body)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("%s %s: %w", method, apiPath, err)
			if attempt < c.maxRetries && retryableErr(err) {
				if werr := backoff(ctx, attempt, 0); werr != nil {
					return nil, werr
				}
				continue
			}
			return nil, lastErr
		}

		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("%s %s: reading response: %w", method, apiPath, readErr)
			if attempt < c.maxRetries {
				if werr := backoff(ctx, attempt, 0); werr != nil {
					return nil, werr
				}
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return respBody, nil
		}

		apiErr := parseAPIError(resp.StatusCode, respBody)
		if attempt < c.maxRetries && retryableStatus(resp.StatusCode) {
			floor := parseRetryAfter(resp.Header.Get("Retry-After"))
			if werr := backoff(ctx, attempt, floor); werr != nil {
				return nil, werr
			}
			lastErr = apiErr
			continue
		}
		return nil, apiErr
	}
}

// GetItem GETs a single object and decodes the item envelope.
func (c *Client) GetItem(ctx context.Context, apiPath string, query url.Values) (*Item, error) {
	b, err := c.Do(ctx, http.MethodGet, apiPath, query, nil)
	if err != nil {
		return nil, err
	}
	return decodeItem(b)
}

// CreateItem POSTs body to a collection path and decodes the created item. A
// 2xx with an empty body yields (nil, nil) so the caller can Read to populate.
func (c *Client) CreateItem(ctx context.Context, apiPath string, body any) (*Item, error) {
	payload, err := marshalBody(body)
	if err != nil {
		return nil, err
	}
	b, err := c.Do(ctx, http.MethodPost, apiPath, nil, payload)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, nil
	}
	return decodeItem(b)
}

// UpdateItem PATCHes body to an item path and decodes the returned item. A 2xx
// with an empty body (204) yields (nil, nil) so the caller can Read to refresh.
func (c *Client) UpdateItem(ctx context.Context, apiPath string, body any) (*Item, error) {
	payload, err := marshalBody(body)
	if err != nil {
		return nil, err
	}
	b, err := c.Do(ctx, http.MethodPatch, apiPath, nil, payload)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, nil
	}
	return decodeItem(b)
}

// Post issues a POST for an action endpoint (suspend, reinstall, reset-password,
// role assign/remove) that may return an item, an empty 204, or nothing useful.
func (c *Client) Post(ctx context.Context, apiPath string, body any) (*Item, error) {
	payload, err := marshalBody(body)
	if err != nil {
		return nil, err
	}
	b, err := c.Do(ctx, http.MethodPost, apiPath, nil, payload)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil, nil
	}
	return decodeItem(b)
}

// DeleteItem DELETEs an item path. A 404 is returned as an *APIError so the
// caller may treat an already-absent object as success.
func (c *Client) DeleteItem(ctx context.Context, apiPath string) error {
	_, err := c.Do(ctx, http.MethodDelete, apiPath, nil, nil)
	return err
}

// ListItems GETs every page of a collection, following meta.pagination, and
// returns the concatenated items. Caller-supplied query params (filter, sort,
// include, per_page) are preserved; page is managed here.
func (c *Client) ListItems(ctx context.Context, apiPath string, query url.Values) ([]Item, error) {
	base := cloneValues(query)
	var all []Item
	for page := 1; page <= maxListPages; page++ {
		q := cloneValues(base)
		q.Set("page", strconv.Itoa(page))
		b, err := c.Do(ctx, http.MethodGet, apiPath, q, nil)
		if err != nil {
			return nil, err
		}
		l, err := decodeList(b)
		if err != nil {
			return nil, err
		}
		all = append(all, l.Data...)
		if l.Meta.Pagination.TotalPages <= page {
			break
		}
	}
	return all, nil
}

func (c *Client) newRequest(ctx context.Context, method, apiPath string, query url.Values, body []byte) (*http.Request, error) {
	u := c.baseURL.JoinPath(splitPath(apiPath)...)
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("building %s request: %w", method, err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

func marshalBody(body any) ([]byte, error) {
	switch v := body.(type) {
	case nil:
		return nil, nil
	case []byte:
		return v, nil
	case json.RawMessage:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		return b, nil
	}
}

func cloneValues(v url.Values) url.Values {
	out := url.Values{}
	for k, vals := range v {
		cp := make([]string, len(vals))
		copy(cp, vals)
		out[k] = cp
	}
	return out
}

func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || (code >= 500 && code <= 599)
}

func retryableErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}

func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

func backoff(ctx context.Context, attempt int, floor time.Duration) error {
	wait := time.Duration(math.Pow(2, float64(attempt))) * backoffBase
	if wait > maxRetryBackoff {
		wait = maxRetryBackoff
	}
	if floor > wait {
		wait = floor
	}
	if wait > maxRetryBackoff {
		wait = maxRetryBackoff
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("retry backoff interrupted: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}
