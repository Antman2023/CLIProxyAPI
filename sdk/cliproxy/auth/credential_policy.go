package auth

import "strings"

const (
	// CredentialPolicyCodexAlphaSearchV1 selects credentials supported by Codex Alpha Search.
	CredentialPolicyCodexAlphaSearchV1 = "codex_alpha_search_v1"
)

func normalizeCredentialPolicy(policy string) string {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case CredentialPolicyCodexAlphaSearchV1:
		return CredentialPolicyCodexAlphaSearchV1
	default:
		return ""
	}
}

func credentialPolicyAllows(policy string, auth *Auth) bool {
	if auth == nil {
		return false
	}
	switch policy {
	case CredentialPolicyCodexAlphaSearchV1:
		if !strings.EqualFold(strings.TrimSpace(auth.Provider), "codex") {
			return false
		}
		switch auth.AuthKind() {
		case AuthKindOAuth:
			return true
		case AuthKindAPIKey:
			return strings.EqualFold(authAttribute(auth, AttributeCodexAlphaSearch), "true")
		default:
			return false
		}
	default:
		return false
	}
}

// credentialPolicyAllowsModelIndependentSelection reports whether a policy may
// select an auth without requiring the route model to be registered for it.
// Explicit routing prefixes remain authoritative.
func credentialPolicyAllowsModelIndependentSelection(policy string, auth *Auth, routeModel string) bool {
	if policy != CredentialPolicyCodexAlphaSearchV1 || auth == nil {
		return false
	}
	model := canonicalModelKey(routeModel)
	slash := strings.IndexByte(model, '/')
	if slash < 0 {
		return true
	}
	prefix := strings.TrimSpace(auth.Prefix)
	return prefix != "" && strings.EqualFold(strings.TrimSpace(model[:slash]), prefix)
}
