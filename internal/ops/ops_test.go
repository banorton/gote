package ops

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type echoReq struct {
	N int `json:"n"`
}
type echoResp struct {
	Doubled int `json:"doubled"`
}

func TestRegisterDuplicatePanics(t *testing.T) {
	Handle("test.dup", func(ctx context.Context, env *Env, r echoReq) (echoResp, error) { return echoResp{}, nil })
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	Handle("test.dup", func(ctx context.Context, env *Env, r echoReq) (echoResp, error) { return echoResp{}, nil })
}

func TestHandleRoundTrip(t *testing.T) {
	Handle("test.echo", func(ctx context.Context, env *Env, r echoReq) (echoResp, error) {
		return echoResp{r.N * 2}, nil
	})
	op, ok := Lookup("test.echo")
	if !ok {
		t.Fatal("not found")
	}
	out, err := op.Run(context.Background(), nil, json.RawMessage(`{"n":4}`))
	if err != nil || out.(echoResp).Doubled != 8 {
		t.Fatalf("%v %v", out, err)
	}
	if _, err := op.Run(context.Background(), nil, nil); err != nil {
		t.Fatalf("empty request should decode to zero value: %v", err)
	}
}

func TestHandleBadRequestIsInvalid(t *testing.T) {
	Handle("test.bad", func(ctx context.Context, env *Env, r echoReq) (echoResp, error) { return echoResp{}, nil })
	op, _ := Lookup("test.bad")
	_, err := op.Run(context.Background(), nil, json.RawMessage(`{"n":"x"}`))
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeInvalid {
		t.Fatalf("%v", err)
	}
	_, err = op.Run(context.Background(), nil, json.RawMessage(`{"zzz":1}`))
	if !errors.As(err, &e) || e.Code != CodeInvalid {
		t.Fatalf("unknown field: %v", err)
	}
}

func TestLookupMissing(t *testing.T) {
	if _, ok := Lookup("nope.nope"); ok {
		t.Fatal()
	}
}

func TestErrorFormat(t *testing.T) {
	e := &Error{Code: CodeConflict, Msg: "file changed"}
	if e.Error() != "conflict: file changed" {
		t.Fatal(e.Error())
	}
}
