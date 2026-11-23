package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v8/sdk/cliproxy/executor"
)

func TestIssue7741WrapStreamResultClosesOnCancellationDuringUpstreamStall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	remaining := make(chan cliproxyexecutor.StreamChunk)
	t.Cleanup(func() { close(remaining) })

	hook := newIssue7738ResultHook()
	manager := NewManager(nil, nil, hook)
	result := manager.wrapStreamResult(
		ctx,
		&Auth{ID: "issue7741-auth", Provider: "issue7741"},
		"issue7741",
		"issue7741-model",
		"issue7741-model",
		nil,
		nil,
		remaining,
		OAuthModelAliasResult{},
		false,
		cliproxyexecutor.Options{},
		1,
	)

	go func() {
		remaining <- cliproxyexecutor.StreamChunk{Payload: []byte(`{"type":"response.created"}`)}
	}()

	select {
	case chunk, ok := <-result.Chunks:
		if !ok {
			t.Fatal("output closed before forwarding the first upstream chunk")
		}
		if len(chunk.Payload) == 0 {
			t.Fatal("forwarded chunk has an empty payload")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the first upstream chunk")
	}

	cancel()

	select {
	case _, ok := <-result.Chunks:
		if ok {
			t.Fatal("output produced another chunk after cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("output did not close when context was canceled during upstream stall")
	}

	upstreamErr := &requestScopedStatusError{status: http.StatusBadGateway, message: "upstream rejected the request"}
	select {
	case remaining <- cliproxyexecutor.StreamChunk{Err: upstreamErr}:
	case <-time.After(time.Second):
		t.Fatal("upstream was not drained after cancellation")
	}
	select {
	case <-hook.done:
		results := hook.snapshot()
		if len(results) != 1 || results[0].Success || results[0].Error == nil || results[0].Error.Code != ErrorCodeRequestScoped {
			t.Fatalf("execution results = %+v, want a request-scoped failure", results)
		}
	case <-time.After(time.Second):
		t.Fatal("did not record the upstream failure after cancellation")
	}
}
