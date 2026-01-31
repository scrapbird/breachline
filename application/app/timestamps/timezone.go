package timestamps

import (
	"breachline/app/settings"
	"strings"
	"time"
)

// GetLocationForTZ resolves a timezone name to a *time.Location. Supports "Local", "UTC", and IANA TZ names.
func GetLocationForTZ(name string) *time.Location {
	tzName := strings.TrimSpace(name)
	switch strings.ToUpper(tzName) {
	case "", "LOCAL":
		return time.Local
	case "UTC":
		return time.UTC
	default:
		if l, err := time.LoadLocation(tzName); err == nil {
			return l
		}
		return time.Local
	}
}

// GetDefaultIngestTimezone returns the default ingest timezone from settings
func GetDefaultIngestTimezone() *time.Location {
	effective := settings.GetEffectiveSettings()
	return GetLocationForTZ(effective.DefaultIngestTimezone)
}

// GetIngestTimezoneWithOverride returns the effective ingest timezone
// If override is non-empty, uses that; otherwise returns default from settings
func GetIngestTimezoneWithOverride(override string) *time.Location {
	if override != "" {
		return GetLocationForTZ(override)
	}
	return GetDefaultIngestTimezone()
}
