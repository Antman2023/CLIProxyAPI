package cliproxy

import (
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/config"
)

func TestAntigravityRegistryPublicationFreePrefix(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		prefix      string
		forcePrefix bool
		wantIDs     []string
		wantAfter   []string
	}{
		{
			name: "free filters aliased routes", prefix: "free",
			wantIDs: []string{"future-luna", "free/future-luna", "codex-auto-review", "free/codex-auto-review"},
		},
		{
			name: "forced free filters aliased routes", prefix: "free", forcePrefix: true,
			wantIDs: []string{"free/future-luna", "free/codex-auto-review"},
		},
		{
			name: "other prefix preserves routes", prefix: "paid", forcePrefix: true,
			wantIDs:   []string{"paid/future-luna", "paid/codex-auto-review", "paid/gpt-oss-120b-medium"},
			wantAfter: []string{"paid/gpt-oss-120b-medium"},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resetAntigravityCapabilityCache()
			t.Cleanup(resetAntigravityCapabilityCache)
			cfg := &config.Config{OAuthModelAlias: map[string][]config.OAuthModelAlias{
				"antigravity": {
					{Name: "gemini-3.1-flash-lite", Alias: "future-luna"},
					{Name: "gemini-pro-agent", Alias: "codex-auto-review"},
				},
			}}
			cfg.ForceModelPrefix = testCase.forcePrefix
			manager := coreauth.NewManager(nil, nil, nil)
			svc := &Service{cfg: cfg, coreManager: manager}
			auth := antigravityTestAuth("publication-free-prefix", "http://127.0.0.1:1")
			auth.Prefix = testCase.prefix
			var err error
			auth, err = manager.Register(t.Context(), auth)
			if err != nil {
				t.Fatal(err)
			}
			reg := GlobalModelRegistry()
			t.Cleanup(func() { reg.UnregisterClient(auth.ID) })
			key := svc.antigravityCapabilityKey(auth)
			publish := func(hints antigravityModelCapabilityHints) {
				t.Helper()
				antigravityCapabilityMu.Lock()
				antigravityCapabilityCache[key] = antigravityCapabilityCacheEntry{hints: hints, probeRevision: hints.revision}
				antigravityCapabilityMu.Unlock()
				epoch := reg.ClientRegistrationEpoch(auth.ID)
				svc.applyAntigravityModelHints(t.Context(), auth, "antigravity", hints, key, auth.RegistrationEpoch, epoch)
			}
			assertIDs := func(want []string) {
				t.Helper()
				got := codexModelIDSet(reg.GetModelsForClient(auth.ID))
				if len(got) != len(want) {
					t.Fatalf("registered models = %v, want %v", got, want)
				}
				for _, id := range want {
					if _, ok := got[id]; !ok {
						t.Errorf("missing registered model %q", id)
					}
				}
			}
			publish(antigravityModelCapabilityHints{ModelIDs: map[string]struct{}{
				"gemini-3.1-flash-lite": {}, "gemini-pro-agent": {}, "gpt-oss-120b-medium": {},
			}, revision: 1})
			assertIDs(testCase.wantIDs)
			// An account refresh with only disallowed models must revoke old free routes.
			publish(antigravityModelCapabilityHints{ModelIDs: map[string]struct{}{
				"gpt-oss-120b-medium": {},
			}, revision: 2})
			assertIDs(testCase.wantAfter)
		})
	}
}

func TestAntigravityPublishedCacheDoesNotReconcileEmptyRegistry(t *testing.T) {
	resetAntigravityCapabilityCache()
	t.Cleanup(resetAntigravityCapabilityCache)
	manager := coreauth.NewManager(nil, nil, nil)
	svc := &Service{cfg: &config.Config{}, coreManager: manager}
	const model = "gemini-3.1-flash-lite"
	auth := antigravityTestAuth("publication-cooldown", "http://127.0.0.1:1")
	retry := time.Now().Add(time.Hour)
	auth.ModelStates = map[string]*coreauth.ModelState{model: {Unavailable: true, NextRetryAfter: retry, Status: coreauth.StatusError}}
	auth, _ = manager.Register(t.Context(), auth)
	t.Cleanup(func() { GlobalModelRegistry().UnregisterClient(auth.ID) })
	key := svc.antigravityCapabilityKey(auth)
	hints := antigravityModelCapabilityHints{ModelIDs: map[string]struct{}{model: {}}, revision: 1}
	antigravityCapabilityMu.Lock()
	antigravityCapabilityCache[key] = antigravityCapabilityCacheEntry{hints: hints, probeRevision: 1, expiresAt: time.Now().Add(antigravityCapabilityCacheTTL)}
	antigravityCapabilityMu.Unlock()
	// Cache published, but the async publisher has not registered any model yet.
	svc.reconcileRegisteredModelStates(t.Context(), auth)
	current, _ := manager.GetByID(auth.ID)
	if current.ModelStates[model] == nil || !current.ModelStates[model].NextRetryAfter.Equal(retry) {
		t.Fatal("cache publication erased cooldown before registry publication")
	}
	epoch := GlobalModelRegistry().ClientRegistrationEpoch(auth.ID)
	svc.applyAntigravityModelHints(t.Context(), auth, "antigravity", hints, key, auth.RegistrationEpoch, epoch)
	current, _ = manager.GetByID(auth.ID)
	if current.ModelStates[model] == nil || !current.ModelStates[model].NextRetryAfter.Equal(retry) {
		t.Fatal("registry publication lost retained cooldown")
	}
}

func TestAntigravityRegistryPublicationFencesCacheRevision(t *testing.T) {
	resetAntigravityCapabilityCache()
	t.Cleanup(resetAntigravityCapabilityCache)
	manager := coreauth.NewManager(nil, nil, nil)
	svc := &Service{cfg: &config.Config{}, coreManager: manager}
	auth := antigravityTestAuth("publication-version", "http://127.0.0.1:1")
	auth, _ = manager.Register(t.Context(), auth)
	t.Cleanup(func() { GlobalModelRegistry().UnregisterClient(auth.ID) })
	key := svc.antigravityCapabilityKey(auth)
	epoch := GlobalModelRegistry().ClientRegistrationEpoch(auth.ID)
	old := antigravityModelCapabilityHints{ModelIDs: map[string]struct{}{"gemini-3.1-flash-lite": {}}, revision: 1}
	newer := antigravityModelCapabilityHints{ModelIDs: map[string]struct{}{}, revision: 2}
	antigravityCapabilityMu.Lock()
	antigravityCapabilityCache[key] = antigravityCapabilityCacheEntry{hints: newer, probeRevision: 2, expiresAt: time.Now().Add(antigravityCapabilityCacheTTL)}
	antigravityCapabilityMu.Unlock()
	// The old caller resumes after the newer cache publication, with the same registry epoch.
	svc.applyAntigravityModelHints(t.Context(), auth, "antigravity", old, key, auth.RegistrationEpoch, epoch)
	if GlobalModelRegistry().ClientRegistrationEpoch(auth.ID) != epoch {
		t.Fatal("old hints advanced registry epoch")
	}
	svc.applyAntigravityModelHints(t.Context(), auth, "antigravity", newer, key, auth.RegistrationEpoch, epoch)
	if len(GlobalModelRegistry().GetModelsForClient(auth.ID)) != 0 {
		t.Fatal("new empty catalog not applied")
	}
	// A second successful catalog may lose CAS to another publisher. It must
	// retry with a current auth/registry snapshot, never promote its old epoch.
	newest := antigravityModelCapabilityHints{ModelIDs: map[string]struct{}{"gemini-3.1-flash-lite": {}}, revision: 3}
	antigravityCapabilityMu.Lock()
	entry := antigravityCapabilityCache[key]
	entry.hints = newest
	entry.probeRevision = 3
	antigravityCapabilityCache[key] = entry
	antigravityCapabilityMu.Unlock()
	svc.applyAntigravityModelHints(t.Context(), auth, "antigravity", newest, key, auth.RegistrationEpoch, epoch)
	if len(GlobalModelRegistry().GetModelsForClient(auth.ID)) != 0 {
		t.Fatal("stale registry epoch was promoted")
	}
	svc.refreshAntigravityModels(t.Context())
	svc.WaitAntigravityProbes()
	if len(GlobalModelRegistry().GetModelsForClient(auth.ID)) != 1 {
		t.Fatal("periodic retry failed to publish the newer cached catalog")
	}
	// External removal must still win over a cached publication.
	publishedEpoch := GlobalModelRegistry().ClientRegistrationEpoch(auth.ID)
	GlobalModelRegistry().UnregisterClient(auth.ID)
	svc.applyAntigravityModelHints(t.Context(), auth, "antigravity", newest, key, auth.RegistrationEpoch, publishedEpoch)
	if len(GlobalModelRegistry().GetModelsForClient(auth.ID)) != 0 {
		t.Fatal("external unregister overwritten")
	}
}
