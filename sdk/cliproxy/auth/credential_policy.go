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

// credentialPolicyAllowsExcludedModel reports whether a policy may use an auth
// even though the requested route model was removed from its registered model
// set by that auth's exclusion rules.
func (m *Manager) credentialPolicyAllowsExcludedModel(policy string, auth *Auth, routeModel string) bool {
	if policy != CredentialPolicyCodexAlphaSearchV1 || auth == nil || auth.Attributes == nil {
		return false
	}
	excludedModels := strings.TrimSpace(auth.Attributes["excluded_models"])
	if excludedModels == "" {
		return false
	}
	excluded := strings.Split(excludedModels, ",")

	models := []string{
		rewriteModelForAuth(strings.TrimSpace(routeModel), auth),
		m.selectionModelForAuth(auth, routeModel),
		m.ResolveExecutionModel(auth, routeModel),
	}
	for _, model := range models {
		model = strings.ToLower(strings.TrimSpace(model))
		if model == "" {
			continue
		}
		for _, pattern := range excluded {
			if matchExcludedModelPattern(strings.ToLower(strings.TrimSpace(pattern)), model) {
				return true
			}
		}
	}
	return false
}

// matchExcludedModelPattern matches the exclusion syntax used by model
// registration, where '*' represents any substring.
func matchExcludedModelPattern(pattern, model string) bool {
	if pattern == "" {
		return false
	}
	if !strings.Contains(pattern, "*") {
		return pattern == model
	}

	parts := strings.Split(pattern, "*")
	if prefix := parts[0]; prefix != "" {
		if !strings.HasPrefix(model, prefix) {
			return false
		}
		model = model[len(prefix):]
	}
	if suffix := parts[len(parts)-1]; suffix != "" {
		if !strings.HasSuffix(model, suffix) {
			return false
		}
		model = model[:len(model)-len(suffix)]
	}
	for _, segment := range parts[1 : len(parts)-1] {
		if segment == "" {
			continue
		}
		index := strings.Index(model, segment)
		if index < 0 {
			return false
		}
		model = model[index+len(segment):]
	}
	return true
}
