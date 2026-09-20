package auth

import "testing"

func TestWebsocketsEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		auth *Auth
		want bool
	}{
		{name: "nil credential", want: false},
		{name: "codex unset", auth: &Auth{Provider: "codex"}, want: true},
		{name: "codex unset case insensitive", auth: &Auth{Provider: " CODEX "}, want: true},
		{name: "codex attribute false", auth: &Auth{Provider: "codex", Attributes: map[string]string{"websockets": "false"}}, want: false},
		{name: "codex metadata false", auth: &Auth{Provider: "codex", Metadata: map[string]any{"websockets": false}}, want: false},
		{name: "codex attribute true overrides metadata", auth: &Auth{Provider: "codex", Attributes: map[string]string{"websockets": "true"}, Metadata: map[string]any{"websockets": false}}, want: true},
		{name: "codex invalid setting", auth: &Auth{Provider: "codex", Metadata: map[string]any{"websockets": "invalid"}}, want: false},
		{name: "xai unset", auth: &Auth{Provider: "xai"}, want: false},
		{name: "xai enabled", auth: &Auth{Provider: "xai", Attributes: map[string]string{"websockets": "true"}}, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := WebsocketsEnabled(test.auth); got != test.want {
				t.Fatalf("WebsocketsEnabled() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestWebsocketsSettingFallsBackToMetadataAfterInvalidAttribute(t *testing.T) {
	t.Parallel()

	auth := &Auth{
		Provider:   "codex",
		Attributes: map[string]string{"websockets": "invalid"},
		Metadata:   map[string]any{"websockets": false},
	}
	got, configured := WebsocketsSetting(auth)
	if !configured || got {
		t.Fatalf("WebsocketsSetting() = (%t, %t), want (false, true)", got, configured)
	}
}
