package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/banorton/gote/internal/config"
)

func init() { retryDelays = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond} }

func openaiCfg(url string) config.Semantic {
	return config.Semantic{Provider: "openai", BaseURL: url, Model: "m", APIKey: "sk-secret-key", BatchSize: 64}
}

func TestOpenAIRequestShapeAndReorder(t *testing.T) {
	var gotBody map[string]any
	var gotAuth, gotPath, gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath, gotCT = r.Header.Get("Authorization"), r.URL.Path, r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte(`{"data":[{"index":1,"embedding":[0,1]},{"index":0,"embedding":[1,0]}]}`))
	}))
	defer srv.Close()
	cfg := openaiCfg(srv.URL + "/v1/")
	cfg.Dimensions = 2
	e, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if e.Model() == "m" {
		t.Fatal("model id must distinguish dimensions")
	}
	out, err := e.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, [][]float32{{1, 0}, {0, 1}}) {
		t.Fatalf("%v", out)
	}
	if gotAuth != "Bearer sk-secret-key" || gotPath != "/v1/embeddings" || gotCT != "application/json" {
		t.Fatal(gotAuth, gotPath, gotCT)
	}
	if gotBody["model"] != "m" || gotBody["dimensions"] != float64(2) || !reflect.DeepEqual(gotBody["input"], []any{"a", "b"}) {
		t.Fatalf("%v", gotBody)
	}
}

func TestOpenAINoDimensionsOmitted(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte(`{"data":[{"index":0,"embedding":[1]}]}`))
	}))
	defer srv.Close()
	e, _ := New(openaiCfg(srv.URL))
	if e.Model() != "m" {
		t.Fatal(e.Model())
	}
	if _, err := e.Embed(context.Background(), []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := gotBody["dimensions"]; ok {
		t.Fatal("dimensions sent")
	}
}

func TestOpenAIRetry429ThenSuccess(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) <= 2 {
			w.WriteHeader(429)
			return
		}
		w.Write([]byte(`{"data":[{"index":0,"embedding":[1]}]}`))
	}))
	defer srv.Close()
	e, _ := New(openaiCfg(srv.URL))
	if _, err := e.Embed(context.Background(), []string{"a"}); err != nil || n.Load() != 3 {
		t.Fatalf("%v calls=%d", err, n.Load())
	}
}

func TestOpenAIRetryExhausted(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		w.WriteHeader(503)
		w.Write([]byte("overloaded"))
	}))
	defer srv.Close()
	e, _ := New(openaiCfg(srv.URL))
	_, err := e.Embed(context.Background(), []string{"a"})
	if err == nil || !strings.Contains(err.Error(), "503") || n.Load() != 4 {
		t.Fatalf("%v calls=%d", err, n.Load())
	}
}

// Some OpenAI-compatible gateways answer 201 Created with a normal embeddings body.
func TestOpenAIAcceptsAny2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"object":"list","data":[{"index":0,"embedding":[3,4]}]}`))
	}))
	defer srv.Close()
	e, _ := New(openaiCfg(srv.URL))
	got, err := e.Embed(context.Background(), []string{"a"})
	if err != nil || len(got) != 1 || len(got[0]) != 2 {
		t.Fatalf("got %v, err %v", got, err)
	}
}

func TestOpenAIErrorExcerptNoKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte("bad key sk-secret-key " + strings.Repeat("x", 500)))
	}))
	defer srv.Close()
	e, _ := New(openaiCfg(srv.URL))
	_, err := e.Embed(context.Background(), []string{"a"})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatal(err)
	}
	if strings.Contains(err.Error(), "sk-secret-key") || len(err.Error()) > 400 {
		t.Fatalf("leak or too long: %v", err)
	}
}

func TestOpenAIBadResponses(t *testing.T) {
	for name, body := range map[string]string{
		"count":    `{"data":[{"index":0,"embedding":[1]}]}`,
		"index":    `{"data":[{"index":0,"embedding":[1]},{"index":5,"embedding":[1]}]}`,
		"dupindex": `{"data":[{"index":0,"embedding":[1]},{"index":0,"embedding":[1]}]}`,
		"garbage":  `nope`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer srv.Close()
			e, _ := New(openaiCfg(srv.URL))
			if _, err := e.Embed(context.Background(), []string{"a", "b"}); err == nil {
				t.Fatal("want error")
			}
		})
	}
}

func okServer(t *testing.T, auth *string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*auth = r.Header.Get("Authorization")
		w.Write([]byte(`{"data":[{"index":0,"embedding":[1]}]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestKeySources(t *testing.T) {
	var auth string
	srv := okServer(t, &auth)
	t.Setenv("GOTE_TEST_KEY", " envkey ")
	cfg := openaiCfg(srv.URL)
	cfg.APIKey, cfg.APIKeyEnv = "", "GOTE_TEST_KEY"
	e, _ := New(cfg)
	if _, err := e.Embed(context.Background(), []string{"a"}); err != nil || auth != "Bearer envkey" {
		t.Fatalf("%v %q", err, auth)
	}

	cfg.APIKeyEnv, cfg.APIKeyCmd = "", "echo '  cmdkey  '"
	e, _ = New(cfg)
	if _, err := e.Embed(context.Background(), []string{"a"}); err != nil || auth != "Bearer cmdkey" {
		t.Fatalf("%v %q", err, auth)
	}
}

func TestKeySourceErrors(t *testing.T) {
	var auth string
	srv := okServer(t, &auth)
	cfg := openaiCfg(srv.URL)
	cfg.APIKey, cfg.APIKeyEnv = "", "GOTE_MISSING_VAR"
	os.Unsetenv("GOTE_MISSING_VAR")
	e, _ := New(cfg)
	if _, err := e.Embed(context.Background(), []string{"a"}); err == nil || !strings.Contains(err.Error(), "GOTE_MISSING_VAR") {
		t.Fatalf("%v", err)
	}
	cfg.APIKeyEnv, cfg.APIKeyCmd = "", "true"
	e, _ = New(cfg)
	if _, err := e.Embed(context.Background(), []string{"a"}); err == nil || !strings.Contains(err.Error(), "api_key_cmd") {
		t.Fatalf("%v", err)
	}
	cfg.APIKeyCmd = "echo nope >&2; exit 3"
	e, _ = New(cfg)
	if _, err := e.Embed(context.Background(), []string{"a"}); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("%v", err)
	}
}

func script(t *testing.T, body string) string {
	p := filepath.Join(t.TempDir(), "emb.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCommandSuccessReadsStdin(t *testing.T) {
	s := script(t, `cat > "$0.in"; echo '{"embeddings":[[1,0],[0,1]]}'`)
	e, err := New(config.Semantic{Provider: "command", Model: "cm", EmbedCmd: s})
	if err != nil {
		t.Fatal(err)
	}
	if e.Model() != "cm" {
		t.Fatal(e.Model())
	}
	out, err := e.Embed(context.Background(), []string{"x", "y"})
	if err != nil || !reflect.DeepEqual(out, [][]float32{{1, 0}, {0, 1}}) {
		t.Fatalf("%v %v", out, err)
	}
	in, _ := os.ReadFile(s + ".in")
	var got struct {
		Model string
		Texts []string
	}
	if err := json.Unmarshal(in, &got); err != nil || got.Model != "cm" || !reflect.DeepEqual(got.Texts, []string{"x", "y"}) {
		t.Fatalf("%s", in)
	}
}

func TestCommandCountMismatch(t *testing.T) {
	s := script(t, `echo '{"embeddings":[[1]]}'`)
	e, _ := New(config.Semantic{Provider: "command", Model: "cm", EmbedCmd: s})
	if _, err := e.Embed(context.Background(), []string{"x", "y"}); err == nil || !strings.Contains(err.Error(), "1") {
		t.Fatalf("%v", err)
	}
}

func TestCommandNonZeroExitIncludesStderr(t *testing.T) {
	s := script(t, `echo "boom happened" >&2; exit 2`)
	e, _ := New(config.Semantic{Provider: "command", Model: "cm", EmbedCmd: s})
	if _, err := e.Embed(context.Background(), []string{"x"}); err == nil || !strings.Contains(err.Error(), "boom happened") {
		t.Fatalf("%v", err)
	}
}

func TestNewRejectsBadProvider(t *testing.T) {
	if _, err := New(config.Semantic{}); err == nil {
		t.Fatal("want error")
	}
}
