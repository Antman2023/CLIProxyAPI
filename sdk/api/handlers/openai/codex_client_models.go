package openai

import (
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	codexmodels "github.com/router-for-me/CLIProxyAPI/v7/internal/client/codex/models"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	log "github.com/sirupsen/logrus"
)

func (h *OpenAIAPIHandler) proxyCodexClientModels(c *gin.Context) bool {
	if h == nil || h.AuthManager == nil {
		return false
	}
	auth := highestTierCodexAuth(h.AuthManager.List())
	if auth == nil {
		return false
	}

	baseURL := strings.TrimSpace(auth.Attributes["base_url"])
	if baseURL == "" {
		baseURL = "https://chatgpt.com/backend-api/codex"
	}
	upstreamURL := strings.TrimRight(baseURL, "/") + "/models"
	if c.Request.URL.RawQuery != "" {
		upstreamURL += "?" + c.Request.URL.RawQuery
	}

	headers := make(http.Header)
	for _, name := range []string{"Accept", "Originator", "User-Agent", "Version", "If-None-Match", "X-Codex-Beta-Features"} {
		if value := strings.TrimSpace(c.GetHeader(name)); value != "" {
			headers.Set(name, value)
		}
	}
	if headers.Get("Accept") == "" {
		headers.Set("Accept", "application/json")
	}
	if accountID, ok := auth.Metadata["account_id"].(string); ok && strings.TrimSpace(accountID) != "" {
		headers.Set("Chatgpt-Account-Id", accountID)
	}

	req, errRequest := h.AuthManager.NewHttpRequest(c.Request.Context(), auth, http.MethodGet, upstreamURL, nil, headers)
	if errRequest != nil {
		log.WithError(errRequest).Warn("codex catalog: failed to prepare upstream request")
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to request Codex catalog"})
		return true
	}
	resp, errRequest := h.AuthManager.HttpRequest(c.Request.Context(), auth, req)
	if errRequest != nil {
		log.WithError(errRequest).Warn("codex catalog: upstream request failed")
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to request Codex catalog"})
		return true
	}
	if resp == nil || resp.Body == nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "empty Codex catalog response"})
		return true
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.WithError(errClose).Warn("codex catalog: failed to close upstream response")
		}
	}()
	for _, name := range []string{"Content-Type", "ETag", "Cache-Control", "Last-Modified", "Retry-After", "Vary"} {
		for _, value := range resp.Header.Values(name) {
			c.Writer.Header().Add(name, value)
		}
	}
	c.Status(resp.StatusCode)
	if _, errCopy := io.Copy(c.Writer, resp.Body); errCopy != nil {
		log.WithError(errCopy).Warn("codex catalog: failed to forward upstream response")
	}
	return true
}

func highestTierCodexAuth(auths []*coreauth.Auth) *coreauth.Auth {
	now := time.Now()
	var eligible []*coreauth.Auth
	for _, auth := range auths {
		if auth == nil || !strings.EqualFold(auth.Provider, "codex") || auth.AuthKind() != coreauth.AuthKindOAuth || auth.Disabled || !auth.HasValidAccessToken(now) {
			continue
		}
		eligible = append(eligible, auth)
	}
	sort.Slice(eligible, func(i, j int) bool {
		leftRank := codexPlanRank(eligible[i])
		rightRank := codexPlanRank(eligible[j])
		if leftRank != rightRank {
			return leftRank > rightRank
		}
		if !eligible[i].CreatedAt.Equal(eligible[j].CreatedAt) {
			return eligible[i].CreatedAt.Before(eligible[j].CreatedAt)
		}
		return eligible[i].ID < eligible[j].ID
	})
	if len(eligible) == 0 {
		return nil
	}
	return eligible[0]
}

func codexPlanRank(auth *coreauth.Auth) int {
	plan := strings.ToLower(strings.TrimSpace(auth.Attributes["plan_type"]))
	if plan == "" {
		plan, _ = auth.Metadata["plan_type"].(string)
		plan = strings.ToLower(strings.TrimSpace(plan))
	}
	switch plan {
	case "pro", "enterprise":
		return 4
	case "team", "business", "edu":
		return 3
	case "plus":
		return 2
	case "go":
		return 1
	default:
		return 0
	}
}

func (h *OpenAIAPIHandler) codexClientModelsResponse(clientVersion ...string) map[string]any {
	version := ""
	if len(clientVersion) > 0 {
		version = clientVersion[0]
	}
	optimizeMultiAgentV2 := h != nil && h.Cfg != nil && h.Cfg.CodexOptimizeMultiAgentV2
	return codexmodels.BuildResponseForClient(h.Models(), registry.GetGlobalRegistry().GetModelProviders, optimizeMultiAgentV2, version)
}

// CodexClientModelsResponse builds a Codex client model response.
func CodexClientModelsResponse(models []map[string]any) map[string]any {
	return codexmodels.BuildResponse(models, nil, false)
}

// CodexClientModelsResponseWithMultiAgentV2 builds a Codex client model response
// and advertises multi-agent v2 for synthesized models when enabled.
func CodexClientModelsResponseWithMultiAgentV2(models []map[string]any, enabled bool) map[string]any {
	return codexmodels.BuildResponse(models, nil, enabled)
}

// CodexClientModelsResponseForClient builds a Codex client model response
// tailored for a specific client version.
func CodexClientModelsResponseForClient(models []map[string]any, clientVersion string, enabled bool) map[string]any {
	return codexmodels.BuildResponseForClient(models, nil, enabled, clientVersion)
}
