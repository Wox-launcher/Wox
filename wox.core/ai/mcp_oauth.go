package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"wox/common"
	"wox/util"
	"wox/util/browser"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

const mcpOAuthCallbackAddr = "127.0.0.1:19788"
const mcpOAuthCallbackPath = "/mcp/oauth/callback"

var mcpOAuthTokens = util.NewHashMap[string, string]()

type mcpOAuthTransport struct {
	config   common.AIChatMCPServerConfig
	resource string
	mu       sync.Mutex
}

func newMCPOAuthTransport(_ context.Context, config common.AIChatMCPServerConfig) *mcpOAuthTransport {
	return &mcpOAuthTransport{config: config, resource: strings.TrimSpace(config.Url)}
}

func (t *mcpOAuthTransport) cachedToken() string {
	if token, ok := mcpOAuthTokens.Load(t.config.Name); ok {
		return token
	}
	return ""
}

func (t *mcpOAuthTransport) authorize(ctx context.Context, failed *http.Request, resp *http.Response) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if token := t.cachedToken(); token != "" {
		return token, nil
	}

	resource := t.resource
	if resource == "" && failed != nil {
		resource = failed.URL.String()
	}
	metadataURL := resourceMetadataURL(resp, resource)
	if resp != nil && resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
	}

	authServer, err := discoverMCPAuthServer(ctx, metadataURL, resource)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("MCP OAuth discovery failed for %s: %s", t.config.Name, err))
		return "", err
	}

	token, err := t.clientCredentialsToken(ctx, authServer)
	if err == nil && token != "" {
		mcpOAuthTokens.Store(t.config.Name, token)
		return token, nil
	}

	token, err = t.authorizationCodeToken(ctx, authServer)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("MCP OAuth login failed for %s: %s", t.config.Name, err))
		return "", err
	}
	mcpOAuthTokens.Store(t.config.Name, token)
	return token, nil
}

func (t *mcpOAuthTransport) clientCredentialsToken(ctx context.Context, authServer *mcpAuthServerMeta) (string, error) {
	if t.config.Auth == nil || strings.TrimSpace(t.config.Auth.ClientSecret) == "" || authServer.TokenEndpoint == "" {
		return "", fmt.Errorf("client credentials are not configured")
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", interpolateMCPValue(t.config.Auth.ClientID))
	form.Set("client_secret", interpolateMCPValue(t.config.Auth.ClientSecret))
	if scopes := trimStringList(t.config.Auth.Scopes); len(scopes) > 0 {
		form.Set("scope", strings.Join(scopes, " "))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authServer.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("client credentials grant failed: %s", strings.TrimSpace(string(body)))
	}
	var token oauth2.Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return "", err
	}
	if token.AccessToken == "" {
		return "", fmt.Errorf("client credentials grant returned no access token")
	}
	return token.AccessToken, nil
}

func (t *mcpOAuthTransport) authorizationCodeToken(ctx context.Context, authServer *mcpAuthServerMeta) (string, error) {
	if t.config.Auth == nil || strings.TrimSpace(t.config.Auth.ClientID) == "" {
		return "", fmt.Errorf("OAuth CLIENT_ID is required")
	}
	if authServer.AuthorizationEndpoint == "" || authServer.TokenEndpoint == "" {
		return "", fmt.Errorf("authorization server metadata is incomplete")
	}

	redirectURL := "http://" + mcpOAuthCallbackAddr + mcpOAuthCallbackPath
	cfg := &oauth2.Config{
		ClientID:     interpolateMCPValue(t.config.Auth.ClientID),
		ClientSecret: interpolateMCPValue(t.config.Auth.ClientSecret),
		Endpoint: oauth2.Endpoint{
			AuthURL:  authServer.AuthorizationEndpoint,
			TokenURL: authServer.TokenEndpoint,
		},
		RedirectURL: redirectURL,
		Scopes:      trimStringList(t.config.Auth.Scopes),
	}

	state := uuid.NewString()
	authURL := cfg.AuthCodeURL(state, oauth2.AccessTypeOffline)
	code, err := waitMCPOAuthCode(ctx, authURL, state)
	if err != nil {
		return "", err
	}
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return "", err
	}
	if token.AccessToken == "" {
		return "", fmt.Errorf("authorization code grant returned no access token")
	}
	return token.AccessToken, nil
}

func waitMCPOAuthCode(ctx context.Context, authURL, state string) (string, error) {
	listener, err := net.Listen("tcp", mcpOAuthCallbackAddr)
	if err != nil {
		return "", fmt.Errorf("MCP OAuth callback port %s is in use; register http://%s%s as the redirect URL", mcpOAuthCallbackAddr, mcpOAuthCallbackAddr, mcpOAuthCallbackPath)
	}
	defer listener.Close()

	type result struct {
		code string
		err  error
	}
	results := make(chan result, 1)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != mcpOAuthCallbackPath {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("state"); got != state {
			http.Error(w, "invalid OAuth state", http.StatusBadRequest)
			results <- result{err: fmt.Errorf("invalid OAuth state")}
			return
		}
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			http.Error(w, errMsg, http.StatusBadRequest)
			results <- result{err: fmt.Errorf("OAuth error: %s", errMsg)}
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing authorization code", http.StatusBadRequest)
			results <- result{err: fmt.Errorf("missing authorization code")}
			return
		}
		_, _ = io.WriteString(w, "Wox MCP authorization finished. You can close this window.")
		results <- result{code: code}
	})
	go func() {
		_ = server.Serve(listener)
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := browser.OpenURL(authURL, ""); err != nil {
		return "", fmt.Errorf("open OAuth browser: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("MCP OAuth: opened browser for %s", authURL))

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case item := <-results:
		return item.code, item.err
	}
}

type mcpAuthServerMeta struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

type mcpProtectedResourceMeta struct {
	AuthorizationServers []string `json:"authorization_servers"`
}

func resourceMetadataURL(resp *http.Response, resource string) string {
	if resp != nil {
		if header := resp.Header.Get("WWW-Authenticate"); header != "" {
			if metadata := parseResourceMetadata(header); metadata != "" {
				return metadata
			}
		}
	}
	if resource == "" {
		return ""
	}
	parsed, err := url.Parse(resource)
	if err != nil {
		return ""
	}
	parsed.Path = "/.well-known/oauth-protected-resource"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func parseResourceMetadata(header string) string {
	lower := strings.ToLower(header)
	idx := strings.Index(lower, "resource_metadata=")
	if idx < 0 {
		return ""
	}
	value := strings.TrimSpace(header[idx+len("resource_metadata="):])
	value = strings.Trim(value, `"`)
	if comma := strings.Index(value, ","); comma >= 0 {
		value = value[:comma]
	}
	return strings.Trim(value, `"`)
}

func discoverMCPAuthServer(ctx context.Context, metadataURL, resource string) (*mcpAuthServerMeta, error) {
	issuer := ""
	if metadataURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
		if err == nil {
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					var resourceMeta mcpProtectedResourceMeta
					if err := json.NewDecoder(resp.Body).Decode(&resourceMeta); err == nil && len(resourceMeta.AuthorizationServers) > 0 {
						issuer = resourceMeta.AuthorizationServers[0]
					}
				}
			}
		}
	}
	if issuer == "" && resource != "" {
		parsed, err := url.Parse(resource)
		if err == nil {
			issuer = parsed.Scheme + "://" + parsed.Host
		}
	}
	if issuer == "" {
		return nil, fmt.Errorf("could not discover OAuth authorization server")
	}

	candidates := []string{
		strings.TrimRight(issuer, "/") + "/.well-known/oauth-authorization-server",
		strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration",
	}
	for _, candidate := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
		if err != nil {
			continue
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		var meta mcpAuthServerMeta
		decodeErr := json.NewDecoder(resp.Body).Decode(&meta)
		resp.Body.Close()
		if decodeErr != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}
		if meta.AuthorizationEndpoint != "" || meta.TokenEndpoint != "" {
			return &meta, nil
		}
	}
	return nil, fmt.Errorf("authorization server metadata not found at %s", issuer)
}
