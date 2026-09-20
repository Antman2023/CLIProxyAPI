package auth

import (
	"strconv"
	"strings"
)

// WebsocketsSetting returns the explicitly configured websocket setting.
// A valid attribute takes precedence over metadata. If neither source has a
// valid value, a present invalid value is treated as explicitly disabled.
func WebsocketsSetting(auth *Auth) (bool, bool) {
	if auth == nil {
		return false, false
	}

	configured := false
	if raw, ok := auth.Attributes["websockets"]; ok {
		raw = strings.TrimSpace(raw)
		if raw != "" {
			configured = true
			if parsed, errParse := strconv.ParseBool(raw); errParse == nil {
				return parsed, true
			}
		}
	}

	if raw, ok := auth.Metadata["websockets"]; ok && raw != nil {
		configured = true
		switch value := raw.(type) {
		case bool:
			return value, true
		case string:
			if parsed, errParse := strconv.ParseBool(strings.TrimSpace(value)); errParse == nil {
				return parsed, true
			}
		}
	}

	return false, configured
}

// WebsocketsEnabled returns the effective websocket setting for a credential.
// Codex credentials default to enabled when the setting is omitted; all other
// providers remain disabled by default.
func WebsocketsEnabled(auth *Auth) bool {
	if value, configured := WebsocketsSetting(auth); configured {
		return value
	}
	return auth != nil && strings.EqualFold(strings.TrimSpace(auth.Provider), "codex")
}
