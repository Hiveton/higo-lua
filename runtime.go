package higolua

import (
	"context"
	"io"

	"github.com/hiveton/higolua/state"
	"github.com/hiveton/higolua/stdlib"
	"github.com/hiveton/higolua/value"
)

type Runtime struct {
	options []state.Option
}

func NewRuntime(options ...state.Option) *Runtime {
	if len(options) == 0 {
		options = []state.Option{state.WithStdlib(stdlib.Full())}
	}
	return &Runtime{options: options}
}

func (r *Runtime) DoString(ctx context.Context, source string) (value.Value, error) {
	return r.DoChunk(ctx, "string", source)
}

func (r *Runtime) DoChunk(ctx context.Context, name, source string) (value.Value, error) {
	st := state.New(r.options...)
	defer st.Close()
	return st.DoChunk(ctx, name, source)
}

func (r *Runtime) DoFile(ctx context.Context, path string) (value.Value, error) {
	st := state.New(r.options...)
	defer st.Close()
	return st.DoFile(ctx, path)
}

func (r *Runtime) DoReader(ctx context.Context, name string, reader io.Reader) (value.Value, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return value.Nil, err
	}
	return r.DoChunk(ctx, name, string(data))
}
