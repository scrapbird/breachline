package app

import (
	"breachline/app/settings"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// PublicKey is the ECDSA public key used to verify license JWT signatures.
// This is a placeholder - replace with the actual public key.
var PublicKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEdJO2IHmjuQj0ZftNCs2BAkwPFpDN
3VD7WGoDAN0Q7i40iuVDxfXD6shIcB8zX5/zmtW1VFO03fR5jE6gsnVkKA==
-----END PUBLIC KEY-----`

// LicenseService manages license validation and storage
type LicenseService struct {
	ctx context.Context
	app *App
}

// NewLicenseService creates a new LicenseService
func NewLicenseService() *LicenseService {
	return &LicenseService{}
}

// SetApp allows the main function to inject the App reference
func (ls *LicenseService) SetApp(app *App) {
	ls.app = app
}

// Startup receives the Wails context
func (ls *LicenseService) Startup(ctx context.Context) {
	ls.ctx = ctx
}

// isValidUUID checks if a string is a valid UUID (RFC 4122)
func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	// Check format: 8-4-4-4-12 (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx)
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	// Check all other characters are valid hex
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// IsLicenseValid checks if the provided license content is valid.
// It performs the following checks:
// 1. Base64 decodes the content
// 2. Parses it as a JWT
// 3. Validates the JWT time constraints (start/end)
// 4. Verifies the JWT signature using our public key
func IsLicenseValid(licenseContent string) error {
	if licenseContent == "" {
		return errors.New("license content is empty")
	}

	// Step 1: Base64 decode
	decoded, err := base64.StdEncoding.DecodeString(licenseContent)
	if err != nil {
		return fmt.Errorf("failed to base64 decode license: %w", err)
	}

	tokenString := string(decoded)

	// Step 2: Parse the public key
	pubKey, err := parsePublicKey(PublicKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	// Step 3: Parse and validate the JWT
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method is ECDSA
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return pubKey, nil
	})

	if err != nil {
		return fmt.Errorf("failed to parse JWT: %w", err)
	}

	if !token.Valid {
		return errors.New("JWT token is invalid")
	}

	// Step 4: Validate time constraints and required claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return errors.New("failed to parse JWT claims")
	}

	// Check for required email claim
	email, emailOk := claims["email"]
	if !emailOk {
		return errors.New("license missing required 'email' claim")
	}
	if emailStr, ok := email.(string); !ok || emailStr == "" {
		return errors.New("license 'email' claim is empty or invalid")
	}

	// Check for required id claim and validate it's a UUID
	id, idOk := claims["id"]
	if !idOk {
		return errors.New("license missing required 'id' claim")
	}
	idStr, ok := id.(string)
	if !ok || idStr == "" {
		return errors.New("license 'id' claim is empty or invalid")
	}
	// Validate UUID format (RFC 4122)
	if !isValidUUID(idStr) {
		return errors.New("license 'id' claim is not a valid UUID")
	}

	now := time.Now()

	// Check "nbf" (not before) - start time should not be in the future
	if nbf, ok := claims["nbf"]; ok {
		var notBefore time.Time
		switch v := nbf.(type) {
		case float64:
			notBefore = time.Unix(int64(v), 0)
		case int64:
			notBefore = time.Unix(v, 0)
		default:
			return errors.New("invalid 'nbf' claim format")
		}
		if now.Before(notBefore) {
			return fmt.Errorf("license not valid yet (starts at %v)", notBefore)
		}
	}

	// Check "exp" (expiration) - end time should not be in the past
	if exp, ok := claims["exp"]; ok {
		var expiresAt time.Time
		switch v := exp.(type) {
		case float64:
			expiresAt = time.Unix(int64(v), 0)
		case int64:
			expiresAt = time.Unix(v, 0)
		default:
			return errors.New("invalid 'exp' claim format")
		}
		if now.After(expiresAt) {
			return fmt.Errorf("license has expired (expired at %v)", expiresAt)
		}
	}

	return nil
}

// IsLicensed returns true if the application has a valid license stored in settings
func IsLicensed() bool {
	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.License == "" {
		return false
	}
	return IsLicenseValid(currentSettings.License) == nil
}

// LicenseDetails contains information extracted from a license
type LicenseDetails struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}

// LicenseOpenResult represents the result of opening a license file
type LicenseOpenResult struct {
	Success   bool   `json:"success"`
	Email     string `json:"email"`
	IsExpired bool   `json:"isExpired"`
	Message   string `json:"message"`
}

// GetLicenseDetails returns the details from the currently stored license.
// Returns an error if no license is stored or if the license is invalid.
func GetLicenseDetails() (*LicenseDetails, error) {
	currentSettings := settings.GetEffectiveSettings()
	if currentSettings.License == "" {
		return nil, errors.New("no license found")
	}

	// Validate the license first
	if err := IsLicenseValid(currentSettings.License); err != nil {
		return nil, fmt.Errorf("invalid license: %w", err)
	}

	// Decode and parse the license
	decoded, err := base64.StdEncoding.DecodeString(currentSettings.License)
	if err != nil {
		return nil, fmt.Errorf("failed to decode license: %w", err)
	}

	tokenString := string(decoded)

	// Parse without validation since we already validated above
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("failed to parse JWT claims")
	}

	details := &LicenseDetails{}

	// Extract id
	if id, ok := claims["id"].(string); ok {
		details.ID = id
	}

	// Extract email
	if email, ok := claims["email"].(string); ok {
		details.Email = email
	}

	// Extract start date (nbf - not before)
	if nbf, ok := claims["nbf"]; ok {
		switch v := nbf.(type) {
		case float64:
			details.StartDate = time.Unix(int64(v), 0)
		case int64:
			details.StartDate = time.Unix(v, 0)
		}
	}

	// Extract end date (exp - expiration)
	if exp, ok := claims["exp"]; ok {
		switch v := exp.(type) {
		case float64:
			details.EndDate = time.Unix(int64(v), 0)
		case int64:
			details.EndDate = time.Unix(v, 0)
		}
	}

	return details, nil
}

// GetLicenseDetails returns the details from the currently stored license.
// This method is exposed to the frontend via Wails.
func (ls *LicenseService) GetLicenseDetails() (*LicenseDetails, error) {
	return GetLicenseDetails()
}

// ImportLicenseFile opens a file dialog for the user to select a license file,
// validates it, and stores it in settings if valid.
func (ls *LicenseService) ImportLicenseFile() (*LicenseOpenResult, error) {
	if ls.ctx == nil {
		return nil, errors.New("service not initialized")
	}

	// Open file dialog - prefer .lic files but allow any file type
	filePath, err := runtime.OpenFileDialog(ls.ctx, runtime.OpenDialogOptions{
		Title: "Open License File",
		Filters: []runtime.FileFilter{
			{DisplayName: "License Files", Pattern: "*.lic"},
			{DisplayName: "All Files", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open file dialog: %w", err)
	}

	// User cancelled
	if filePath == "" {
		return nil, nil
	}

	// Read the file content
	content, err := readFileAsString(filePath)
	if err != nil {
		return &LicenseOpenResult{
			Success: false,
			Message: "Failed to read license file",
		}, nil
	}

	// Validate the license
	validationErr := IsLicenseValid(content)
	fmt.Println(validationErr)
	if validationErr != nil {
		// Check if it's an expiration error
		errMsg := validationErr.Error()
		isExpired := strings.Contains(errMsg, "token is expired")
		return &LicenseOpenResult{
			Success:   false,
			IsExpired: isExpired,
			Message:   errMsg,
		}, nil
	}

	// Extract email from the license
	var email string
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err == nil {
		tokenString := string(decoded)
		token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
		if err == nil {
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				if emailClaim, ok := claims["email"].(string); ok {
					email = emailClaim
				}
			}
		}
	}

	// Save to settings
	currentSettings := settings.GetEffectiveSettings()
	currentSettings.License = content

	// Use settings service to save
	settingsSvc := settings.NewSettingsService()
	if err := settingsSvc.SaveSettings(currentSettings); err != nil {
		return &LicenseOpenResult{
			Success: false,
			Message: "Failed to save license",
		}, nil
	}

	return &LicenseOpenResult{
		Success: true,
		Email:   email,
		Message: "License successfully registered",
	}, nil
}

// parsePublicKey parses a PEM-encoded ECDSA public key
func parsePublicKey(pemStr string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not ECDSA")
	}

	return ecdsaPub, nil
}

// readFileAsString reads a file and returns its content as a string
func readFileAsString(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
