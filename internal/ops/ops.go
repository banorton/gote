package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/store"
)

const (
	CodeNotFound = "not_found"
	CodeConflict = "conflict"
	CodeInvalid  = "invalid"
	CodeInternal = "internal"
)

type Error struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

func (e *Error) Error() string { return e.Code + ": " + e.Msg }

func Errorf(code, format string, args ...any) *Error {
	return &Error{Code: code, Msg: fmt.Sprintf(format, args...)}
}

type Env struct {
	Cfg   *config.Config
	Store *store.Store
	Index *index.Indexer
	Now   func() time.Time
}

type Op struct {
	Name string
	Run  func(ctx context.Context, env *Env, req json.RawMessage) (any, error)
}

var (
	mu       sync.RWMutex
	registry = map[string]Op{}
)

// Register adds an op; call from init(). Panics on duplicate names.
func Register(op Op) {
	mu.Lock()
	defer mu.Unlock()
	if op.Name == "" || op.Run == nil {
		panic("ops: op needs a name and Run")
	}
	if _, dup := registry[op.Name]; dup {
		panic("ops: duplicate op " + op.Name)
	}
	registry[op.Name] = op
}

func Lookup(name string) (Op, bool) {
	mu.RLock()
	defer mu.RUnlock()
	op, ok := registry[name]
	return op, ok
}

// Handle registers a typed op. An empty request decodes to the zero Req;
// unknown fields are rejected as invalid.
func Handle[Req, Resp any](name string, fn func(context.Context, *Env, Req) (Resp, error)) {
	Register(Op{Name: name, Run: func(ctx context.Context, env *Env, raw json.RawMessage) (any, error) {
		var req Req
		if len(bytes.TrimSpace(raw)) > 0 {
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				return nil, Errorf(CodeInvalid, "%s: bad request: %v", name, err)
			}
		}
		return fn(ctx, env, req)
	}})
}
