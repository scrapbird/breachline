package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"breachline/app"
	"breachline/app/interfaces"
	"breachline/app/settings"

	"github.com/scrapbird/breachline/infra/sync-api/src/api"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	// BaseURL is the base URL for the sync API
	BaseURL = "https://u9trlagch8.execute-api.ap-southeast-2.amazonaws.com/v1"
)

// MenuUpdater is an interface for updating menu items
type MenuUpdater interface {
	UpdateSyncMenuLabel(label string)
}

var menuUpdater MenuUpdater

// SetMenuUpdater sets the menu updater
func SetMenuUpdater(updater MenuUpdater) {
	menuUpdater = updater
}

// SyncService manages sync operations and authentication
type SyncService struct {
	ctx             context.Context
	client          *http.Client
	refreshingToken bool // Prevents concurrent refresh attempts
}

// NewSyncService creates a new sync service
func NewSyncService() *SyncService {
	return &SyncService{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		refreshingToken: false,
	}
}

// Startup receives the Wails context
func (s *SyncService) Startup(ctx context.Context) {
	s.ctx = ctx
	// Update menu label based on current login state
	s.UpdateMenuLabel()
}

// UpdateMenuLabel updates the sync menu item label based on login state
func (s *SyncService) UpdateMenuLabel() {
	if menuUpdater != nil {
		if s.IsLoggedIn() {
			menuUpdater.UpdateSyncMenuLabel("Logout of Sync")
		} else {
			menuUpdater.UpdateSyncMenuLabel("Login to Sync")
		}
	}
}

// RequestPIN requests a PIN to be sent to the user's email
func (s *SyncService) RequestPIN(license string) (*api.RequestPINResponse, error) {
	if license == "" {
		return nil, fmt.Errorf("license is required")
	}

	reqBody := api.RequestPINRequest{
		License: license,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(s.ctx, "POST", BaseURL+"/auth/request-pin", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}

	var result api.RequestPINResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// VerifyPIN verifies the PIN and returns access tokens
func (s *SyncService) VerifyPIN(license, pin string) (*api.VerifyPINResponse, error) {
	if license == "" {
		return nil, fmt.Errorf("license is required")
	}
	if pin == "" {
		return nil, fmt.Errorf("PIN is required")
	}

	reqBody := api.VerifyPINRequest{
		License: license,
		PIN:     pin,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(s.ctx, "POST", BaseURL+"/auth/verify-pin", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}

	var result api.VerifyPINResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// OpenBrowser opens the default browser to the specified URL
func (s *SyncService) OpenBrowser(url string) error {
	if s.ctx == nil {
		return fmt.Errorf("service not initialized")
	}
	runtime.BrowserOpenURL(s.ctx, url)
	return nil
}

// LoginResult contains the result of a login attempt
type LoginResult struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	Email       string `json:"email,omitempty"`
	RateLimited bool   `json:"rate_limited,omitempty"`
}

// InitiateLogin starts the login flow by requesting a PIN
// Returns a message indicating where the PIN was sent
func (s *SyncService) InitiateLogin() (*LoginResult, error) {
	// Get the current license from settings
	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.License == "" {
		return &LoginResult{
			Success: false,
			Message: "No license found. Please import a license file first.",
		}, nil
	}

	// Validate the license
	if err := app.IsLicenseValid(currentSettings.License); err != nil {
		return &LoginResult{
			Success: false,
			Message: fmt.Sprintf("Invalid license: %v", err),
		}, nil
	}

	// Request PIN from sync API
	resp, err := s.RequestPIN(currentSettings.License)
	if err != nil {
		// Check if this is a rate limit error
		if strings.Contains(err.Error(), "rate_limited:") {
			// Extract the rate limit message
			message := strings.TrimPrefix(err.Error(), "rate_limited: ")
			return &LoginResult{
				Success:     false,
				Message:     message,
				RateLimited: true,
			}, nil
		}
		return &LoginResult{
			Success: false,
			Message: fmt.Sprintf("Failed to request PIN: %v", err),
		}, nil
	}

	return &LoginResult{
		Success: true,
		Message: resp.Message,
	}, nil
}

// CompleteLogin completes the login flow by verifying the PIN and storing tokens
func (s *SyncService) CompleteLogin(pin string) (*LoginResult, error) {
	// Get the current license from settings
	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.License == "" {
		return &LoginResult{
			Success: false,
			Message: "No license found. Please import a license file first.",
		}, nil
	}

	// Verify PIN with sync API
	resp, err := s.VerifyPIN(currentSettings.License, pin)
	if err != nil {
		return &LoginResult{
			Success: false,
			Message: fmt.Sprintf("Failed to verify PIN: %v", err),
		}, nil
	}

	// Store the tokens in settings
	currentSettings.SyncSessionToken = resp.AccessToken
	currentSettings.SyncRefreshToken = resp.RefreshToken

	settingsSvc := settings.NewSettingsService()
	if err := settingsSvc.SaveSettings(currentSettings); err != nil {
		return &LoginResult{
			Success: false,
			Message: fmt.Sprintf("Failed to save authentication tokens: %v", err),
		}, nil
	}

	// Update menu label to show "Logout of Sync"
	s.UpdateMenuLabel()

	return &LoginResult{
		Success: true,
		Message: "Successfully logged in to sync service",
		Email:   "", // Email will be retrieved from license
	}, nil
}

// Logout clears the stored authentication tokens
func (s *SyncService) Logout() error {
	// Close any open remote workspace before logging out
	if workspaceManager != nil && workspaceManager.IsRemoteWorkspace() {
		fmt.Println("[Logout] Closing remote workspace due to logout")
		if err := workspaceManager.CloseWorkspace(); err != nil {
			fmt.Printf("[Logout] Warning: failed to close remote workspace: %v\n", err)
		}
	}

	settingsSvc := settings.NewSettingsService()
	err := settingsSvc.ClearSyncTokens()
	if err == nil {
		// Update menu label to show "Login to Sync"
		s.UpdateMenuLabel()
	}
	return err
}

// IsLoggedIn checks if the user has valid authentication tokens
func (s *SyncService) IsLoggedIn() bool {
	currentSettings := settings.GetEffectiveSettings()
	return currentSettings.SyncSessionToken != "" && currentSettings.SyncRefreshToken != ""
}

// RefreshAuthToken refreshes the access token using the refresh token
func (s *SyncService) RefreshAuthToken() error {
	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncRefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	if currentSettings.License == "" {
		return fmt.Errorf("no license available")
	}

	// Prevent concurrent refresh attempts
	if s.refreshingToken {
		return fmt.Errorf("token refresh already in progress")
	}
	s.refreshingToken = true
	defer func() { s.refreshingToken = false }()

	reqBody := api.RefreshRequest{
		RefreshToken: currentSettings.SyncRefreshToken,
		License:      currentSettings.License,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(s.ctx, "POST", BaseURL+"/auth/refresh", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
		}
		if errResp.Error.Code == "" && errResp.Error.Message == "" {
			return fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("token refresh failed: %s: %s", errResp.Error.Code, errResp.Error.Message)
	}

	var result api.RefreshResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Update the tokens in settings
	currentSettings.SyncSessionToken = result.AccessToken
	currentSettings.SyncRefreshToken = result.RefreshToken

	settingsSvc := settings.NewSettingsService()
	if err := settingsSvc.SaveSettings(currentSettings); err != nil {
		return fmt.Errorf("failed to save refreshed tokens: %w", err)
	}

	return nil
}

// doRequestWithAuth performs an HTTP request with automatic token refresh on 401/403 errors
func (s *SyncService) doRequestWithAuth(req *http.Request) (*http.Response, error) {
	// First attempt with current token
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	// If we get a 401 or 403, try refreshing the token and retry once
	// AWS API Gateway returns 403 for expired/invalid tokens
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()

		// Attempt to refresh the token
		if err := s.RefreshAuthToken(); err != nil {
			// If refresh token is expired/invalid, clear the session
			if strings.Contains(err.Error(), "token refresh failed with status 401") ||
				strings.Contains(err.Error(), "token refresh failed with status 403") {
				s.Logout()
				return nil, fmt.Errorf("session expired - please log in again")
			}

			return nil, fmt.Errorf("failed to refresh auth token: %w", err)
		}

		// Update the Authorization header with the new token
		currentSettings := settings.GetEffectiveSettings()
		req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)

		// Retry the request with the new token
		resp, err = s.client.Do(req)
		if err != nil {
			return nil, err
		}
	}

	return resp, nil
}

// GetCurrentUserEmail returns the email of the currently logged-in user from the license, or empty string if not logged in or no license
func (s *SyncService) GetCurrentUserEmail() string {
	if !s.IsLoggedIn() {
		return ""
	}

	// Get email from license details
	licenseDetails, err := app.GetLicenseDetails()
	if err != nil {
		return ""
	}

	return licenseDetails.Email
}

// RemoteWorkspace represents a workspace stored in the sync service
type RemoteWorkspace struct {
	ID          string `json:"WorkspaceID"`
	Name        string `json:"Name"`
	Description string `json:"Description,omitempty"`
	CreatedAt   string `json:"CreatedAt"`
	UpdatedAt   string `json:"UpdatedAt"`
	FileCount   int    `json:"FileCount,omitempty"`
	OwnerEmail  string `json:"OwnerEmail,omitempty"`
	IsShared    bool   `json:"IsShared,omitempty"`
	MemberCount int    `json:"MemberCount,omitempty"`
	Version     int64  `json:"Version,omitempty"`
}

// GetRemoteWorkspaces fetches all remote workspaces for the authenticated user
func (s *SyncService) GetRemoteWorkspaces() ([]RemoteWorkspace, error) {
	// Add debug logging
	if s.ctx != nil {
		runtime.LogDebug(s.ctx, "GetRemoteWorkspaces: Starting request")
	}

	if !s.IsLoggedIn() {
		if s.ctx != nil {
			runtime.LogError(s.ctx, "GetRemoteWorkspaces: Not logged in to sync service")
		}
		return nil, fmt.Errorf("not logged in to sync service")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		if s.ctx != nil {
			runtime.LogError(s.ctx, "GetRemoteWorkspaces: No valid session token")
		}
		return nil, fmt.Errorf("no valid session token")
	}

	// Add token debugging for 403 issues
	if s.ctx != nil {
		tokenLength := len(currentSettings.SyncSessionToken)
		if tokenLength > 20 {
			tokenLength = 20
		}
		runtime.LogDebug(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Using session token: %s... (length: %d)", currentSettings.SyncSessionToken[:tokenLength], len(currentSettings.SyncSessionToken)))

		// Check if we have a refresh token
		if currentSettings.SyncRefreshToken != "" {
			runtime.LogDebug(s.ctx, "GetRemoteWorkspaces: Refresh token is available")
		} else {
			runtime.LogWarning(s.ctx, "GetRemoteWorkspaces: No refresh token available")
		}
	}

	if s.ctx != nil {
		runtime.LogDebug(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Making request to %s/workspaces", BaseURL))
	}

	req, err := http.NewRequestWithContext(s.ctx, "GET", BaseURL+"/workspaces", nil)
	if err != nil {
		if s.ctx != nil {
			runtime.LogError(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Failed to create request: %v", err))
		}
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		if s.ctx != nil {
			runtime.LogError(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Failed to send request: %v", err))
		}
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if s.ctx != nil {
		runtime.LogDebug(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Received response with status %d", resp.StatusCode))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if s.ctx != nil {
			runtime.LogError(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Failed to read response: %v", err))
		}
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if s.ctx != nil {
		runtime.LogDebug(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Response body: %s", string(body)))
	}

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			if s.ctx != nil {
				runtime.LogError(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Failed to parse error response (status %d): %v", resp.StatusCode, err))
				runtime.LogError(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Raw response body: %s", string(body)))
			}
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}

		// Log the parsed error response structure for debugging
		if s.ctx != nil {
			runtime.LogError(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Parsed error response - Code: '%s', Message: '%s'", errResp.Error.Code, errResp.Error.Message))
		}

		// Handle empty error codes/messages
		errorCode := errResp.Error.Code
		errorMessage := errResp.Error.Message
		if errorCode == "" {
			errorCode = "unknown_error"
		}
		if errorMessage == "" {
			errorMessage = fmt.Sprintf("API returned status %d with empty error message", resp.StatusCode)
		}

		if s.ctx != nil {
			runtime.LogError(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: API error (status %d): %s: %s", resp.StatusCode, errorCode, errorMessage))
		}

		// If we get a 403, attempt automatic token refresh
		if resp.StatusCode == 403 {
			if s.ctx != nil {
				runtime.LogWarning(s.ctx, "GetRemoteWorkspaces: 403 Forbidden - attempting automatic token refresh")
			}

			if currentSettings.SyncRefreshToken != "" {
				// Attempt to refresh tokens automatically
				if refreshErr := s.RefreshTokens(); refreshErr != nil {
					if s.ctx != nil {
						runtime.LogError(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Token refresh failed: %v", refreshErr))
						runtime.LogInfo(s.ctx, "GetRemoteWorkspaces: Refresh token also expired - triggering re-authentication")
					}
					// Clear expired tokens and trigger re-authentication
					if clearErr := s.ClearExpiredTokens(); clearErr != nil {
						if s.ctx != nil {
							runtime.LogError(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Failed to clear expired tokens: %v", clearErr))
						}
					}
				} else {
					if s.ctx != nil {
						runtime.LogInfo(s.ctx, "GetRemoteWorkspaces: Tokens refreshed successfully - retrying request")
					}
					// Retry the original request with refreshed tokens
					return s.GetRemoteWorkspaces()
				}
			} else {
				if s.ctx != nil {
					runtime.LogInfo(s.ctx, "GetRemoteWorkspaces: No refresh token available - triggering re-authentication")
				}
				// Clear any remaining tokens and trigger re-authentication
				if clearErr := s.ClearExpiredTokens(); clearErr != nil {
					if s.ctx != nil {
						runtime.LogError(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Failed to clear tokens: %v", clearErr))
					}
				}
			}
		}

		return nil, fmt.Errorf("%s: %s", errorCode, errorMessage)
	}

	var result api.ListWorkspacesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		if s.ctx != nil {
			runtime.LogError(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Failed to unmarshal response: %v", err))
		}
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if s.ctx != nil {
		runtime.LogDebug(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Successfully parsed %d workspaces", len(result.Workspaces)))
	}

	// Convert api.Workspace to RemoteWorkspace
	remoteWorkspaces := make([]RemoteWorkspace, len(result.Workspaces))
	for i, ws := range result.Workspaces {
		remoteWorkspaces[i] = RemoteWorkspace{
			ID:          ws.WorkspaceID,
			Name:        ws.Name,
			CreatedAt:   ws.CreatedAt,
			UpdatedAt:   ws.UpdatedAt,
			OwnerEmail:  ws.OwnerEmail,
			IsShared:    ws.IsShared,
			MemberCount: ws.MemberCount,
			Version:     ws.Version,
		}
	}

	if s.ctx != nil {
		runtime.LogDebug(s.ctx, fmt.Sprintf("GetRemoteWorkspaces: Returning %d remote workspaces", len(remoteWorkspaces)))
	}

	return remoteWorkspaces, nil
}

// TestSyncConnection tests the sync API connection and authentication
func (s *SyncService) TestSyncConnection() error {
	if s.ctx != nil {
		runtime.LogDebug(s.ctx, "TestSyncConnection: Starting connection test")
	}

	if !s.IsLoggedIn() {
		return fmt.Errorf("not logged in to sync service")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return fmt.Errorf("no valid session token")
	}

	// Test with a simple GET request to the base API
	req, err := http.NewRequestWithContext(s.ctx, "GET", BaseURL+"/workspaces", nil)
	if err != nil {
		return fmt.Errorf("failed to create test request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	if s.ctx != nil {
		runtime.LogDebug(s.ctx, fmt.Sprintf("TestSyncConnection: Making test request to %s", BaseURL+"/workspaces"))
		tokenLength := len(currentSettings.SyncSessionToken)
		if tokenLength > 20 {
			tokenLength = 20
		}
		runtime.LogDebug(s.ctx, fmt.Sprintf("TestSyncConnection: Using token: %s...", currentSettings.SyncSessionToken[:tokenLength]))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		if s.ctx != nil {
			runtime.LogError(s.ctx, fmt.Sprintf("TestSyncConnection: Request failed: %v", err))
		}
		return fmt.Errorf("connection test failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read test response: %w", err)
	}

	if s.ctx != nil {
		runtime.LogDebug(s.ctx, fmt.Sprintf("TestSyncConnection: Response status: %d", resp.StatusCode))
		runtime.LogDebug(s.ctx, fmt.Sprintf("TestSyncConnection: Response body: %s", string(body)))
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("connection test failed with status %d: %s", resp.StatusCode, string(body))
	}

	if s.ctx != nil {
		runtime.LogDebug(s.ctx, "TestSyncConnection: Connection test successful")
	}

	return nil
}

// RefreshTokens refreshes the access and refresh tokens using the current refresh token
func (s *SyncService) RefreshTokens() error {
	if s.ctx != nil {
		runtime.LogDebug(s.ctx, "RefreshTokens: Starting token refresh")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncRefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	// Get current license data from settings
	if currentSettings.License == "" {
		return fmt.Errorf("no license data available")
	}

	// Create refresh request
	refreshReq := api.RefreshRequest{
		RefreshToken: currentSettings.SyncRefreshToken,
		License:      currentSettings.License,
	}

	reqBody, err := json.Marshal(refreshReq)
	if err != nil {
		return fmt.Errorf("failed to marshal refresh request: %w", err)
	}

	if s.ctx != nil {
		runtime.LogDebug(s.ctx, "RefreshTokens: Making refresh request to API")
	}

	req, err := http.NewRequestWithContext(s.ctx, "POST", BaseURL+"/auth/refresh", strings.NewReader(string(reqBody)))
	if err != nil {
		return fmt.Errorf("failed to create refresh request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Note: Refresh endpoint does NOT require Authorization header - it validates the refresh token itself

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read refresh response: %w", err)
	}

	if s.ctx != nil {
		runtime.LogDebug(s.ctx, fmt.Sprintf("RefreshTokens: Response status: %d", resp.StatusCode))
	}

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("token refresh failed: %s: %s", errResp.Error.Code, errResp.Error.Message)
	}

	var refreshResp api.RefreshResponse
	if err := json.Unmarshal(body, &refreshResp); err != nil {
		return fmt.Errorf("failed to unmarshal refresh response: %w", err)
	}

	// Update settings with new tokens
	currentSettings.SyncSessionToken = refreshResp.AccessToken
	currentSettings.SyncRefreshToken = refreshResp.RefreshToken

	settingsSvc := settings.NewSettingsService()
	if err := settingsSvc.SaveSettings(currentSettings); err != nil {
		return fmt.Errorf("failed to save refreshed tokens: %w", err)
	}

	if s.ctx != nil {
		runtime.LogInfo(s.ctx, "RefreshTokens: Tokens refreshed successfully")
	}

	return nil
}

// ClearExpiredTokens clears expired tokens and emits an event to trigger re-authentication
func (s *SyncService) ClearExpiredTokens() error {
	if s.ctx != nil {
		runtime.LogInfo(s.ctx, "ClearExpiredTokens: Clearing expired tokens and triggering re-authentication")
	}

	// Clear the tokens from settings
	settingsSvc := settings.NewSettingsService()
	if err := settingsSvc.ClearSyncTokens(); err != nil {
		return fmt.Errorf("failed to clear expired tokens: %w", err)
	}

	// Emit event to trigger re-authentication in the frontend
	if s.ctx != nil {
		runtime.EventsEmit(s.ctx, "sync:tokens_expired")
		runtime.LogInfo(s.ctx, "ClearExpiredTokens: Emitted sync:tokens_expired event")
	}

	return nil
}

// OpenRemoteWorkspaceResult contains the result of opening a remote workspace
type OpenRemoteWorkspaceResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// OpenRemoteWorkspace opens a remote workspace by downloading it from the sync service
func (s *SyncService) OpenRemoteWorkspace(workspaceID string) (*OpenRemoteWorkspaceResult, error) {
	if !s.IsLoggedIn() {
		return &OpenRemoteWorkspaceResult{
			Success: false,
			Message: "Not logged in to sync service",
		}, nil
	}

	if workspaceID == "" {
		return &OpenRemoteWorkspaceResult{
			Success: false,
			Message: "Workspace ID is required",
		}, nil
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return &OpenRemoteWorkspaceResult{
			Success: false,
			Message: "No valid session token",
		}, nil
	}

	if workspaceManager == nil {
		return &OpenRemoteWorkspaceResult{
			Success: false,
			Message: "Workspace manager not initialized",
		}, nil
	}

	// Use the workspace manager to open the remote workspace
	err := workspaceManager.OpenRemoteWorkspace(workspaceID)
	if err != nil {
		return &OpenRemoteWorkspaceResult{
			Success: false,
			Message: fmt.Sprintf("Failed to open remote workspace: %v", err),
		}, nil
	}

	return &OpenRemoteWorkspaceResult{
		Success: true,
		Message: "Remote workspace opened successfully",
	}, nil
}

// WorkspaceDetails represents detailed information about a workspace
type WorkspaceDetails struct {
	WorkspaceID string                 `json:"workspace_id"`
	HashKey     string                 `json:"hash_key"`
	Name        string                 `json:"name"`
	OwnerID     string                 `json:"owner_id"`
	IsShared    bool                   `json:"is_shared"`
	MemberCount int                    `json:"member_count"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
	Version     int64                  `json:"version"`
	Statistics  map[string]interface{} `json:"statistics,omitempty"`
}

// GetWorkspaceDetails fetches detailed information about a specific workspace
func (s *SyncService) GetWorkspaceDetails(workspaceID string) (*WorkspaceDetails, error) {
	if !s.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in to sync service")
	}

	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return nil, fmt.Errorf("no valid session token")
	}

	req, err := http.NewRequestWithContext(s.ctx, "GET", BaseURL+"/workspaces/"+workspaceID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}

	var result api.GetWorkspaceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Convert api.GetWorkspaceResponse to WorkspaceDetails
	details := &WorkspaceDetails{
		WorkspaceID: result.WorkspaceID,
		HashKey:     result.HashKey,
		Name:        result.Name,
		OwnerID:     result.OwnerEmail,
		IsShared:    result.IsShared,
		MemberCount: result.MemberCount,
		CreatedAt:   result.CreatedAt,
		UpdatedAt:   result.UpdatedAt,
		Version:     result.Version,
	}

	return details, nil
}

// GetWorkspaceAnnotations fetches all annotations for a workspace
func (s *SyncService) GetWorkspaceAnnotations(workspaceID string) ([]api.Annotation, error) {
	if !s.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in to sync service")
	}

	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return nil, fmt.Errorf("no valid session token")
	}

	req, err := http.NewRequestWithContext(s.ctx, "GET", BaseURL+"/workspaces/"+workspaceID+"/annotations", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}

	var result api.ListAnnotationsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Annotations, nil
}

// GetWorkspaceFiles fetches all files for a workspace
func (s *SyncService) GetWorkspaceFiles(workspaceID string) ([]api.WorkspaceFile, error) {
	if !s.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in to sync service")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return nil, fmt.Errorf("no session token available")
	}

	url := fmt.Sprintf("%s/workspaces/%s/files", BaseURL, workspaceID)
	req, err := http.NewRequestWithContext(s.ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResponse api.ErrorResponse
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			return nil, fmt.Errorf("API error: %s", errorResponse.Error.Message)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Debug: log the raw API response to see if no_header_row is present
	fmt.Printf("[SYNC_GET_FILES] Raw API response: %s\n", string(body))

	var result api.ListFilesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Debug: log the unmarshaled NoHeaderRow values
	for _, file := range result.Files {
		if file.Options.NoHeaderRow {
			fmt.Printf("[SYNC_GET_FILES] File %s has NoHeaderRow=true after unmarshal\n", file.FileHash)
		}
	}

	return result.Files, nil
}

// CreateWorkspace creates a new remote workspace
func (s *SyncService) CreateWorkspace(name string) (*api.CreateWorkspaceResponse, error) {
	if !s.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in to sync service")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return nil, fmt.Errorf("no session token available")
	}

	request := api.CreateWorkspaceRequest{
		Name: name,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/workspaces", BaseURL)
	req, err := http.NewRequestWithContext(s.ctx, "POST", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		var errorResponse api.ErrorResponse
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			return nil, fmt.Errorf("API error: %s", errorResponse.Error.Message)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result api.CreateWorkspaceResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
}

// DeleteWorkspace deletes a remote workspace and all its data (files, annotations, members)
func (s *SyncService) DeleteWorkspace(workspaceID string) error {
	if !s.IsLoggedIn() {
		return fmt.Errorf("not logged in to sync service")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return fmt.Errorf("no session token available")
	}

	url := fmt.Sprintf("%s/workspaces/%s", BaseURL, workspaceID)
	req, err := http.NewRequestWithContext(s.ctx, "DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResponse api.ErrorResponse
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			return fmt.Errorf("API error: %s", errorResponse.Error.Message)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteFile deletes a specific file variant from a remote workspace
func (s *SyncService) DeleteFile(workspaceID, fileHash string, opts interfaces.FileOptions) error {
	fmt.Printf("[SYNC_DELETE_FILE] Starting deletion: workspaceID=%s, fileHash=%s, opts=%+v\n",
		workspaceID, fileHash, opts)

	if !s.IsLoggedIn() {
		return fmt.Errorf("not logged in to sync service")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return fmt.Errorf("no session token available")
	}

	// Build request body with FileOptions - now using unified type, no manual copying needed
	request := api.DeleteFileRequest{
		FileHash: fileHash,
		Options:  api.FileOptions(opts),
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/workspaces/%s/files/%s", BaseURL, workspaceID, fileHash)
	fmt.Printf("[SYNC_DELETE_FILE] Calling DELETE %s\n", url)

	req, err := http.NewRequestWithContext(s.ctx, "DELETE", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		fmt.Printf("[SYNC_DELETE_FILE] Request failed: %v\n", err)
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	fmt.Printf("[SYNC_DELETE_FILE] Response: status=%d, body=%s\n", resp.StatusCode, string(body))

	if resp.StatusCode != http.StatusOK {
		var errorResponse api.ErrorResponse
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			return fmt.Errorf("API error: %s", errorResponse.Error.Message)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	fmt.Printf("[SYNC_DELETE_FILE] Successfully deleted file\n")
	return nil
}

// UpdateFileDescription updates the description of a file in a remote workspace
func (s *SyncService) UpdateFileDescription(workspaceID, fileHash string, opts interfaces.FileOptions, description string) error {
	if !s.IsLoggedIn() {
		return fmt.Errorf("not logged in to sync service")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return fmt.Errorf("no session token available")
	}

	request := api.UpdateFileRequest{
		FileHash:    fileHash,
		Options:     api.FileOptions(opts),
		Description: description,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/workspaces/%s/files/%s", BaseURL, workspaceID, fileHash)
	req, err := http.NewRequestWithContext(s.ctx, "PUT", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResponse api.ErrorResponse
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			return fmt.Errorf("API error: %s", errorResponse.Error.Message)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// WorkspaceManager interface for opening remote workspaces
type WorkspaceManager interface {
	OpenRemoteWorkspace(workspaceID string) error
	IsRemoteWorkspace() bool
	CloseWorkspace() error
}

var workspaceManager WorkspaceManager

// SetWorkspaceManager sets the workspace manager
func (s *SyncService) SetWorkspaceManager(wm WorkspaceManager) {
	workspaceManager = wm
}

// GetRemoteWorkspacesForClient implements workspace.SyncClient interface
// Returns workspaces in a format compatible with workspace.RemoteWorkspace
func (s *SyncService) GetRemoteWorkspacesForClient() (interface{}, error) {
	workspaces, err := s.GetRemoteWorkspaces()
	if err != nil {
		return nil, err
	}

	// Convert to generic interface slice that workspace package can unmarshal
	result := make([]map[string]interface{}, len(workspaces))
	for i, ws := range workspaces {
		result[i] = map[string]interface{}{
			"id":          ws.ID,
			"name":        ws.Name,
			"description": ws.Description,
			"created_at":  ws.CreatedAt,
			"updated_at":  ws.UpdatedAt,
			"file_count":  ws.FileCount,
		}
	}
	return result, nil
}

// GetWorkspaceDetailsForClient implements workspace.SyncClient interface
func (s *SyncService) GetWorkspaceDetailsForClient(workspaceID string) (interface{}, error) {
	details, err := s.GetWorkspaceDetails(workspaceID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"workspace_id": details.WorkspaceID,
		"hash_key":     details.HashKey,
		"name":         details.Name,
		"owner_id":     details.OwnerID,
		"is_shared":    details.IsShared,
		"member_count": details.MemberCount,
		"created_at":   details.CreatedAt,
		"updated_at":   details.UpdatedAt,
		"version":      details.Version,
		"statistics":   details.Statistics,
	}, nil
}

// GetWorkspaceAnnotationsForClient implements workspace.SyncClient interface
func (s *SyncService) GetWorkspaceAnnotationsForClient(workspaceID string) (interface{}, error) {
	annotations, err := s.GetWorkspaceAnnotations(workspaceID)
	if err != nil {
		return nil, err
	}

	// Convert to generic interface slice
	result := make([]map[string]interface{}, len(annotations))
	for i, annot := range annotations {
		result[i] = map[string]interface{}{
			"annotation_id": annot.AnnotationID,
			"workspace_id":  annot.WorkspaceID,
			"file_hash":     annot.FileHash,
			"options":       annot.Options, // File options (jpath, noHeaderRow, ingestTimezoneOverride)
			"row_index":     annot.RowIndex,
			"column_hashes": annot.ColumnHashes,
			"note":          annot.Note,
			"color":         annot.Color,
			"created_by":    annot.CreatedBy,
			"created_at":    annot.CreatedAt,
			"updated_at":    annot.UpdatedAt,
			"version":       annot.Version,
		}
	}
	return result, nil
}

// GetWorkspaceFilesForClient implements workspace.SyncClient interface
func (s *SyncService) GetWorkspaceFilesForClient(workspaceID string) (interface{}, error) {
	files, err := s.GetWorkspaceFiles(workspaceID)
	if err != nil {
		return nil, err
	}

	// Convert to generic interface slice
	result := make([]map[string]interface{}, len(files))
	for i, file := range files {
		result[i] = map[string]interface{}{
			"workspace_id":    file.WorkspaceID,
			"file_identifier": file.FileIdentifier,
			"file_hash":       file.FileHash,
			"options":         file.Options,
			"description":     file.Description,
			"version":         file.Version,
			"created_at":      file.CreatedAt,
			"updated_at":      file.UpdatedAt,
			"created_by":      file.CreatedBy,
		}
	}
	return result, nil
}

// CreateWorkspaceForClient implements workspace.SyncClient interface
func (s *SyncService) CreateWorkspaceForClient(name string) (interface{}, error) {
	response, err := s.CreateWorkspace(name)
	if err != nil {
		return nil, err
	}

	// Convert to generic interface map
	result := map[string]interface{}{
		"workspace_id": response.WorkspaceID,
		"hash_key":     response.HashKey,
		"name":         response.Name,
		"owner_email":  response.OwnerEmail,
		"is_shared":    response.IsShared,
		"version":      response.Version,
		"created_at":   response.CreatedAt,
	}
	return result, nil
}

// StoreFileLocation stores an absolute file path for the current Breachline instance
func (s *SyncService) StoreFileLocation(instanceID, workspaceID, fileHash, filePath string) error {
	if !s.IsLoggedIn() {
		return fmt.Errorf("not logged in")
	}

	request := api.StoreFileLocationRequest{
		InstanceID:  instanceID,
		WorkspaceID: workspaceID,
		FileHash:    fileHash,
		FilePath:    filePath,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", BaseURL+"/file-locations", strings.NewReader(string(requestBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	currentSettings := settings.GetEffectiveSettings()
	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetFileLocation retrieves an absolute file path for the current Breachline instance
func (s *SyncService) GetFileLocation(instanceID, fileHash string) (string, error) {
	if !s.IsLoggedIn() {
		return "", fmt.Errorf("not logged in")
	}

	request := api.GetFileLocationRequest{
		InstanceID: instanceID,
		FileHash:   fileHash,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", BaseURL+"/file-locations/get", strings.NewReader(string(requestBody)))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	currentSettings := settings.GetEffectiveSettings()
	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("file location not found")
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var response api.FileLocationResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return response.FilePath, nil
}

// CreateFile creates a new file in a remote workspace
func (s *SyncService) CreateFile(workspaceID, fileHash string, opts interfaces.FileOptions, description, filePath string) error {
	if !s.IsLoggedIn() {
		return fmt.Errorf("not logged in")
	}

	// Get instance ID for file location tracking
	instanceID := s.GetInstanceID()

	request := api.CreateFileRequest{
		WorkspaceID: workspaceID,
		FileHash:    fileHash,
		Options:     api.FileOptions(opts),
		Description: description,
		FilePath:    filePath,
		InstanceID:  instanceID,
	}

	// Debug: log the settings being sent to sync API
	fmt.Printf("[SYNC_CREATE_FILE] Creating file with opts=%+v, fileHash=%s, filePath=%s, instanceID=%s\n", opts, fileHash, filePath, instanceID)

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/workspaces/%s/files", BaseURL, workspaceID)
	req, err := http.NewRequestWithContext(s.ctx, "POST", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	currentSettings := settings.GetEffectiveSettings()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp api.ErrorResponse
		if json.Unmarshal(body, &errorResp) == nil {
			return fmt.Errorf("API error: %s", errorResp.Error.Message)
		}
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetInstanceID returns the current Breachline instance ID
func (s *SyncService) GetInstanceID() string {
	currentSettings := settings.GetEffectiveSettings()
	return currentSettings.InstanceID
}

// GetFileLocationsForInstance retrieves all file locations for the current instance
func (s *SyncService) GetFileLocationsForInstance() (interface{}, error) {
	if !s.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in")
	}

	instanceID := s.GetInstanceID()
	fmt.Printf("[SYNC_FILE_LOCATIONS] Getting file locations for instanceID=%s\n", instanceID)

	request := api.ListFileLocationsRequest{
		InstanceID: instanceID,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/file-locations/all", BaseURL)
	req, err := http.NewRequestWithContext(s.ctx, "GET", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	currentSettings := settings.GetEffectiveSettings()
	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp api.ErrorResponse
		if json.Unmarshal(body, &errorResp) == nil {
			return nil, fmt.Errorf("API error: %s", errorResp.Error.Message)
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	fmt.Printf("[SYNC_FILE_LOCATIONS] Retrieved file locations response: %s\n", string(body))
	return result, nil
}

// CreateAnnotation creates a new annotation in a remote workspace
func (s *SyncService) CreateAnnotation(workspaceID, fileHash string, opts interfaces.FileOptions, rowIndex int, note, color string) (string, error) {
	if !s.IsLoggedIn() {
		return "", fmt.Errorf("not logged in to sync service")
	}

	if workspaceID == "" {
		return "", fmt.Errorf("workspace ID is required")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return "", fmt.Errorf("no valid session token")
	}

	request := api.CreateAnnotationRequest{
		WorkspaceID: workspaceID,
		FileHash:    fileHash,
		Options:     api.FileOptions(opts),
		RowIndex:    rowIndex,
		Note:        note,
		Color:       api.AnnotationColor(color),
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(s.ctx, "POST", BaseURL+"/workspaces/"+workspaceID+"/annotations", bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return "", fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}

	var result api.AnnotationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.AnnotationID, nil
}

// createAnnotationBatchInternal creates multiple annotations in a remote workspace in a single request
func (s *SyncService) createAnnotationBatchInternal(workspaceID, fileHash string, opts interfaces.FileOptions, annotationRows []api.AnnotationRow, note, color string) ([]string, error) {
	if !s.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in to sync service")
	}

	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	if len(annotationRows) == 0 {
		return nil, fmt.Errorf("annotation rows are required")
	}

	if len(annotationRows) > 256 {
		return nil, fmt.Errorf("batch size exceeds maximum of 256 annotations")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return nil, fmt.Errorf("no valid session token")
	}

	request := api.BatchCreateAnnotationRequest{
		WorkspaceID:    workspaceID,
		FileHash:       fileHash,
		Options:        api.FileOptions(opts),
		Note:           note,
		Color:          api.AnnotationColor(color),
		AnnotationRows: annotationRows,
	}

	fmt.Printf("[SYNC_ANNOTATION_BATCH] Creating batch annotation: workspaceID=%s, fileHash=%s, opts=%+v, note=%s, color=%s, rows=%d\n",
		workspaceID, fileHash, opts, note, color, len(annotationRows))

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(s.ctx, "POST", BaseURL+"/workspaces/"+workspaceID+"/annotations", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}

	var result api.BatchAnnotationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.AnnotationIDs, nil
}

// CreateAnnotationBatch wrapper for interface compatibility
func (s *SyncService) CreateAnnotationBatch(workspaceID, fileHash string, opts interfaces.FileOptions, annotationRows []interface{}, note, color string) ([]string, error) {
	// Convert interface{} slice to api.AnnotationRow slice
	apiRows := make([]api.AnnotationRow, len(annotationRows))
	for i, row := range annotationRows {
		if apiRow, ok := row.(api.AnnotationRow); ok {
			apiRows[i] = apiRow
		} else {
			return nil, fmt.Errorf("invalid annotation row type at index %d", i)
		}
	}

	return s.createAnnotationBatchInternal(workspaceID, fileHash, opts, apiRows, note, color)
}

// UpdateAnnotation updates an existing annotation in a remote workspace
func (s *SyncService) UpdateAnnotation(workspaceID, annotationID, note, color string) error {
	if !s.IsLoggedIn() {
		return fmt.Errorf("not logged in to sync service")
	}

	if workspaceID == "" || annotationID == "" {
		return fmt.Errorf("workspace ID and annotation ID are required")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return fmt.Errorf("no valid session token")
	}

	request := api.UpdateAnnotationRequest{
		AnnotationID: annotationID,
		Note:         note,
		Color:        api.AnnotationColor(color),
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(s.ctx, "PUT", BaseURL+"/workspaces/"+workspaceID+"/annotations/"+annotationID, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}

	return nil
}

// updateAnnotationBatchInternal handles the internal batch update logic
func (s *SyncService) updateAnnotationBatchInternal(workspaceID string, updates []api.UpdateAnnotationRequest) (*api.BatchUpdateAnnotationResponse, error) {
	if !s.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in to sync service")
	}

	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	if len(updates) == 0 {
		return nil, fmt.Errorf("at least one update is required")
	}

	if len(updates) > 256 {
		return nil, fmt.Errorf("batch size exceeds maximum of 256 updates")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return nil, fmt.Errorf("no valid session token")
	}

	request := api.BatchUpdateAnnotationRequest{
		WorkspaceID: workspaceID,
		Updates:     updates,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch update request: %w", err)
	}

	req, err := http.NewRequestWithContext(s.ctx, "PUT", BaseURL+"/workspaces/"+workspaceID+"/annotations", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create batch update request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send batch update request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read batch update response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("batch update request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}

	var response api.BatchUpdateAnnotationResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch update response: %w", err)
	}

	return &response, nil
}

// UpdateAnnotationBatch updates multiple annotations in a remote workspace
func (s *SyncService) UpdateAnnotationBatch(workspaceID string, updates []api.UpdateAnnotationRequest) (*api.BatchUpdateAnnotationResponse, error) {
	return s.updateAnnotationBatchInternal(workspaceID, updates)
}

// DeleteAnnotation deletes an annotation from a remote workspace
func (s *SyncService) DeleteAnnotation(workspaceID, annotationID string) error {
	if !s.IsLoggedIn() {
		return fmt.Errorf("not logged in to sync service")
	}

	if workspaceID == "" || annotationID == "" {
		return fmt.Errorf("workspace ID and annotation ID are required")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return fmt.Errorf("no valid session token")
	}

	req, err := http.NewRequestWithContext(s.ctx, "DELETE", BaseURL+"/workspaces/"+workspaceID+"/annotations/"+annotationID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}

	return nil
}

// deleteAnnotationBatchInternal handles the core batch annotation deletion logic
func (s *SyncService) deleteAnnotationBatchInternal(workspaceID string, annotationIDs []string) ([]string, error) {
	if !s.IsLoggedIn() {
		return nil, fmt.Errorf("not logged in to sync service")
	}

	if workspaceID == "" || len(annotationIDs) == 0 {
		return nil, fmt.Errorf("workspace ID and annotation IDs are required")
	}

	// Validate batch size limit (max 256)
	if len(annotationIDs) > 256 {
		return nil, fmt.Errorf("maximum batch size is 256 annotations")
	}

	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.SyncSessionToken == "" {
		return nil, fmt.Errorf("no valid session token")
	}

	request := api.BatchDeleteAnnotationRequest{
		WorkspaceID:   workspaceID,
		AnnotationIDs: annotationIDs,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(s.ctx, "DELETE", BaseURL+"/workspaces/"+workspaceID+"/annotations", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+currentSettings.SyncSessionToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.doRequestWithAuth(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		var errResp api.ErrorResponse
		if err := json.Unmarshal(body, &errResp); err != nil {
			return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}

	var result api.BatchDeleteAnnotationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.DeletedIDs, nil
}

// DeleteAnnotationBatch wrapper for interface compatibility
func (s *SyncService) DeleteAnnotationBatch(workspaceID string, annotationIDs []string) ([]string, error) {
	return s.deleteAnnotationBatchInternal(workspaceID, annotationIDs)
}
