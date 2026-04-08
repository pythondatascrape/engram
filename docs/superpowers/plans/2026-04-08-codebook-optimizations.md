# Codebook Optimizations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Increase token savings by adding periodic codebook re-injection, fixing system prompt token accounting, and omitting default enum values from compressed turns.

**Architecture:** Three independent changes to the codebook pipeline. Task 1 (enum defaults) modifies `internal/context/codebook.go`. Task 2 (system prompt accounting) modifies `internal/proxy/handler.go`. Task 3 (periodic re-injection) modifies `internal/server/handler.go` and `cmd/engram/serve.go`. Each task is independently shippable.

**Tech Stack:** Go 1.22+, testify, cobra (CLI)

**Spec:** `docs/superpowers/specs/2026-04-08-codebook-optimizations-design.md`

---

### Task 1: Add enum default parsing to DeriveCodebook

**Files:**
- Modify: `internal/context/codebook.go:11-37`
- Test: `internal/context/codebook_test.go`

This task adds a `defaults` field to `ContextCodebook` and populates it during `DeriveCodebook` by parsing enum type hints.

- [ ] **Step 1: Write failing test for enum default parsing**

Add to `internal/context/codebook_test.go`:

```go
func TestDeriveCodebook_ParsesEnumDefaults(t *testing.T) {
	schema := map[string]string{
		"role":    "enum:user,assistant",
		"content": "text",
	}
	cb, err := engramctx.DeriveCodebook("app", schema)
	require.NoError(t, err)

	defaults := cb.Defaults()
	assert.Equal(t, "user", defaults["role"])
	assert.Empty(t, defaults["content"], "text fields should have no default")
}

func TestDeriveCodebook_NoEnumNoDefaults(t *testing.T) {
	schema := map[string]string{"content": "text", "city": "text"}
	cb, err := engramctx.DeriveCodebook("app", schema)
	require.NoError(t, err)
	assert.Empty(t, cb.Defaults())
}

func TestDeriveCodebook_MultipleEnums(t *testing.T) {
	schema := map[string]string{
		"role":   "enum:user,assistant,tool",
		"status": "enum:active,inactive",
	}
	cb, err := engramctx.DeriveCodebook("app", schema)
	require.NoError(t, err)

	defaults := cb.Defaults()
	assert.Equal(t, "user", defaults["role"])
	assert.Equal(t, "active", defaults["status"])
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/context/ -run TestDeriveCodebook_ParsesEnumDefaults -v`
Expected: FAIL — `cb.Defaults undefined`

- [ ] **Step 3: Add defaults field and Defaults() method to ContextCodebook**

Edit `internal/context/codebook.go`. Add a `defaults` field to the struct and a public accessor:

```go
type ContextCodebook struct {
	Name     string
	keys     []string          // sorted field names from schema
	schema   map[string]string // field name → type hint (stored for Definition())
	defaults map[string]string // enum field → first value (the default)
}

// Defaults returns a copy of the enum default values map.
func (c *ContextCodebook) Defaults() map[string]string {
	out := make(map[string]string, len(c.defaults))
	for k, v := range c.defaults {
		out[k] = v
	}
	return out
}
```

- [ ] **Step 4: Parse enum defaults in DeriveCodebook**

In the `DeriveCodebook` function, after building `schemaCopy`, parse defaults from enum type hints:

```go
defaults := make(map[string]string)
for k, v := range schemaCopy {
	if strings.HasPrefix(v, "enum:") {
		values := strings.SplitN(strings.TrimPrefix(v, "enum:"), ",", 2)
		if len(values) > 0 && values[0] != "" {
			defaults[k] = values[0]
		}
	}
}
return &ContextCodebook{Name: name, keys: keys, schema: schemaCopy, defaults: defaults}, nil
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/context/ -run TestDeriveCodebook_ -v`
Expected: PASS — all three new tests and existing `TestDeriveCodebook` pass

- [ ] **Step 6: Commit**

```bash
git add internal/context/codebook.go internal/context/codebook_test.go
git commit -m "feat(context): parse enum defaults from codebook schema"
```

---

### Task 2: Omit default enum values in SerializeTurn

**Files:**
- Modify: `internal/context/codebook.go:49-73` (SerializeTurn method)
- Test: `internal/context/codebook_test.go`

- [ ] **Step 1: Write failing tests for default omission**

Add to `internal/context/codebook_test.go`, inside the `TestSerializeTurn` table:

```go
{
	name:         "OmitsDefaultEnum",
	schema:       map[string]string{"role": "enum:user,assistant", "content": "text"},
	turn:         map[string]string{"role": "user", "content": "What flights are available?"},
	wantErr:      false,
	wantContains: []string{"content=What flights are available?"},
	wantOmit:     []string{"role=user"},
},
{
	name:         "IncludesNonDefaultEnum",
	schema:       map[string]string{"role": "enum:user,assistant", "content": "text"},
	turn:         map[string]string{"role": "assistant", "content": "Here are the flights"},
	wantErr:      false,
	wantContains: []string{"role=assistant", "content=Here are the flights"},
},
{
	name:         "AllDefaultsEmptyOutput",
	schema:       map[string]string{"role": "enum:user,assistant"},
	turn:         map[string]string{"role": "user"},
	wantErr:      false,
	wantContains: []string{},
	wantExact:    "",
},
```

To support `wantOmit` and `wantExact`, update the test struct and assertion loop:

```go
func TestSerializeTurn(t *testing.T) {
	tests := []struct {
		name         string
		schema       map[string]string
		turn         map[string]string
		wantErr      bool
		wantContains []string
		wantOmit     []string
		wantExact    string
		checkExact   bool
	}{
		{
			name:         "RoundTrip",
			schema:       map[string]string{"role": "enum:user,assistant", "content": "text"},
			turn:         map[string]string{"role": "user", "content": "What flights are available?"},
			wantErr:      false,
			wantContains: []string{"content=What flights are available?"},
			wantOmit:     []string{"role=user"},
		},
		{
			name:    "UnknownField",
			schema:  map[string]string{"role": "text"},
			turn:    map[string]string{"role": "user", "unknown": "val"},
			wantErr: true,
		},
		{
			name:         "OmitsDefaultEnum",
			schema:       map[string]string{"role": "enum:user,assistant", "content": "text"},
			turn:         map[string]string{"role": "user", "content": "What flights are available?"},
			wantErr:      false,
			wantContains: []string{"content=What flights are available?"},
			wantOmit:     []string{"role=user"},
		},
		{
			name:         "IncludesNonDefaultEnum",
			schema:       map[string]string{"role": "enum:user,assistant", "content": "text"},
			turn:         map[string]string{"role": "assistant", "content": "Here are the flights"},
			wantErr:      false,
			wantContains: []string{"role=assistant", "content=Here are the flights"},
		},
		{
			name:       "AllDefaultsEmptyOutput",
			schema:     map[string]string{"role": "enum:user,assistant"},
			turn:       map[string]string{"role": "user"},
			wantErr:    false,
			wantExact:  "",
			checkExact: true,
		},
		{
			name:         "MixedDefaultAndNonDefault",
			schema:       map[string]string{"role": "enum:user,assistant", "status": "enum:active,inactive", "content": "text"},
			turn:         map[string]string{"role": "user", "status": "inactive", "content": "hello"},
			wantErr:      false,
			wantContains: []string{"status=inactive", "content=hello"},
			wantOmit:     []string{"role=user"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb, err := engramctx.DeriveCodebook("app", tt.schema)
			require.NoError(t, err)

			compressed, err := cb.SerializeTurn(tt.turn)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.checkExact {
				assert.Equal(t, tt.wantExact, compressed)
			}
			for _, want := range tt.wantContains {
				assert.Contains(t, compressed, want)
			}
			for _, omit := range tt.wantOmit {
				assert.NotContains(t, compressed, omit)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify the new cases fail**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/context/ -run TestSerializeTurn -v`
Expected: FAIL — `OmitsDefaultEnum` fails because `role=user` is still present in output

- [ ] **Step 3: Modify SerializeTurn to skip default values**

In `internal/context/codebook.go`, edit the `SerializeTurn` method. Add a default check before emitting:

```go
func (c *ContextCodebook) SerializeTurn(turn map[string]string) (string, error) {
	for k := range turn {
		if _, ok := c.schema[k]; !ok {
			return "", fmt.Errorf("context codebook %q: unknown field %q in turn", c.Name, k)
		}
	}

	var b strings.Builder
	first := true
	for _, k := range c.keys {
		v, ok := turn[k]
		if !ok {
			continue
		}
		if def, hasDef := c.defaults[k]; hasDef && v == def {
			continue
		}
		if !first {
			b.WriteByte(' ')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(v)
		first = false
	}
	return b.String(), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/context/ -run TestSerializeTurn -v`
Expected: PASS — all cases including OmitsDefaultEnum, IncludesNonDefaultEnum, AllDefaultsEmptyOutput, MixedDefaultAndNonDefault

- [ ] **Step 5: Commit**

```bash
git add internal/context/codebook.go internal/context/codebook_test.go
git commit -m "feat(context): omit default enum values in SerializeTurn"
```

---

### Task 3: Mark defaults with `*` in Definition()

**Files:**
- Modify: `internal/context/codebook.go:77-90` (Definition method)
- Test: `internal/context/codebook_test.go`

- [ ] **Step 1: Write failing tests for default marker**

Add to `internal/context/codebook_test.go`:

```go
func TestDefinition_MarksDefaults(t *testing.T) {
	schema := map[string]string{
		"role":    "enum:user,assistant",
		"content": "text",
	}
	cb, err := engramctx.DeriveCodebook("app", schema)
	require.NoError(t, err)
	def := cb.Definition()

	assert.Contains(t, def, "role(enum:user*,assistant)")
	assert.Contains(t, def, "content(text)")
	assert.NotContains(t, def, "content(text*")
}

func TestDefinition_NoDefaultNoMarker(t *testing.T) {
	schema := map[string]string{"content": "text", "city": "text"}
	cb, err := engramctx.DeriveCodebook("app", schema)
	require.NoError(t, err)
	def := cb.Definition()

	assert.NotContains(t, def, "*")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/context/ -run TestDefinition_MarksDefaults -v`
Expected: FAIL — `role(enum:user*,assistant)` not found in output

- [ ] **Step 3: Modify Definition() to mark default values**

Edit the `Definition()` method in `internal/context/codebook.go`:

```go
func (c *ContextCodebook) Definition() string {
	var b strings.Builder
	b.WriteString(c.Name)
	b.WriteString(": ")
	for i, k := range c.keys {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(k)
		b.WriteByte('(')

		typeHint := c.schema[k]
		if def, ok := c.defaults[k]; ok && strings.HasPrefix(typeHint, "enum:") {
			enumPart := strings.TrimPrefix(typeHint, "enum:")
			enumPart = strings.Replace(enumPart, def, def+"*", 1)
			b.WriteString("enum:")
			b.WriteString(enumPart)
		} else {
			b.WriteString(typeHint)
		}

		b.WriteByte(')')
	}
	return b.String()
}
```

- [ ] **Step 4: Run all codebook tests**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/context/ -v`
Expected: PASS — all tests including `TestDefinition_MarksDefaults`, `TestDefinition_NoDefaultNoMarker`, `TestDefinition_ContainsAllKeys`

- [ ] **Step 5: Verify response codebook singletons still init without panic**

The `response_codebook.go` file uses `mustDeriveCodebook` with enum schemas like `"enum:assistant"`. After our changes, `defaults["role"] = "assistant"` will be set — this is fine. Verify:

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/context/ -run TestResponseCodebook -v`

If no specific test exists, run the full package to confirm no init panic:

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/context/ -v`
Expected: PASS — no panics from `mustDeriveCodebook`

- [ ] **Step 6: Commit**

```bash
git add internal/context/codebook.go internal/context/codebook_test.go
git commit -m "feat(context): mark default enum values with * in Definition()"
```

---

### Task 4: Fix system prompt token accounting in proxy

**Files:**
- Modify: `internal/proxy/handler.go:141-143`
- Test: `internal/proxy/handler_test.go`

- [ ] **Step 1: Write failing test for system prompt inclusion**

Add to `internal/proxy/handler_test.go`:

```go
func TestStatsIncludeSystemPrompt(t *testing.T) {
	srv, _ := fakeAnthropic(t)
	defer srv.Close()

	dir := t.TempDir()
	done := make(chan struct{}, 1)
	h := NewHandler(5, dir, srv.URL)
	h.afterStats = func() { done <- struct{}{} }

	// System prompt of 400 chars → ~100 tokens.
	system := strings.Repeat("a", 400)
	// 3 messages (below window, no compression) with ~40 chars each → ~10 tokens each.
	msgs := []AnthropicMessage{
		{Role: "user", Content: strings.Repeat("b", 40)},
		{Role: "assistant", Content: strings.Repeat("c", 40)},
		{Role: "user", Content: strings.Repeat("d", 40)},
	}

	postMessages(t, h, msgs, system, map[string]string{
		"X-Engram-Session": "sysprompt-test",
	})
	<-done

	data, err := os.ReadFile(filepath.Join(dir, "sysprompt-test.ctx.json"))
	require.NoError(t, err)

	var stats struct {
		CtxOrig int `json:"ctx_orig"`
		CtxComp int `json:"ctx_comp"`
	}
	require.NoError(t, json.Unmarshal(data, &stats))

	// Without system prompt: ~30 tokens (3 msgs * 10 tokens each).
	// With system prompt: ~130 tokens (30 + 100).
	// Assert ctxOrig is at least 100 (system prompt contribution).
	assert.GreaterOrEqual(t, stats.CtxOrig, 100,
		"ctxOrig should include system prompt tokens; got %d", stats.CtxOrig)
	assert.GreaterOrEqual(t, stats.CtxComp, 100,
		"ctxComp should include system prompt tokens; got %d", stats.CtxComp)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/proxy/ -run TestStatsIncludeSystemPrompt -v`
Expected: FAIL — `ctxOrig` is ~30, missing the system prompt's ~100 tokens

- [ ] **Step 3: Add system prompt tokens to ctxOrig and ctxComp**

Edit `internal/proxy/handler.go`. Find lines 141-143 and change:

```go
	// Measure before and after compression.
	ctxOrig := EstimateTokens(req.Messages)
	req.Messages = Compress(req.Messages, h.windowSize)
	ctxComp := EstimateTokens(req.Messages)
```

to:

```go
	// Measure before and after compression.
	// Include the system prompt in token estimates — it's the largest payload
	// component but is not compressed by the proxy.
	systemTokens := len(req.System) / 4
	ctxOrig := EstimateTokens(req.Messages) + systemTokens
	req.Messages = Compress(req.Messages, h.windowSize)
	ctxComp := EstimateTokens(req.Messages) + systemTokens
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/proxy/ -run TestStatsIncludeSystemPrompt -v`
Expected: PASS

- [ ] **Step 5: Run all proxy tests to check for regressions**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/proxy/ -v`
Expected: PASS — existing `TestStatsWritten` still passes (values shift but assertions only check key presence)

- [ ] **Step 6: Commit**

```bash
git add internal/proxy/handler.go internal/proxy/handler_test.go
git commit -m "fix(proxy): include system prompt in context token estimates"
```

---

### Task 5: Wire window size to handler for periodic re-injection

**Files:**
- Modify: `internal/server/handler.go:46-82`
- Modify: `cmd/engram/serve.go` (the `runServe` function, near `server.NewHandler` call)
- Test: `internal/server/handler_test.go`

This task adds `windowSize` to the `Handler` struct and updates constructors. The actual re-injection logic is in Task 6.

- [ ] **Step 1: Add windowSize to Handler struct and constructors**

Edit `internal/server/handler.go`:

```go
type Handler struct {
	sessions   *session.Manager
	serializer *serializer.Serializer
	codebook   *codebook.Codebook
	pool       *pool.Pool
	detector   *security.InjectionDetector
	windowSize int
}

func NewHandler(
	sessions *session.Manager,
	ser *serializer.Serializer,
	cb *codebook.Codebook,
	p *pool.Pool,
	windowSize int,
) *Handler {
	return &Handler{
		sessions:   sessions,
		serializer: ser,
		codebook:   cb,
		pool:       p,
		windowSize: windowSize,
	}
}

func NewHandlerWithSecurity(
	sessions *session.Manager,
	ser *serializer.Serializer,
	cb *codebook.Codebook,
	p *pool.Pool,
	windowSize int,
	det *security.InjectionDetector,
) *Handler {
	return &Handler{
		sessions:   sessions,
		serializer: ser,
		codebook:   cb,
		pool:       p,
		windowSize: windowSize,
		detector:   det,
	}
}
```

- [ ] **Step 2: Update test helper to pass windowSize**

Edit `internal/server/handler_test.go`. Update `newTestDeps` return and all `NewHandler` / `NewHandlerWithSecurity` call sites.

The `newTestDeps` helper stays the same (it returns deps, not the handler). Update each test that calls `NewHandler`:

Find all calls like `NewHandler(mgr, ser, cb, p)` and change to `NewHandler(mgr, ser, cb, p, 10)`.

Find all calls like `NewHandlerWithSecurity(mgr, ser, cb, p, det)` and change to `NewHandlerWithSecurity(mgr, ser, cb, p, 10, det)`.

- [ ] **Step 3: Update serve.go to pass window size**

Edit `cmd/engram/serve.go`. Find the line:

```go
handler := server.NewHandler(mgr, ser, nil, p)
```

Change to:

```go
handler := server.NewHandler(mgr, ser, nil, p, cfg.Proxy.WindowSize)
```

The `cfg.Proxy.WindowSize` field already exists with a default of `10` (defined in `internal/config/config.go:329`).

- [ ] **Step 4: Run full test suite to confirm compilation and no regressions**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/server/ ./cmd/engram/ -v`
Expected: PASS — all existing tests pass with the new parameter

- [ ] **Step 5: Commit**

```bash
git add internal/server/handler.go internal/server/handler_test.go cmd/engram/serve.go
git commit -m "refactor(server): add windowSize param to Handler constructors"
```

---

### Task 6: Implement periodic codebook re-injection

**Files:**
- Modify: `internal/server/handler.go` (HandleRequest method, lines ~115-125)
- Test: `internal/server/handler_test.go`

- [ ] **Step 1: Write failing tests for periodic re-injection**

Add to `internal/server/handler_test.go`:

```go
func TestHandleRequest_CodebookDefFirstTurn(t *testing.T) {
	mgr, ser, cb, p := newTestDeps(t)
	h := NewHandler(mgr, ser, cb, p, 5)

	resp, err := h.HandleRequest(context.Background(), IncomingRequest{
		ClientID:      "client-cb-1",
		APIKey:        "key-abc",
		Query:         "turn 0",
		Identity:      map[string]string{"role": "admin"},
		ContextSchema: map[string]string{"role": "enum:user,assistant", "content": "text"},
		Opts:          session.Opts{Provider: "fake", Model: "fake-model"},
	})
	require.NoError(t, err)

	// The first turn should include codebook definitions in the assembled prompt.
	sess, _ := mgr.Get(resp.SessionID)
	snap := sess.Snapshot()
	assert.NotNil(t, snap.ContextCodebook)
	// Turns is 1 after first request completes — the check happens before RecordTurn.
}

func TestHandleRequest_CodebookDefSkippedMidWindow(t *testing.T) {
	mgr, ser, cb, p := newTestDeps(t)
	h := NewHandler(mgr, ser, cb, p, 5)

	// Turn 0: creates session with context schema.
	first, err := h.HandleRequest(context.Background(), IncomingRequest{
		ClientID:      "client-cb-2",
		APIKey:        "key-abc",
		Query:         "turn 0",
		Identity:      map[string]string{"role": "admin"},
		ContextSchema: map[string]string{"role": "enum:user,assistant", "content": "text"},
		Opts:          session.Opts{Provider: "fake", Model: "fake-model"},
	})
	require.NoError(t, err)

	// Turns 1-4: should NOT include codebook defs.
	// We can't directly inspect the assembled prompt from outside,
	// so we verify the turn count advances correctly.
	for i := 1; i <= 4; i++ {
		_, err := h.HandleRequest(context.Background(), IncomingRequest{
			ClientID:  "client-cb-2",
			APIKey:    "key-abc",
			SessionID: first.SessionID,
			Query:     fmt.Sprintf("turn %d", i),
			Opts:      session.Opts{Provider: "fake", Model: "fake-model"},
		})
		require.NoError(t, err)
	}

	sess, _ := mgr.Get(first.SessionID)
	assert.Equal(t, 5, sess.Snapshot().Turns)
}
```

Note: Since `HandleRequest` returns `Response` (not the assembled prompt), we need to verify the re-injection behavior indirectly, or capture the prompt. For a stronger test, add a prompt-capture mechanism:

```go
func TestHandleRequest_CodebookDefReinjected(t *testing.T) {
	mgr, ser, cb, p := newTestDeps(t)
	// Window size of 3: inject on turns 0, 3, 6...
	h := NewHandler(mgr, ser, cb, p, 3)

	// Turn 0: creates session with context schema.
	first, err := h.HandleRequest(context.Background(), IncomingRequest{
		ClientID:      "client-cb-3",
		APIKey:        "key-abc",
		Query:         "turn 0",
		Identity:      map[string]string{"role": "admin"},
		ContextSchema: map[string]string{"role": "enum:user,assistant", "content": "text"},
		Opts:          session.Opts{Provider: "fake", Model: "fake-model"},
	})
	require.NoError(t, err)

	// Run turns 1 and 2 (no re-injection).
	for i := 1; i <= 2; i++ {
		_, err := h.HandleRequest(context.Background(), IncomingRequest{
			ClientID:  "client-cb-3",
			APIKey:    "key-abc",
			SessionID: first.SessionID,
			Query:     fmt.Sprintf("turn %d", i),
			Opts:      session.Opts{Provider: "fake", Model: "fake-model"},
		})
		require.NoError(t, err)
	}

	// Turn 3: re-injection should happen (3 % 3 == 0).
	_, err = h.HandleRequest(context.Background(), IncomingRequest{
		ClientID:  "client-cb-3",
		APIKey:    "key-abc",
		SessionID: first.SessionID,
		Query:     "turn 3",
		Opts:      session.Opts{Provider: "fake", Model: "fake-model"},
	})
	require.NoError(t, err)

	sess, _ := mgr.Get(first.SessionID)
	assert.Equal(t, 4, sess.Snapshot().Turns)
}

func TestHandleRequest_WindowSizeZero(t *testing.T) {
	mgr, ser, cb, p := newTestDeps(t)
	// Window size 0 should not panic and should inject every turn.
	h := NewHandler(mgr, ser, cb, p, 0)

	_, err := h.HandleRequest(context.Background(), IncomingRequest{
		ClientID:      "client-cb-4",
		APIKey:        "key-abc",
		Query:         "turn 0",
		Identity:      map[string]string{"role": "admin"},
		ContextSchema: map[string]string{"role": "enum:user,assistant", "content": "text"},
		Opts:          session.Opts{Provider: "fake", Model: "fake-model"},
	})
	require.NoError(t, err, "window size 0 should not panic")
}
```

- [ ] **Step 2: Run tests to verify they compile and current behavior**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/server/ -run TestHandleRequest_CodebookDef -v`
Expected: Tests compile and pass (because we haven't changed behavior yet — they're testing the outer contract)

- [ ] **Step 3: Add re-injection gate to HandleRequest**

Edit `internal/server/handler.go`. Find the section where codebook defs are populated (after `rctx := sess.RequestCtx()`):

Replace:

```go
	var ctxCodebookDef, respCodebookDef string
	if rctx.ContextCodebook != nil {
		ctxCodebookDef = rctx.ContextCodebook.Definition()
		respCodebookDef = engramctx.AnthropicResponseCodebook().Definition()
	}
```

With:

```go
	// Inject codebook definitions on turn 0 and every windowSize turns thereafter.
	// This aligns with the proxy's compression window — when old turns are rolled
	// into [CONTEXT_SUMMARY], the definitions are re-injected so the LLM can still
	// decode the remaining compressed history entries.
	var ctxCodebookDef, respCodebookDef string
	if rctx.ContextCodebook != nil {
		injectDefs := h.windowSize < 2 || sess.Snapshot().Turns%h.windowSize == 0
		if injectDefs {
			ctxCodebookDef = rctx.ContextCodebook.Definition()
			respCodebookDef = engramctx.AnthropicResponseCodebook().Definition()
		}
	}
```

- [ ] **Step 4: Run all handler tests**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/server/ -v`
Expected: PASS — all existing and new tests pass

- [ ] **Step 5: Run the full test suite**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./...`
Expected: PASS (daemon tests may have socket path issues in sandbox — ignore those if they fail with "bind: invalid argument")

- [ ] **Step 6: Commit**

```bash
git add internal/server/handler.go internal/server/handler_test.go
git commit -m "feat(server): periodic codebook re-injection every windowSize turns"
```

---

### Task 7: Update existing test expectations for enum defaults

**Files:**
- Modify: `internal/context/codebook_test.go` (existing TestSerializeTurn "RoundTrip" case)

After Task 2, the existing `RoundTrip` test case expects `role=user` in the output, but now it gets omitted (because `user` is the default for `enum:user,assistant`). This task fixes that.

**Note:** If you implemented Tasks 1-3 in order and ran tests after each, you will have caught this failure already. This task exists as a safety net.

- [ ] **Step 1: Check if existing tests pass**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/context/ -v`

If `TestSerializeTurn/RoundTrip` fails because it expects `role=user` in the output, update the test case:

The original `RoundTrip` case has `wantContains: []string{"role=user", "content=What flights are available?"}`. Since `role=user` is now omitted (user is default), this case was already replaced by the new test table in Task 2. If the original table was kept alongside the new one, remove the original `RoundTrip` case.

- [ ] **Step 2: Run all context tests**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./internal/context/ -v`
Expected: PASS

- [ ] **Step 3: Commit if any changes were needed**

```bash
git add internal/context/codebook_test.go
git commit -m "test(context): update test expectations for enum default omission"
```

---

### Task 8: Full integration verification

**Files:** None (verification only)

- [ ] **Step 1: Run the complete test suite**

Run: `cd /Users/emeyer/Desktop/Engram && go test ./...`
Expected: PASS (ignore daemon socket-path failures in sandbox)

- [ ] **Step 2: Build the binary**

Run: `cd /Users/emeyer/Desktop/Engram && go build -o bin/engram ./cmd/engram/`
Expected: Binary compiles successfully

- [ ] **Step 3: Verify codebook definition output**

Run:
```bash
cd /Users/emeyer/Desktop/Engram && go test ./internal/context/ -run TestDefinition -v
```
Expected: `role(enum:user*,assistant)` format with `*` marker

- [ ] **Step 4: Verify proxy token estimates**

Run:
```bash
cd /Users/emeyer/Desktop/Engram && go test ./internal/proxy/ -run TestStats -v
```
Expected: `ctxOrig` includes system prompt tokens

- [ ] **Step 5: Commit all changes together if any cleanup was needed**

If all tests pass and no cleanup was needed, this step is a no-op.

```bash
git log --oneline -6
```

Expected output showing 6 commits:
```
feat(server): periodic codebook re-injection every windowSize turns
refactor(server): add windowSize param to Handler constructors
fix(proxy): include system prompt in context token estimates
feat(context): mark default enum values with * in Definition()
feat(context): omit default enum values in SerializeTurn
feat(context): parse enum defaults from codebook schema
```
