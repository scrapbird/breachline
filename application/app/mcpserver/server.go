package mcpserver

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// commandTimeout bounds how long a UI-dispatched tool waits for the frontend.
const commandTimeout = 30 * time.Second

// Server hosts the MCP endpoint. It is not bound to the frontend directly;
// BridgeService is the bound surface. Lifecycle is driven by ApplyMCPConfig.
type Server struct {
	bridge  AppBridge
	version string

	ctx    context.Context
	mcp    *mcp.Server

	mu      sync.Mutex
	enabled bool
	address string
	token   string
	running bool
	lastErr string
	httpSrv *http.Server

	// pending correlates UI command requests with their frontend replies.
	pendingMu sync.Mutex
	pending   map[string]chan cmdResult
}

type cmdResult struct {
	result json.RawMessage
	errMsg string
}

// Status is the current MCP server state, surfaced in Settings.
type Status struct {
	Enabled bool   `json:"enabled"`
	Running bool   `json:"running"`
	Address string `json:"address"`
	Token   string `json:"token"`
	Error   string `json:"error,omitempty"`
}

// New creates the server and registers its tools. It does not start listening;
// call ApplyMCPConfig (usually from settings) to start or stop it.
func New(bridge AppBridge, version string) *Server {
	s := &Server{
		bridge:  bridge,
		version: version,
		pending: make(map[string]chan cmdResult),
	}
	s.mcp = mcp.NewServer(&mcp.Implementation{
		Name:    "breachline",
		Title:   "BreachLine",
		Version: version,
	}, nil)
	s.registerTools()
	return s
}

// Startup stores the Wails context (used to emit UI commands).
func (s *Server) Startup(ctx context.Context) {
	s.ctx = ctx
}

// ApplyMCPConfig starts, stops, or restarts the server to match the given
// settings. Safe to call repeatedly. Implements settings.MCPController.
func (s *Server) ApplyMCPConfig(enabled bool, address, token string) {
	s.mu.Lock()
	// Nothing changed and already in the desired running state.
	if enabled == s.enabled && address == s.address && token == s.token && enabled == s.running && s.lastErr == "" {
		s.mu.Unlock()
		return
	}
	s.enabled = enabled
	s.address = address
	s.token = token
	s.mu.Unlock()

	s.stop()
	if enabled {
		s.start()
	}
}

func (s *Server) start() {
	s.mu.Lock()
	address := s.address
	s.mu.Unlock()

	ln, err := net.Listen("tcp", address)
	if err != nil {
		s.mu.Lock()
		s.running = false
		s.lastErr = fmt.Sprintf("could not listen on %s: %v", address, err)
		s.mu.Unlock()
		s.logf("MCP server failed to start: %v", err)
		return
	}

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcp }, nil)
	mux := http.NewServeMux()
	mux.Handle("/", s.authMiddleware(handler))
	srv := &http.Server{Handler: mux}

	s.mu.Lock()
	s.httpSrv = srv
	s.running = true
	s.lastErr = ""
	s.mu.Unlock()

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.mu.Lock()
			s.running = false
			s.lastErr = err.Error()
			s.mu.Unlock()
		}
	}()
	s.logf("MCP server listening on %s", address)
}

func (s *Server) stop() {
	s.mu.Lock()
	srv := s.httpSrv
	s.httpSrv = nil
	s.running = false
	s.mu.Unlock()
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

// Status returns the current server state.
func (s *Server) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Status{
		Enabled: s.enabled,
		Running: s.running,
		Address: s.address,
		Token:   s.token,
		Error:   s.lastErr,
	}
}

// authMiddleware enforces the bearer token (when set) and refuses non-loopback
// hosts as a second line of defence behind the loopback bind.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		token := s.token
		s.mu.Unlock()
		if token != "" {
			auth := strings.TrimSpace(r.Header.Get("Authorization"))
			const p = "Bearer "
			ok := strings.HasPrefix(auth, p) &&
				subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(auth, p)), []byte(token)) == 1
			if !ok {
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// runCommand dispatches a state-changing action to the frontend and waits for
// its reply, so the visible window performs (and reflects) the action.
func (s *Server) runCommand(ctx context.Context, action string, params any) (json.RawMessage, error) {
	if s.ctx == nil {
		return nil, errors.New("app is not ready yet")
	}
	id := newID()
	ch := make(chan cmdResult, 1)
	s.pendingMu.Lock()
	s.pending[id] = ch
	s.pendingMu.Unlock()
	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
	}()

	wruntime.EventsEmit(s.ctx, "mcp:command", map[string]any{
		"id":     id,
		"action": action,
		"params": params,
	})

	select {
	case res := <-ch:
		if res.errMsg != "" {
			return nil, errors.New(res.errMsg)
		}
		return res.result, nil
	case <-time.After(commandTimeout):
		return nil, fmt.Errorf("timed out waiting for the app window to handle %q", action)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// resolve delivers a frontend reply to the waiting runCommand call.
func (s *Server) resolve(id string, result json.RawMessage, errMsg string) {
	s.pendingMu.Lock()
	ch := s.pending[id]
	s.pendingMu.Unlock()
	if ch != nil {
		ch <- cmdResult{result: result, errMsg: errMsg}
	}
}

func (s *Server) logf(format string, args ...any) {
	if s.ctx != nil {
		wruntime.LogInfof(s.ctx, "[mcp] "+format, args...)
	}
}

func newID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// BridgeService is the Wails-bound surface the frontend uses to reply to MCP
// commands and to read server status for the Settings panel.
type BridgeService struct {
	srv *Server
}

// NewBridgeService wraps a Server for binding to the frontend.
func NewBridgeService(srv *Server) *BridgeService {
	return &BridgeService{srv: srv}
}

// MCPResolve delivers the result of a UI command back to the waiting tool call.
// resultJSON is the action's structured result; errMsg is non-empty on failure.
func (b *BridgeService) MCPResolve(requestID string, resultJSON string, errMsg string) {
	b.srv.resolve(requestID, json.RawMessage(resultJSON), errMsg)
}

// MCPGetStatus returns the current server status for the Settings panel.
func (b *BridgeService) MCPGetStatus() Status {
	return b.srv.Status()
}
