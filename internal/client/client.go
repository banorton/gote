package client

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/banorton/gote/internal/ops"
)

type Client interface {
	Call(ctx context.Context, op string, req, resp any) error
}

type local struct{ env *ops.Env }

// Local runs ops in-process but round-trips through JSON so behavior matches a
// remote client.
func Local(env *ops.Env) Client { return &local{env} }

func (l *local) Call(ctx context.Context, name string, req, resp any) error {
	raw, err := json.Marshal(req)
	if err != nil {
		return ops.Errorf(ops.CodeInvalid, "%s: encode request: %v", name, err)
	}
	op, ok := ops.Lookup(name)
	if !ok {
		return ops.Errorf(ops.CodeNotFound, "unknown op %q", name)
	}
	out, err := op.Run(ctx, l.env, raw)
	if err != nil {
		var oe *ops.Error
		if errors.As(err, &oe) {
			return roundTripError(oe)
		}
		return &ops.Error{Code: ops.CodeInternal, Msg: err.Error()}
	}
	data, err := json.Marshal(out)
	if err != nil {
		return ops.Errorf(ops.CodeInternal, "%s: encode response: %v", name, err)
	}
	if resp == nil {
		return nil
	}
	if err := json.Unmarshal(data, resp); err != nil {
		return ops.Errorf(ops.CodeInternal, "%s: decode response: %v", name, err)
	}
	return nil
}

func roundTripError(e *ops.Error) error {
	data, _ := json.Marshal(e)
	var out ops.Error
	if err := json.Unmarshal(data, &out); err != nil {
		return e
	}
	return &out
}
