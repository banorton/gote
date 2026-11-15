package client

import (
	"context"
	"errors"
	"testing"

	"github.com/banorton/gote/internal/ops"
)

type req struct {
	A string `json:"a"`
}
type resp struct {
	B string `json:"b"`
}

func init() {
	ops.Handle("test.client.ok", func(ctx context.Context, env *ops.Env, r req) (resp, error) {
		return resp{r.A + "!"}, nil
	})
	ops.Handle("test.client.conflict", func(ctx context.Context, env *ops.Env, r req) (resp, error) {
		return resp{}, &ops.Error{Code: ops.CodeConflict, Msg: "mtime changed"}
	})
	ops.Handle("test.client.plain", func(ctx context.Context, env *ops.Env, r req) (resp, error) {
		return resp{}, errors.New("boom")
	})
}

func TestLocalRoundTrip(t *testing.T) {
	var out resp
	if err := Local(&ops.Env{}).Call(context.Background(), "test.client.ok", req{"x"}, &out); err != nil || out.B != "x!" {
		t.Fatalf("%v %+v", err, out)
	}
}

func TestLocalNilReqResp(t *testing.T) {
	if err := Local(&ops.Env{}).Call(context.Background(), "test.client.ok", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestErrorSurvives(t *testing.T) {
	err := Local(&ops.Env{}).Call(context.Background(), "test.client.conflict", req{}, nil)
	var e *ops.Error
	if !errors.As(err, &e) || e.Code != ops.CodeConflict || e.Msg != "mtime changed" {
		t.Fatalf("%v", err)
	}
}

func TestPlainErrorBecomesInternal(t *testing.T) {
	err := Local(&ops.Env{}).Call(context.Background(), "test.client.plain", req{}, nil)
	var e *ops.Error
	if !errors.As(err, &e) || e.Code != ops.CodeInternal || e.Msg != "boom" {
		t.Fatalf("%v", err)
	}
}

func TestUnknownOp(t *testing.T) {
	err := Local(&ops.Env{}).Call(context.Background(), "no.such", nil, nil)
	var e *ops.Error
	if !errors.As(err, &e) || e.Code != ops.CodeNotFound {
		t.Fatalf("%v", err)
	}
}
