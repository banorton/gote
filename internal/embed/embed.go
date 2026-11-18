package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/banorton/gote/internal/config"
)

type Embedder interface {
	// Model identifies the vector space; vectors from different models never mix.
	Model() string
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

func New(cfg config.Semantic) (Embedder, error) {
	switch cfg.Provider {
	case config.ProviderOpenAI:
		return &openai{cfg: cfg, hc: &http.Client{Timeout: 120 * time.Second}}, nil
	case config.ProviderCommand:
		return &command{cfg: cfg}, nil
	}
	return nil, fmt.Errorf("semantic.provider: must be openai or command, got %q", cfg.Provider)
}

var retryDelays = []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second}

const minRedactLen = 8

const keyCmdTimeout = 10 * time.Second

func excerpt(b []byte, n int) string {
	if len(b) > n {
		b = b[:n]
	}
	return strings.TrimSpace(string(b))
}

type openai struct {
	cfg config.Semantic
	hc  *http.Client

	keyOnce sync.Once
	key     string
	keyErr  error
}

func (o *openai) Model() string {
	if o.cfg.Dimensions > 0 {
		return o.cfg.Model + ":" + strconv.Itoa(o.cfg.Dimensions)
	}
	return o.cfg.Model
}

func (o *openai) apiKey(ctx context.Context) (string, error) {
	o.keyOnce.Do(func() { o.key, o.keyErr = o.resolveKey(ctx) })
	return o.key, o.keyErr
}

func (o *openai) resolveKey(ctx context.Context) (string, error) {
	c := o.cfg
	switch {
	case c.APIKey != "":
		return c.APIKey, nil
	case c.APIKeyEnv != "":
		k := strings.TrimSpace(os.Getenv(c.APIKeyEnv))
		if k == "" {
			return "", fmt.Errorf("semantic.api_key_env: environment variable %s is empty or unset", c.APIKeyEnv)
		}
		return k, nil
	case c.APIKeyCmd != "":
		cctx, cancel := context.WithTimeout(ctx, keyCmdTimeout)
		defer cancel()
		cmd := exec.CommandContext(cctx, "sh", "-c", c.APIKeyCmd)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			if cctx.Err() != nil {
				return "", fmt.Errorf("semantic.api_key_cmd: timed out after %s", keyCmdTimeout)
			}
			return "", fmt.Errorf("semantic.api_key_cmd: %v: %s", err, excerpt(stderr.Bytes(), 200))
		}
		k := strings.TrimSpace(stdout.String())
		if k == "" {
			return "", fmt.Errorf("semantic.api_key_cmd: command printed no key")
		}
		return k, nil
	}
	return "", fmt.Errorf("semantic.api_key: no key source configured")
}

type openaiReq struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type openaiResp struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (o *openai) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	key, err := o.apiKey(ctx)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(openaiReq{Model: o.cfg.Model, Input: texts, Dimensions: o.cfg.Dimensions})
	if err != nil {
		return nil, err
	}
	url := strings.TrimRight(o.cfg.BaseURL, "/") + "/embeddings"
	// Keys shorter than minRedactLen are placeholders for local servers;
	// replacing them would mangle ordinary words in the message.
	redact := func(s string) string {
		if len(key) < minRedactLen {
			return s
		}
		return strings.ReplaceAll(s, key, "***")
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("embeddings request: %s", redact(err.Error()))
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := o.hc.Do(req)
		if err != nil {
			return nil, fmt.Errorf("embeddings request to %s: %s", url, redact(err.Error()))
		}
		data, rerr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if rerr != nil {
			return nil, fmt.Errorf("embeddings response: %s", redact(rerr.Error()))
		}
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if retryable && attempt < len(retryDelays) {
			select {
			case <-time.After(retryDelays[attempt]):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return nil, fmt.Errorf("embeddings request failed: status %d: %s", resp.StatusCode, redact(excerpt(data, 200)))
		}
		return decodeOpenAI(data, len(texts))
	}
}

func decodeOpenAI(data []byte, n int) ([][]float32, error) {
	var r openaiResp
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("embeddings response: invalid JSON: %v", err)
	}
	if len(r.Data) != n {
		return nil, fmt.Errorf("embeddings response: got %d vectors for %d inputs", len(r.Data), n)
	}
	out := make([][]float32, n)
	for _, d := range r.Data {
		if d.Index < 0 || d.Index >= n || out[d.Index] != nil {
			return nil, fmt.Errorf("embeddings response: bad or duplicate index %d", d.Index)
		}
		out[d.Index] = d.Embedding
	}
	return out, nil
}

type command struct{ cfg config.Semantic }

const commandTimeout = 60 * time.Second

func (c *command) Model() string { return c.cfg.Model }

func (c *command) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	in, err := json.Marshal(map[string]any{"model": c.cfg.Model, "texts": texts})
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "sh", "-c", c.cfg.EmbedCmd)
	cmd.Stdin = bytes.NewReader(in)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if cctx.Err() != nil && ctx.Err() == nil {
			return nil, fmt.Errorf("semantic.embed_cmd: timed out after %s", commandTimeout)
		}
		return nil, fmt.Errorf("semantic.embed_cmd: %v: %s", err, excerpt(stderr.Bytes(), 500))
	}
	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("semantic.embed_cmd: invalid JSON output: %v: %s", err, excerpt(stderr.Bytes(), 500))
	}
	if len(out.Embeddings) != len(texts) {
		return nil, fmt.Errorf("semantic.embed_cmd: returned %d embeddings for %d texts: %s", len(out.Embeddings), len(texts), excerpt(stderr.Bytes(), 500))
	}
	return out.Embeddings, nil
}
