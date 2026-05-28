package state

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Hiveton/higo-lua/internal/ast"
	"github.com/Hiveton/higo-lua/internal/bytecode"
	"github.com/Hiveton/higo-lua/internal/lexer"
	"github.com/Hiveton/higo-lua/internal/parser"
	"github.com/Hiveton/higo-lua/internal/vm"
	"github.com/Hiveton/higo-lua/stdlib"
	"github.com/Hiveton/higo-lua/value"
)

type GoFunc func(context.Context, Args) (value.Value, error)
type MultiGoFunc func(context.Context, Args) ([]value.Value, error)

const dumpedFunctionPrefix = "\x1bHiGoLuaDump:"

var (
	luaHexNumberPattern     = regexp.MustCompile(`^[+-]?0[xX][0-9A-Fa-f]+`)
	luaDecimalNumberPattern = regexp.MustCompile(`^[+-]?(?:(?:[0-9]+\.?[0-9]*)|(?:\.[0-9]+))(?:[eE][+-]?[0-9]+)?`)
	errorPositionPattern    = regexp.MustCompile(`^(.+):([0-9]+):([0-9]+): `)
)

type Args []value.Value

func (a Args) Get(i int) value.Value {
	if i < 0 || i >= len(a) {
		return value.Nil
	}
	return a[i]
}

func (a Args) Number(i int) float64 {
	n, _ := value.ToNumber(a.Get(i))
	return n
}

func (a Args) String(i int) string { return a.Get(i).String() }

type Option func(*State)

func WithStdlib(profile stdlib.Profile) Option {
	return func(s *State) { s.stdlib = profile }
}

type State struct {
	global          *env
	closed          bool
	stdlib          stdlib.Profile
	stack           []string
	envStack        []*env
	chunkStack      []string
	registry        *value.Table
	hostCalls       map[string]bool
	loadingStdlib   bool
	input           *fileHandle
	output          *fileHandle
	stderr          *fileHandle
	debugHook       value.Value
	debugHookMask   string
	debugHookCount  int
	debugHookTick   int
	debugHookActive bool
	dumpCounter     int
	dumpedFuncs     map[string]value.Value
}

type luaFunc struct {
	params       []string
	vararg       bool
	body         []ast.Stmt
	env          *env
	name         string
	upvalueNames []string
	source       string
	lineDefined  int
}

func (f *luaFunc) Type() value.Type { return value.TypeFunction }
func (f *luaFunc) String() string   { return fmt.Sprintf("function: %p", f) }

func newLuaFunc(params []string, vararg bool, body []ast.Stmt, e *env, name, source string, lineDefined int) *luaFunc {
	return &luaFunc{params: params, vararg: vararg, body: body, env: e, name: name, upvalueNames: envSnapshotNames(e), source: source, lineDefined: lineDefined}
}

func envSnapshotNames(e *env) []string {
	if e == nil {
		return nil
	}
	names := make([]string, 0, len(e.values))
	for name := range e.values {
		if name == "_G" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (f *luaFunc) call(ctx context.Context, s *State, args []value.Value) (value.Value, error) {
	values, err := f.callMulti(ctx, s, args)
	return first(values), err
}

func (f *luaFunc) callMulti(ctx context.Context, s *State, args []value.Value) ([]value.Value, error) {
	frame := f.displayName()
	callEnv := newEnv(f.env)
	s.stack = append(s.stack, frame)
	s.envStack = append(s.envStack, callEnv)
	defer func() {
		s.stack = s.stack[:len(s.stack)-1]
		s.envStack = s.envStack[:len(s.envStack)-1]
	}()
	for i, name := range f.params {
		if i < len(args) {
			callEnv.define(name, args[i])
		} else {
			callEnv.define(name, value.Nil)
		}
	}
	if f.vararg {
		callEnv.varargs = append([]value.Value(nil), args[len(f.params):]...)
		t := value.NewTable()
		for i := len(f.params); i < len(args); i++ {
			t.Append(args[i])
		}
		t.Set(value.String("n"), value.Number(len(args)-len(f.params)))
		callEnv.define("arg", t)
	}
	if err := s.callDebugHook(ctx, "call", -1); err != nil {
		return nil, err
	}
	res, err := s.execBlock(ctx, callEnv, f.body)
	if err != nil {
		return nil, appendRuntimeStack(err, f.displayName())
	}
	if err := s.callDebugHook(ctx, "return", -1); err != nil {
		return nil, appendRuntimeStack(err, f.displayName())
	}
	if res.returned {
		return res.values, nil
	}
	return []value.Value{value.Nil}, nil
}

func (f *luaFunc) displayName() string {
	if f.name == "" {
		return "function"
	}
	return f.name
}

type goFunction struct {
	fn   GoFunc
	name string
}

func (f *goFunction) Type() value.Type { return value.TypeFunction }
func (f *goFunction) String() string   { return fmt.Sprintf("function: %p", f) }

type multiGoFunction struct{ fn MultiGoFunc }

func (f *multiGoFunction) Type() value.Type { return value.TypeFunction }
func (f *multiGoFunction) String() string   { return fmt.Sprintf("function: %p", f) }

type fileHandle struct {
	file   *os.File
	reader *bufio.Reader
	closer io.Closer
	wait   func() error
}

func (f *fileHandle) Type() value.Type { return value.TypeUserData }
func (f *fileHandle) String() string {
	if f == nil || f.file == nil {
		return "file: closed"
	}
	return fmt.Sprintf("file: %p", f.file)
}

type proxyUserData struct {
	metatable *value.Table
}

func (p *proxyUserData) Type() value.Type { return value.TypeUserData }
func (p *proxyUserData) String() string   { return fmt.Sprintf("userdata: %p", p) }

type vmGlobalEnv struct {
	env *env
}

func (g vmGlobalEnv) Get(name string) value.Value { return g.env.get(name) }
func (g vmGlobalEnv) Set(name string, v value.Value) {
	g.env.set(name, v)
}

type coroutineContextKey struct{}

type coroutineResult struct {
	values []value.Value
	err    error
}

type coroutineThread struct {
	fn       value.Value
	mu       sync.Mutex
	status   string
	started  bool
	resumeCh chan []value.Value
	resultCh chan coroutineResult
}

func newCoroutineThread(fn value.Value) *coroutineThread {
	return &coroutineThread{fn: fn, status: "suspended"}
}

func (c *coroutineThread) Type() value.Type { return value.TypeThread }
func (c *coroutineThread) String() string   { return fmt.Sprintf("thread: %p", c) }

func (c *coroutineThread) statusValue() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

func (c *coroutineThread) setStatus(status string) {
	c.mu.Lock()
	c.status = status
	c.mu.Unlock()
}

func (c *coroutineThread) resume(ctx context.Context, s *State, args []value.Value) ([]value.Value, error) {
	c.mu.Lock()
	switch c.status {
	case "dead":
		c.mu.Unlock()
		return []value.Value{value.Bool(false), value.String("cannot resume dead coroutine")}, nil
	case "running":
		c.mu.Unlock()
		return []value.Value{value.Bool(false), value.String("cannot resume running coroutine")}, nil
	}
	if !c.started {
		c.started = true
		c.resumeCh = make(chan []value.Value)
		c.resultCh = make(chan coroutineResult)
		go c.run(ctx, s)
	}
	c.status = "running"
	c.mu.Unlock()

	select {
	case c.resumeCh <- args:
	case <-ctx.Done():
		c.setStatus("dead")
		return []value.Value{value.Bool(false), value.String(ctx.Err().Error())}, nil
	}

	select {
	case result := <-c.resultCh:
		if result.err != nil {
			c.setStatus("dead")
			return []value.Value{value.Bool(false), value.String(result.err.Error())}, nil
		}
		return append([]value.Value{value.Bool(true)}, result.values...), nil
	case <-ctx.Done():
		c.setStatus("dead")
		return []value.Value{value.Bool(false), value.String(ctx.Err().Error())}, nil
	}
}

func (c *coroutineThread) run(ctx context.Context, s *State) {
	runCtx := context.WithValue(ctx, coroutineContextKey{}, c)
	args := <-c.resumeCh
	values, err := s.callValueMulti(runCtx, c.fn, args)
	c.setStatus("dead")
	c.resultCh <- coroutineResult{values: values, err: err}
	close(c.resultCh)
}

func (c *coroutineThread) yield(ctx context.Context, values []value.Value) ([]value.Value, error) {
	c.setStatus("suspended")
	select {
	case c.resultCh <- coroutineResult{values: values}:
	case <-ctx.Done():
		c.setStatus("dead")
		return nil, ctx.Err()
	}
	select {
	case args := <-c.resumeCh:
		c.setStatus("running")
		return args, nil
	case <-ctx.Done():
		c.setStatus("dead")
		return nil, ctx.Err()
	}
}

func New(options ...Option) *State {
	s := &State{global: newEnv(nil), stdlib: stdlib.Full(), registry: value.NewTable(), hostCalls: map[string]bool{}, dumpedFuncs: map[string]value.Value{}}
	s.input = &fileHandle{file: os.Stdin, reader: bufio.NewReader(os.Stdin)}
	s.output = &fileHandle{file: os.Stdout}
	s.stderr = &fileHandle{file: os.Stderr}
	for _, option := range options {
		option(s)
	}
	s.loadingStdlib = true
	s.openStdlib()
	s.loadingStdlib = false
	return s
}

func (s *State) Close() { s.closed = true }

func (s *State) Register(name string, fn GoFunc) {
	s.global.set(name, &goFunction{fn: fn, name: name})
	if !s.loadingStdlib {
		s.hostCalls[name] = true
	}
}

func (s *State) RegisterMulti(name string, fn MultiGoFunc) {
	s.global.set(name, &multiGoFunction{fn: fn})
	if !s.loadingStdlib {
		s.hostCalls[name] = true
	}
}

func (s *State) RegisterModule(name string, funcs map[string]GoFunc) error {
	pkg, ok := s.global.get("package").(*value.Table)
	if !ok {
		return fmt.Errorf("package library is not loaded")
	}
	preload, ok := pkg.Get(value.String("preload")).(*value.Table)
	if !ok {
		return fmt.Errorf("package.preload must be table")
	}
	moduleName := name
	moduleFuncs := make(map[string]GoFunc, len(funcs))
	for fnName, fn := range funcs {
		moduleFuncs[fnName] = fn
	}
	preload.Set(value.String(moduleName), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
		tab := value.NewTable()
		for fnName, fn := range moduleFuncs {
			tab.Set(value.String(fnName), &goFunction{fn: fn, name: moduleName + "." + fnName})
		}
		return tab, nil
	}, name: "package.preload." + moduleName})
	return nil
}

func (s *State) SetGlobal(name string, v value.Value) {
	if v == nil {
		v = value.Nil
	}
	s.global.set(name, v)
}

func (s *State) GetGlobal(name string) (value.Value, error) {
	if s.closed {
		return value.Nil, errors.New("higolua: state is closed")
	}
	return s.global.get(name), nil
}

func (s *State) DoString(ctx context.Context, source string) error {
	_, err := s.DoChunk(ctx, "string", source)
	return err
}

func (s *State) DoReader(ctx context.Context, name string, r io.Reader) (value.Value, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return value.Nil, err
	}
	return s.DoChunk(ctx, name, string(data))
}

func (s *State) DoFile(ctx context.Context, path string) (value.Value, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return value.Nil, err
	}
	return s.DoChunk(ctx, path, string(data))
}

func (s *State) DoChunk(ctx context.Context, name, source string) (out value.Value, err error) {
	values, err := s.DoChunkValues(ctx, name, source)
	return first(values), err
}

func (s *State) DoChunkValues(ctx context.Context, name, source string) (out []value.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			out = []value.Value{value.Nil}
			if e, ok := r.(error); ok {
				err = wrapChunkError(name, e)
			} else {
				err = &RuntimeError{Chunk: name, Err: fmt.Errorf("%v", r)}
			}
		}
	}()
	if s.closed {
		return nil, &RuntimeError{Chunk: name, Err: errors.New("higolua: state is closed")}
	}
	source = stripShebang(source)
	chunk, err := parser.Parse(name, source)
	if err != nil {
		return nil, newSyntaxError(name, err)
	}
	if values, ok, err := s.tryBytecode(ctx, chunk); ok || err != nil {
		if err != nil {
			return nil, wrapChunkError(name, err)
		}
		return values, nil
	}
	chunkEnv := newEnv(s.global)
	s.envStack = append(s.envStack, chunkEnv)
	s.chunkStack = append(s.chunkStack, name)
	defer func() {
		s.envStack = s.envStack[:len(s.envStack)-1]
		s.chunkStack = s.chunkStack[:len(s.chunkStack)-1]
	}()
	res, err := s.execBlock(ctx, chunkEnv, chunk.Block)
	if err != nil {
		return nil, wrapChunkError(name, err)
	}
	if res.returned {
		return res.values, nil
	}
	return []value.Value{value.Nil}, nil
}

func stripShebang(source string) string {
	if !strings.HasPrefix(source, "#!") {
		return source
	}
	newline := strings.IndexByte(source, '\n')
	if newline < 0 {
		return "\n"
	}
	return strings.Repeat(" ", newline) + source[newline:]
}

func (s *State) tryBytecode(ctx context.Context, chunk *ast.Chunk) ([]value.Value, bool, error) {
	if s.debugHook != nil && s.debugHook != value.Nil {
		return nil, false, nil
	}
	proto, err := bytecode.CompileWithHostCallsAndTables(chunk, s.bytecodeHostCalls(), s.bytecodeHostTables())
	if err != nil {
		return nil, false, nil
	}
	if len(proto.Prototypes) > 0 {
		return nil, false, nil
	}
	results, err := vm.New(vm.WithGlobals(vmGlobalEnv{env: s.global}), vm.WithCaller(vmCaller{state: s})).ExecuteValues(ctx, proto)
	if err != nil {
		return nil, true, err
	}
	return results, true, nil
}

func (s *State) bytecodeHostCalls() []string {
	names := make([]string, 0, len(s.hostCalls))
	for name := range s.hostCalls {
		names = append(names, name)
	}
	for _, name := range []string{"pairs", "ipairs", "next", "setmetatable", "getmetatable", "rawget", "rawset", "type", "tostring", "tonumber", "assert", "error", "rawequal", "collectgarbage", "gcinfo"} {
		switch s.global.get(name).(type) {
		case *multiGoFunction, *goFunction:
			names = append(names, name)
		}
	}
	return names
}

func (s *State) bytecodeHostTables() []string {
	names := []string(nil)
	for _, name := range []string{"string", "math", "table", "package", "coroutine", "io", "os", "debug"} {
		if _, ok := s.global.get(name).(*value.Table); ok {
			names = append(names, name)
		}
	}
	return names
}

type vmCaller struct {
	state *State
}

func (c vmCaller) Call(ctx context.Context, fn value.Value, args []value.Value) ([]value.Value, error) {
	return c.state.callValueMulti(ctx, fn, args)
}

func wrapChunkError(chunk string, err error) error {
	if err == nil {
		return nil
	}
	var syntaxErr *SyntaxError
	var runtimeErr *RuntimeError
	var bridgeErr *BridgeError
	var contextErr *ContextError
	if errors.As(err, &runtimeErr) {
		if runtimeErr.Chunk == "" {
			runtimeErr.Chunk = chunk
		}
		return runtimeErr
	}
	if errors.As(err, &syntaxErr) || errors.As(err, &bridgeErr) || errors.As(err, &contextErr) {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &ContextError{Err: err}
	}
	return &RuntimeError{Chunk: chunk, Err: err}
}

func appendRuntimeStack(err error, frame string) error {
	if err == nil {
		return nil
	}
	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) {
		runtimeErr.Stack = append(runtimeErr.Stack, frame)
		return runtimeErr
	}
	return &RuntimeError{Err: err, Stack: []string{frame}}
}

func wrapBridgeError(name string, err error) error {
	if err == nil {
		return nil
	}
	var syntaxErr *SyntaxError
	var runtimeErr *RuntimeError
	var bridgeErr *BridgeError
	var contextErr *ContextError
	if errors.As(err, &syntaxErr) || errors.As(err, &runtimeErr) || errors.As(err, &bridgeErr) || errors.As(err, &contextErr) {
		return err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &ContextError{Err: err}
	}
	return &BridgeError{Name: name, Err: err}
}

func (s *State) Call(ctx context.Context, name string, args ...value.Value) (value.Value, error) {
	values, err := s.CallValues(ctx, name, args...)
	return first(values), err
}

func (s *State) CallValues(ctx context.Context, name string, args ...value.Value) (out []value.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			out = []value.Value{value.Nil}
			if e, ok := r.(error); ok {
				err = &RuntimeError{Err: e}
			} else {
				err = &RuntimeError{Err: fmt.Errorf("%v", r)}
			}
		}
	}()
	return s.callValueMulti(ctx, s.global.get(name), args)
}

type execResult struct {
	returned bool
	broken   bool
	values   []value.Value
}

func (s *State) execBlock(ctx context.Context, e *env, stmts []ast.Stmt) (execResult, error) {
	for _, stmt := range stmts {
		if err := ctx.Err(); err != nil {
			return execResult{}, err
		}
		if err := s.dispatchStatementHooks(ctx, stmt.Position()); err != nil {
			return execResult{}, err
		}
		res, err := s.execStmt(ctx, e, stmt)
		if err != nil || res.returned || res.broken {
			if err != nil {
				err = attachErrorPosition(err, stmt.Position())
			}
			return res, err
		}
	}
	return execResult{}, nil
}

func newSyntaxError(chunk string, err error) *SyntaxError {
	syntaxErr := &SyntaxError{Chunk: chunk, Err: err}
	if line, column, ok := parseErrorPosition(err); ok {
		syntaxErr.Line = line
		syntaxErr.Column = column
	}
	return syntaxErr
}

func parseErrorPosition(err error) (int, int, bool) {
	if err == nil {
		return 0, 0, false
	}
	matches := errorPositionPattern.FindStringSubmatch(err.Error())
	if matches == nil {
		return 0, 0, false
	}
	line, lineErr := strconv.Atoi(matches[2])
	column, columnErr := strconv.Atoi(matches[3])
	if lineErr != nil || columnErr != nil {
		return 0, 0, false
	}
	return line, column, true
}

func attachErrorPosition(err error, pos lexer.Position) error {
	if err == nil || pos.Line <= 0 {
		return err
	}
	var runtimeErr *RuntimeError
	if errors.As(err, &runtimeErr) {
		if runtimeErr.Line == 0 {
			runtimeErr.Line = pos.Line
		}
		if runtimeErr.Column == 0 {
			runtimeErr.Column = pos.Column
		}
		return runtimeErr
	}
	var bridgeErr *BridgeError
	if errors.As(err, &bridgeErr) {
		if bridgeErr.Line == 0 {
			bridgeErr.Line = pos.Line
		}
		if bridgeErr.Column == 0 {
			bridgeErr.Column = pos.Column
		}
		return bridgeErr
	}
	return err
}

func (s *State) dispatchStatementHooks(ctx context.Context, pos lexer.Position) error {
	line := pos.Line
	if line <= 0 {
		line = -1
	}
	if strings.Contains(s.debugHookMask, "l") {
		if err := s.callDebugHook(ctx, "line", line); err != nil {
			return err
		}
	}
	if s.debugHookCount > 0 {
		s.debugHookTick++
		if s.debugHookTick >= s.debugHookCount {
			s.debugHookTick = 0
			if err := s.callDebugHook(ctx, "count", line); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *State) execStmt(ctx context.Context, e *env, stmt ast.Stmt) (execResult, error) {
	switch st := stmt.(type) {
	case *ast.ReturnStmt:
		vals, err := s.evalList(ctx, e, st.Values)
		return execResult{returned: true, values: vals}, err
	case *ast.LocalAssignStmt:
		vals, err := s.evalList(ctx, e, st.Values)
		if err != nil {
			return execResult{}, err
		}
		for i, name := range st.Names {
			e.define(name, valueAt(vals, i))
		}
	case *ast.AssignStmt:
		vals, err := s.evalList(ctx, e, st.Values)
		if err != nil {
			return execResult{}, err
		}
		for i, target := range st.Targets {
			if err := s.assign(ctx, e, target, valueAt(vals, i)); err != nil {
				return execResult{}, err
			}
		}
	case *ast.CallStmt:
		_, err := s.eval(ctx, e, st.Call)
		return execResult{}, err
	case *ast.DoStmt:
		return s.execBlock(ctx, newEnv(e), st.Body)
	case *ast.FunctionStmt:
		fn := newLuaFunc(st.Params, st.Vararg, st.Body, e, st.Name, s.currentChunkName(), st.Position().Line)
		if strings.Contains(st.Name, ".") {
			parts := strings.Split(st.Name, ".")
			base := e.get(parts[0])
			table, ok := base.(*value.Table)
			if !ok {
				table = value.NewTable()
				e.set(parts[0], table)
			}
			for _, part := range parts[1 : len(parts)-1] {
				next := table.Get(value.String(part))
				nextTable, ok := next.(*value.Table)
				if !ok {
					nextTable = value.NewTable()
					table.Set(value.String(part), nextTable)
				}
				table = nextTable
			}
			table.Set(value.String(parts[len(parts)-1]), fn)
		} else {
			e.set(st.Name, fn)
		}
	case *ast.IfStmt:
		cond, err := s.eval(ctx, e, st.Cond)
		if err != nil {
			return execResult{}, err
		}
		if value.IsTruthy(cond) {
			return s.execBlock(ctx, newEnv(e), st.Then)
		}
		return s.execBlock(ctx, newEnv(e), st.Else)
	case *ast.WhileStmt:
		for {
			cond, err := s.eval(ctx, e, st.Cond)
			if err != nil {
				return execResult{}, err
			}
			if !value.IsTruthy(cond) {
				break
			}
			res, err := s.execBlock(ctx, newEnv(e), st.Body)
			if err != nil || res.returned {
				return res, err
			}
			if res.broken {
				break
			}
		}
	case *ast.RepeatStmt:
		for {
			bodyEnv := newEnv(e)
			res, err := s.execBlock(ctx, bodyEnv, st.Body)
			if err != nil || res.returned {
				return res, err
			}
			if res.broken {
				break
			}
			cond, err := s.eval(ctx, bodyEnv, st.Cond)
			if err != nil {
				return execResult{}, err
			}
			if value.IsTruthy(cond) {
				break
			}
		}
	case *ast.ForStmt:
		start, err := s.numberExpr(ctx, e, st.Start)
		if err != nil {
			return execResult{}, err
		}
		end, err := s.numberExpr(ctx, e, st.End)
		if err != nil {
			return execResult{}, err
		}
		step, err := s.numberExpr(ctx, e, st.Step)
		if err != nil {
			return execResult{}, err
		}
		for i := start; (step >= 0 && i <= end) || (step < 0 && i >= end); i += step {
			loopEnv := newEnv(e)
			loopEnv.define(st.Name, value.Number(i))
			res, err := s.execBlock(ctx, loopEnv, st.Body)
			if err != nil || res.returned {
				return res, err
			}
			if res.broken {
				break
			}
		}
	case *ast.GenericForStmt:
		vals, err := s.evalList(ctx, e, st.Exprs)
		if err != nil {
			return execResult{}, err
		}
		if len(vals) == 0 {
			break
		}
		iter := valueAt(vals, 0)
		stateValue := valueAt(vals, 1)
		control := valueAt(vals, 2)
		for {
			nextValues, err := s.callValueMulti(ctx, iter, []value.Value{stateValue, control})
			if err != nil {
				return execResult{}, err
			}
			nextControl := first(nextValues)
			if nextControl == value.Nil {
				break
			}
			control = nextControl
			loopEnv := newEnv(e)
			for i, name := range st.Names {
				loopEnv.define(name, valueAt(nextValues, i))
			}
			res, err := s.execBlock(ctx, loopEnv, st.Body)
			if err != nil || res.returned {
				return res, err
			}
			if res.broken {
				break
			}
		}
	case *ast.BreakStmt:
		return execResult{broken: true}, nil
	}
	return execResult{}, nil
}

func (s *State) assign(ctx context.Context, e *env, target ast.Expr, v value.Value) error {
	switch t := target.(type) {
	case *ast.NameExpr:
		e.set(t.Name, v)
	case *ast.IndexExpr:
		base, err := s.eval(ctx, e, t.X)
		if err != nil {
			return err
		}
		key, err := s.eval(ctx, e, t.Key)
		if err != nil {
			return err
		}
		table, ok := base.(*value.Table)
		if !ok {
			return fmt.Errorf("attempt to index non-table %s", base.Type())
		}
		return s.assignTable(ctx, table, key, v)
	default:
		return errors.New("invalid assignment target")
	}
	return nil
}

func (s *State) assignTable(ctx context.Context, table *value.Table, key value.Value, v value.Value) error {
	if table.RawGet(key) != value.Nil {
		table.RawSet(key, v)
		return nil
	}
	if table.Metatable() != nil {
		newIndex := table.Metatable().RawGet(value.String("__newindex"))
		switch target := newIndex.(type) {
		case *value.Table:
			return s.assignTable(ctx, target, key, v)
		case *luaFunc, *goFunction, *multiGoFunction:
			_, err := s.callValue(ctx, target, []value.Value{table, key, v})
			return err
		default:
			if newIndex != value.Nil {
				return fmt.Errorf("__newindex must be table or function")
			}
		}
	}
	table.RawSet(key, v)
	return nil
}

func (s *State) evalList(ctx context.Context, e *env, exprs []ast.Expr) ([]value.Value, error) {
	vals := make([]value.Value, 0, len(exprs))
	for i, expr := range exprs {
		if i == len(exprs)-1 {
			multi, err := s.evalMulti(ctx, e, expr)
			if err != nil {
				return nil, err
			}
			vals = append(vals, multi...)
			continue
		}
		v, err := s.eval(ctx, e, expr)
		if err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	return vals, nil
}

func (s *State) evalMulti(ctx context.Context, e *env, expr ast.Expr) ([]value.Value, error) {
	if _, ok := expr.(*ast.VarargExpr); ok {
		return e.varargValues(), nil
	}
	if call, ok := expr.(*ast.CallExpr); ok {
		fn, err := s.eval(ctx, e, call.Fn)
		if err != nil {
			return nil, err
		}
		args, err := s.evalList(ctx, e, call.Args)
		if err != nil {
			return nil, err
		}
		return s.callValueMulti(ctx, fn, args)
	}
	v, err := s.eval(ctx, e, expr)
	if err != nil {
		return nil, err
	}
	return []value.Value{v}, nil
}

func (s *State) eval(ctx context.Context, e *env, expr ast.Expr) (value.Value, error) {
	if err := ctx.Err(); err != nil {
		return value.Nil, err
	}
	switch ex := expr.(type) {
	case *ast.LiteralExpr:
		switch ex.Kind {
		case "number":
			n, err := lexer.ParseNumber(ex.Value)
			if err != nil {
				return value.Nil, err
			}
			return value.Number(n), nil
		case "string":
			return value.String(ex.Value), nil
		case "true":
			return value.Bool(true), nil
		case "false":
			return value.Bool(false), nil
		default:
			return value.Nil, nil
		}
	case *ast.NameExpr:
		return e.get(ex.Name), nil
	case *ast.VarargExpr:
		return first(e.varargValues()), nil
	case *ast.UnaryExpr:
		x, err := s.eval(ctx, e, ex.X)
		if err != nil {
			return value.Nil, err
		}
		switch ex.Op {
		case "-":
			n, ok := value.ToNumber(x)
			if !ok {
				if t, tableOK := x.(*value.Table); tableOK && t.Metatable() != nil {
					unm := t.Metatable().RawGet(value.String("__unm"))
					if unm != value.Nil {
						return s.callValue(ctx, unm, []value.Value{t})
					}
				}
				return value.Nil, fmt.Errorf("attempt to negate %s", x.Type())
			}
			return value.Number(-n), nil
		case "not":
			return value.Bool(!value.IsTruthy(x)), nil
		case "#":
			if t, ok := x.(*value.Table); ok {
				if t.Metatable() != nil {
					lenFn := t.Metatable().RawGet(value.String("__len"))
					if lenFn != value.Nil {
						return s.callValue(ctx, lenFn, []value.Value{t})
					}
				}
				return value.Number(t.Len()), nil
			}
			return value.Number(len(x.String())), nil
		}
	case *ast.BinaryExpr:
		return s.evalBinary(ctx, e, ex)
	case *ast.IndexExpr:
		base, err := s.eval(ctx, e, ex.X)
		if err != nil {
			return value.Nil, err
		}
		key, err := s.eval(ctx, e, ex.Key)
		if err != nil {
			return value.Nil, err
		}
		if t, ok := base.(*value.Table); ok {
			return t.Get(key), nil
		}
		if _, ok := base.(value.String); ok {
			if stringTable, ok := s.global.get("string").(*value.Table); ok {
				return stringTable.Get(key), nil
			}
		}
		if f, ok := base.(*fileHandle); ok {
			return s.fileMethod(f, key.String()), nil
		}
		return value.Nil, fmt.Errorf("attempt to index %s", base.Type())
	case *ast.CallExpr:
		fn, err := s.eval(ctx, e, ex.Fn)
		if err != nil {
			return value.Nil, err
		}
		args, err := s.evalList(ctx, e, ex.Args)
		if err != nil {
			return value.Nil, err
		}
		return s.callValue(ctx, fn, args)
	case *ast.FunctionExpr:
		return newLuaFunc(ex.Params, ex.Vararg, ex.Body, e, "function", s.currentChunkName(), ex.Position().Line), nil
	case *ast.TableExpr:
		t := value.NewTable()
		for i, field := range ex.Fields {
			if field.Key == nil && i == len(ex.Fields)-1 {
				values, err := s.evalMulti(ctx, e, field.Value)
				if err != nil {
					return value.Nil, err
				}
				for _, v := range values {
					t.Append(v)
				}
				continue
			}
			v, err := s.eval(ctx, e, field.Value)
			if err != nil {
				return value.Nil, err
			}
			if field.Key == nil {
				t.Append(v)
				continue
			}
			k, err := s.eval(ctx, e, field.Key)
			if err != nil {
				return value.Nil, err
			}
			t.Set(k, v)
		}
		return t, nil
	}
	return value.Nil, fmt.Errorf("unsupported expression %T", expr)
}

func (s *State) evalBinary(ctx context.Context, e *env, ex *ast.BinaryExpr) (value.Value, error) {
	if ex.Op == "and" {
		left, err := s.eval(ctx, e, ex.Left)
		if err != nil || !value.IsTruthy(left) {
			return left, err
		}
		return s.eval(ctx, e, ex.Right)
	}
	if ex.Op == "or" {
		left, err := s.eval(ctx, e, ex.Left)
		if err != nil || value.IsTruthy(left) {
			return left, err
		}
		return s.eval(ctx, e, ex.Right)
	}
	left, err := s.eval(ctx, e, ex.Left)
	if err != nil {
		return value.Nil, err
	}
	right, err := s.eval(ctx, e, ex.Right)
	if err != nil {
		return value.Nil, err
	}
	switch ex.Op {
	case "..":
		if meta, ok, err := s.callBinaryMetamethod(ctx, "__concat", left, right); ok || err != nil {
			return meta, err
		}
		return value.String(left.String() + right.String()), nil
	case "==":
		if meta, ok, err := s.callEqMetamethod(ctx, left, right); ok || err != nil {
			return value.Bool(value.IsTruthy(meta)), err
		}
		return value.Bool(value.Equal(left, right)), nil
	case "~=":
		if meta, ok, err := s.callEqMetamethod(ctx, left, right); ok || err != nil {
			return value.Bool(!value.IsTruthy(meta)), err
		}
		return value.Bool(!value.Equal(left, right)), nil
	case "<", "<=", ">", ">=":
		metaName := map[string]string{
			"<":  "__lt",
			"<=": "__le",
			">":  "__lt",
			">=": "__le",
		}[ex.Op]
		metaLeft, metaRight := left, right
		if ex.Op == ">" || ex.Op == ">=" {
			metaLeft, metaRight = right, left
		}
		if meta, ok, err := s.callBinaryMetamethod(ctx, metaName, metaLeft, metaRight); ok || err != nil {
			return value.Bool(value.IsTruthy(meta)), err
		}
		ln, lok := value.ToNumber(left)
		rn, rok := value.ToNumber(right)
		if lok && rok {
			switch ex.Op {
			case "<":
				return value.Bool(ln < rn), nil
			case "<=":
				return value.Bool(ln <= rn), nil
			case ">":
				return value.Bool(ln > rn), nil
			case ">=":
				return value.Bool(ln >= rn), nil
			}
		}
		switch ex.Op {
		case "<":
			return value.Bool(left.String() < right.String()), nil
		case "<=":
			return value.Bool(left.String() <= right.String()), nil
		case ">":
			return value.Bool(left.String() > right.String()), nil
		case ">=":
			return value.Bool(left.String() >= right.String()), nil
		}
	default:
		ln, lok := value.ToNumber(left)
		rn, rok := value.ToNumber(right)
		if !lok || !rok {
			metaName := map[string]string{
				"+": "__add",
				"-": "__sub",
				"*": "__mul",
				"/": "__div",
				"%": "__mod",
				"^": "__pow",
			}[ex.Op]
			if metaName != "" {
				if meta, ok, err := s.callBinaryMetamethod(ctx, metaName, left, right); ok || err != nil {
					return meta, err
				}
			}
			return value.Nil, fmt.Errorf("arithmetic on non-number")
		}
		switch ex.Op {
		case "+":
			return value.Number(ln + rn), nil
		case "-":
			return value.Number(ln - rn), nil
		case "*":
			return value.Number(ln * rn), nil
		case "/":
			return value.Number(ln / rn), nil
		case "%":
			return value.Number(math.Mod(ln, rn)), nil
		case "^":
			return value.Number(math.Pow(ln, rn)), nil
		}
	}
	return value.Nil, fmt.Errorf("unsupported operator %s", ex.Op)
}

func (s *State) callBinaryMetamethod(ctx context.Context, name string, left, right value.Value) (value.Value, bool, error) {
	if fn := binaryMetamethod(name, left, right); fn != nil {
		result, err := s.callValue(ctx, fn, []value.Value{left, right})
		return result, true, err
	}
	return value.Nil, false, nil
}

func (s *State) callEqMetamethod(ctx context.Context, left, right value.Value) (value.Value, bool, error) {
	leftTable, lok := left.(*value.Table)
	rightTable, rok := right.(*value.Table)
	if !lok || !rok || leftTable.Metatable() == nil || rightTable.Metatable() == nil {
		return value.Nil, false, nil
	}
	leftFn := leftTable.Metatable().RawGet(value.String("__eq"))
	rightFn := rightTable.Metatable().RawGet(value.String("__eq"))
	if leftFn == value.Nil || rightFn == value.Nil || leftFn != rightFn {
		return value.Nil, false, nil
	}
	result, err := s.callValue(ctx, leftFn, []value.Value{left, right})
	return result, true, err
}

func binaryMetamethod(name string, values ...value.Value) value.Value {
	for _, v := range values {
		if t, ok := v.(*value.Table); ok && t.Metatable() != nil {
			fn := t.Metatable().RawGet(value.String(name))
			if _, nilFn := fn.(value.NilType); !nilFn {
				return fn
			}
		}
	}
	return nil
}

func (s *State) numberExpr(ctx context.Context, e *env, expr ast.Expr) (float64, error) {
	v, err := s.eval(ctx, e, expr)
	if err != nil {
		return 0, err
	}
	n, ok := value.ToNumber(v)
	if !ok {
		return 0, fmt.Errorf("expected number, got %s", v.Type())
	}
	return n, nil
}

func (s *State) callValue(ctx context.Context, fn value.Value, args []value.Value) (value.Value, error) {
	values, err := s.callValueMulti(ctx, fn, args)
	return first(values), err
}

func (s *State) callValueMulti(ctx context.Context, fn value.Value, args []value.Value) (out []value.Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			out = []value.Value{value.Nil}
			if e, ok := r.(error); ok {
				err = &RuntimeError{Err: e}
			} else {
				err = &RuntimeError{Err: fmt.Errorf("%v", r)}
			}
		}
	}()
	if fn == nil {
		fn = value.Nil
	}
	switch f := fn.(type) {
	case *luaFunc:
		return f.callMulti(ctx, s, args)
	case *goFunction:
		v, err := f.fn(ctx, Args(args))
		return []value.Value{v}, wrapBridgeError(f.name, err)
	case *multiGoFunction:
		values, err := f.fn(ctx, Args(args))
		return values, wrapBridgeError("", err)
	case *value.Table:
		if f.Metatable() != nil {
			call := f.Metatable().RawGet(value.String("__call"))
			if _, ok := call.(value.NilType); !ok {
				withSelf := append([]value.Value{f}, args...)
				return s.callValueMulti(ctx, call, withSelf)
			}
		}
		return nil, fmt.Errorf("attempt to call table")
	default:
		return nil, fmt.Errorf("attempt to call %s", fn.Type())
	}
}

func (s *State) loadedChunk(name, source string) []value.Value {
	if strings.HasPrefix(source, dumpedFunctionPrefix) {
		if fn, ok := s.dumpedFuncs[source]; ok {
			return []value.Value{fn}
		}
		return []value.Value{value.Nil, value.String("unknown dumped function")}
	}
	if _, err := parser.Parse(name, source); err != nil {
		return []value.Value{value.Nil, value.String(err.Error())}
	}
	fn := &multiGoFunction{fn: func(ctx context.Context, callArgs Args) ([]value.Value, error) {
		return s.DoChunkValues(ctx, name, source)
	}}
	return []value.Value{fn}
}

func (s *State) readLoadSource(ctx context.Context, reader value.Value) (string, error) {
	var source strings.Builder
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		piece, err := s.callValue(ctx, reader, nil)
		if err != nil {
			return "", err
		}
		if piece == nil {
			piece = value.Nil
		}
		if _, ok := piece.(value.NilType); ok {
			return source.String(), nil
		}
		s, ok := piece.(value.String)
		if !ok {
			return "", fmt.Errorf("reader function must return a string or nil, got %s", piece.Type())
		}
		source.WriteString(string(s))
	}
}

func (s *State) gsubReplacement(ctx context.Context, source string, indexes []int, replacement value.Value) (string, error) {
	if replacement == nil {
		replacement = value.Nil
	}
	switch repl := replacement.(type) {
	case value.String:
		return luaReplacement(source, indexes, string(repl))
	case *value.Table:
		key := gsubReplacementKey(source, indexes)
		v := repl.Get(key)
		if !value.IsTruthy(v) {
			return source[indexes[0]:indexes[1]], nil
		}
		return v.String(), nil
	case *luaFunc, *goFunction, *multiGoFunction:
		values, err := s.callValueMulti(ctx, repl, gsubCaptureValues(source, indexes))
		if err != nil {
			return "", err
		}
		v := first(values)
		if !value.IsTruthy(v) {
			return source[indexes[0]:indexes[1]], nil
		}
		return v.String(), nil
	default:
		return repl.String(), nil
	}
}

func gsubReplacementKey(source string, indexes []int) value.Value {
	if len(indexes) > 2 && indexes[2] >= 0 {
		return value.String(source[indexes[2]:indexes[3]])
	}
	return value.String(source[indexes[0]:indexes[1]])
}

func gsubCaptureValues(source string, indexes []int) []value.Value {
	if len(indexes) <= 2 {
		return []value.Value{value.String(source[indexes[0]:indexes[1]])}
	}
	values := make([]value.Value, 0, len(indexes)/2-1)
	for i := 2; i < len(indexes); i += 2 {
		if indexes[i] < 0 {
			values = append(values, value.Nil)
			continue
		}
		values = append(values, value.String(source[indexes[i]:indexes[i+1]]))
	}
	return values
}

func (s *State) installGlobalTable() {
	globals := s.globalTable()
	for name, v := range s.global.values {
		globals.RawSet(value.String(name), v)
	}
	globals.RawSet(value.String("_G"), globals)
	s.global.values["_G"] = globals
}

func (s *State) globalTable() *value.Table {
	if globals, ok := s.global.values["_G"].(*value.Table); ok {
		return globals
	}
	globals := value.NewTable()
	s.global.values["_G"] = globals
	return globals
}

func (s *State) envAtLevel(level int) *env {
	if level < 1 || level > len(s.envStack) {
		return nil
	}
	return s.envStack[len(s.envStack)-level]
}

func tableToEnv(t *value.Table) *env {
	e := newEnv(nil)
	e.table = t
	for _, entry := range t.Entries() {
		if key, ok := entry[0].(value.String); ok {
			e.values[string(key)] = entry[1]
		}
	}
	return e
}

func envToTable(e, global *env) *value.Table {
	if e == global {
		if globals, ok := global.values["_G"].(*value.Table); ok {
			return globals
		}
	}
	t := value.NewTable()
	for cur := e; cur != nil; cur = cur.parent {
		for name, v := range cur.values {
			t.RawSet(value.String(name), v)
		}
	}
	return t
}

func (s *State) ensureModuleTable(name string, loaded *value.Table) (*value.Table, error) {
	key := value.String(name)
	if existing := loaded.RawGet(key); existing != value.Nil {
		table, ok := existing.(*value.Table)
		if !ok {
			return nil, fmt.Errorf("package.loaded[%s] is not a table", name)
		}
		return table, nil
	}
	moduleTable := value.NewTable()
	loaded.RawSet(key, moduleTable)
	parts := strings.Split(name, ".")
	current := s.globalTable()
	for _, part := range parts[:len(parts)-1] {
		next := current.RawGet(value.String(part))
		if next == value.Nil {
			table := value.NewTable()
			current.RawSet(value.String(part), table)
			current = table
			continue
		}
		table, ok := next.(*value.Table)
		if !ok {
			return nil, fmt.Errorf("module parent %s is not a table", part)
		}
		current = table
	}
	current.RawSet(value.String(parts[len(parts)-1]), moduleTable)
	return moduleTable, nil
}

func modulePackageName(name string) string {
	idx := strings.LastIndex(name, ".")
	if idx < 0 {
		return ""
	}
	return name[:idx+1]
}

func readLuaNumber(reader *bufio.Reader) (float64, bool, error) {
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return 0, false, nil
			}
			return 0, false, err
		}
		if !isLuaSpace(b) {
			if err := reader.UnreadByte(); err != nil {
				return 0, false, err
			}
			break
		}
	}

	candidate, err := peekLuaNumberCandidate(reader)
	if err != nil {
		return 0, false, err
	}
	if candidate == "" {
		return 0, false, nil
	}
	text := luaNumberPrefix(candidate)
	if text == "" {
		return 0, false, nil
	}
	n, err := parseLuaNumber(text)
	if err != nil {
		return 0, false, nil
	}
	if _, err := reader.Discard(len(text)); err != nil {
		return 0, false, err
	}
	return n, true, nil
}

func peekLuaNumberCandidate(reader *bufio.Reader) (string, error) {
	var candidate []byte
	for i := 1; ; i++ {
		peeked, err := reader.Peek(i)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, bufio.ErrBufferFull) {
				break
			}
			return "", err
		}
		b := peeked[len(peeked)-1]
		if isLuaSpace(b) {
			break
		}
		candidate = append(candidate, b)
	}
	return string(candidate), nil
}

func luaNumberPrefix(candidate string) string {
	if match := luaHexNumberPattern.FindString(candidate); match != "" {
		return match
	}
	return luaDecimalNumberPattern.FindString(candidate)
}

func parseLuaNumber(text string) (float64, error) {
	sign := 1.0
	unsigned := text
	if strings.HasPrefix(unsigned, "+") {
		unsigned = unsigned[1:]
	} else if strings.HasPrefix(unsigned, "-") {
		sign = -1
		unsigned = unsigned[1:]
	}
	if strings.HasPrefix(unsigned, "0x") || strings.HasPrefix(unsigned, "0X") {
		n, err := strconv.ParseInt(unsigned[2:], 16, 64)
		return sign * float64(n), err
	}
	return strconv.ParseFloat(text, 64)
}

func isLuaSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' || b == '\v'
}

func readFileFormat(handle *fileHandle, format value.Value) (value.Value, error) {
	formatText := format.String()
	if formatText == "nil" {
		formatText = "*l"
	}
	switch formatText {
	case "*a", "*all":
		if handle.reader == nil {
			handle.reader = bufio.NewReader(handle.file)
		}
		data, err := io.ReadAll(handle.reader)
		if err != nil {
			return value.Nil, err
		}
		if len(data) == 0 {
			return value.Nil, nil
		}
		return value.String(string(data)), nil
	case "*l", "*line":
		if handle.reader == nil {
			handle.reader = bufio.NewReader(handle.file)
		}
		line, err := handle.reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return value.Nil, err
		}
		if len(line) == 0 && errors.Is(err, io.EOF) {
			return value.Nil, nil
		}
		return value.String(strings.TrimRight(line, "\r\n")), nil
	case "*n", "*number":
		if handle.reader == nil {
			handle.reader = bufio.NewReader(handle.file)
		}
		n, ok, err := readLuaNumber(handle.reader)
		if err != nil {
			return value.Nil, err
		}
		if !ok {
			return value.Nil, nil
		}
		return value.Number(n), nil
	default:
		n, ok := value.ToNumber(format)
		if !ok {
			return value.Nil, fmt.Errorf("unsupported file:read format %s", formatText)
		}
		buf := make([]byte, int(n))
		read, err := handle.file.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			return value.Nil, err
		}
		if read == 0 && errors.Is(err, io.EOF) {
			return value.Nil, nil
		}
		return value.String(string(buf[:read])), nil
	}
}

func (s *State) fileMethod(handle *fileHandle, name string) value.Value {
	switch name {
	case "read":
		return &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			if handle.file == nil {
				return nil, fmt.Errorf("attempt to use a closed file")
			}
			if len(args) <= 1 {
				v, err := readFileFormat(handle, value.String("*l"))
				if err != nil {
					return nil, err
				}
				return []value.Value{v}, nil
			}
			results := make([]value.Value, 0, len(args)-1)
			for _, format := range args[1:] {
				v, err := readFileFormat(handle, format)
				if err != nil {
					return nil, err
				}
				results = append(results, v)
			}
			return results, nil
		}}
	case "write":
		return &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if handle.file == nil {
				return value.Nil, fmt.Errorf("attempt to use a closed file")
			}
			for _, arg := range args[1:] {
				if _, err := handle.file.WriteString(arg.String()); err != nil {
					return value.Nil, err
				}
			}
			return handle, nil
		}}
	case "flush":
		return &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if handle.file == nil {
				return value.Nil, fmt.Errorf("attempt to use a closed file")
			}
			return value.Bool(true), handle.file.Sync()
		}}
	case "close":
		return &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if handle.file == nil {
				return value.Bool(true), nil
			}
			err := closeFileHandle(handle)
			handle.file = nil
			handle.reader = nil
			handle.closer = nil
			handle.wait = nil
			if err != nil {
				return value.Nil, err
			}
			return value.Bool(true), nil
		}}
	case "setvbuf":
		return &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if handle.file == nil {
				return value.Nil, fmt.Errorf("attempt to use a closed file")
			}
			return value.Bool(true), nil
		}}
	case "lines":
		return &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if handle.file == nil {
				return value.Nil, fmt.Errorf("attempt to use a closed file")
			}
			return fileLineIterator(handle, false), nil
		}}
	case "seek":
		return &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if handle.file == nil {
				return value.Nil, fmt.Errorf("attempt to use a closed file")
			}
			whenceName := args.String(1)
			if whenceName == "nil" {
				whenceName = "cur"
			}
			whence := io.SeekCurrent
			switch whenceName {
			case "set":
				whence = io.SeekStart
			case "cur":
				whence = io.SeekCurrent
			case "end":
				whence = io.SeekEnd
			default:
				return value.Nil, fmt.Errorf("invalid seek whence")
			}
			offset := int64(args.Number(2))
			pos, err := handle.file.Seek(offset, whence)
			if err != nil {
				return value.Nil, err
			}
			handle.reader = bufio.NewReader(handle.file)
			return value.Number(pos), nil
		}}
	}
	return value.Nil
}

func fileLineIterator(handle *fileHandle, closeOnEOF bool) value.Value {
	if handle.reader == nil && handle.file != nil {
		handle.reader = bufio.NewReader(handle.file)
	}
	return &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
		if handle.file == nil || handle.reader == nil {
			return []value.Value{value.Nil}, nil
		}
		line, err := handle.reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if len(line) == 0 && errors.Is(err, io.EOF) {
			if closeOnEOF {
				_ = handle.file.Close()
				handle.file = nil
			}
			return []value.Value{value.Nil}, nil
		}
		if errors.Is(err, io.EOF) && closeOnEOF {
			_ = handle.file.Close()
			handle.file = nil
		}
		return []value.Value{value.String(strings.TrimRight(line, "\r\n"))}, nil
	}}
}

func closeFileHandle(handle *fileHandle) error {
	var errs []string
	if handle.closer != nil {
		if err := handle.closer.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	} else if handle.file != nil {
		if err := handle.file.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if handle.wait != nil {
		if err := handle.wait(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func luaDateTable(t time.Time) *value.Table {
	table := value.NewTable()
	table.Set(value.String("year"), value.Number(t.Year()))
	table.Set(value.String("month"), value.Number(int(t.Month())))
	table.Set(value.String("day"), value.Number(t.Day()))
	table.Set(value.String("hour"), value.Number(t.Hour()))
	table.Set(value.String("min"), value.Number(t.Minute()))
	table.Set(value.String("sec"), value.Number(t.Second()))
	table.Set(value.String("wday"), value.Number(int(t.Weekday())+1))
	table.Set(value.String("yday"), value.Number(t.YearDay()))
	table.Set(value.String("isdst"), value.Bool(isDST(t)))
	return table
}

func luaDateTableInt(table *value.Table, name string, fallback int) int {
	n, ok := value.ToNumber(table.Get(value.String(name)))
	if !ok {
		return fallback
	}
	return int(n)
}

func isDST(t time.Time) bool {
	_, offset := t.Zone()
	jan := time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, t.Location())
	_, janOffset := jan.Zone()
	jul := time.Date(t.Year(), time.July, 1, 0, 0, 0, 0, t.Location())
	_, julOffset := jul.Zone()
	standard := janOffset
	if julOffset < standard {
		standard = julOffset
	}
	return offset != standard
}

func formatLuaDate(format string, t time.Time) string {
	var out strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i == len(format)-1 {
			out.WriteByte(format[i])
			continue
		}
		i++
		switch format[i] {
		case '%':
			out.WriteByte('%')
		case 'a':
			out.WriteString(t.Format("Mon"))
		case 'A':
			out.WriteString(t.Format("Monday"))
		case 'b', 'h':
			out.WriteString(t.Format("Jan"))
		case 'B':
			out.WriteString(t.Format("January"))
		case 'c':
			out.WriteString(t.Format("Mon Jan _2 15:04:05 2006"))
		case 'd':
			out.WriteString(t.Format("02"))
		case 'H':
			out.WriteString(t.Format("15"))
		case 'I':
			out.WriteString(t.Format("03"))
		case 'j':
			fmt.Fprintf(&out, "%03d", t.YearDay())
		case 'm':
			out.WriteString(t.Format("01"))
		case 'M':
			out.WriteString(t.Format("04"))
		case 'p':
			out.WriteString(t.Format("PM"))
		case 'S':
			out.WriteString(t.Format("05"))
		case 'U':
			fmt.Fprintf(&out, "%02d", luaWeekNumber(t, time.Sunday))
		case 'w':
			fmt.Fprintf(&out, "%d", int(t.Weekday()))
		case 'W':
			fmt.Fprintf(&out, "%02d", luaWeekNumber(t, time.Monday))
		case 'x':
			out.WriteString(t.Format("01/02/06"))
		case 'X':
			out.WriteString(t.Format("15:04:05"))
		case 'y':
			out.WriteString(t.Format("06"))
		case 'Y':
			out.WriteString(t.Format("2006"))
		case 'Z':
			out.WriteString(t.Format("MST"))
		default:
			out.WriteByte('%')
			out.WriteByte(format[i])
		}
	}
	return out.String()
}

func luaWeekNumber(t time.Time, first time.Weekday) int {
	yearStart := time.Date(t.Year(), time.January, 1, 0, 0, 0, 0, t.Location())
	offset := (int(first) - int(yearStart.Weekday()) + 7) % 7
	firstWeekStart := yearStart.AddDate(0, 0, offset)
	if t.Before(firstWeekStart) {
		return 0
	}
	return int(t.Sub(firstWeekStart).Hours()/24)/7 + 1
}

func (s *State) openStdlib() {
	defer s.installGlobalTable()

	if s.stdlib.Base {
		s.Register("print", func(ctx context.Context, args Args) (value.Value, error) {
			parts := make([]string, len(args))
			for i, arg := range args {
				rendered, err := s.callValue(ctx, s.global.get("tostring"), []value.Value{arg})
				if err != nil {
					return value.Nil, err
				}
				parts[i] = rendered.String()
			}
			fmt.Println(strings.Join(parts, "\t"))
			return value.Nil, nil
		})
		s.Register("type", func(ctx context.Context, args Args) (value.Value, error) {
			return value.String(args.Get(0).Type()), nil
		})
		s.Register("tostring", func(ctx context.Context, args Args) (value.Value, error) {
			v := args.Get(0)
			if mt := valueMetatable(v); mt != nil {
				fn := mt.RawGet(value.String("__tostring"))
				if fn != value.Nil {
					return s.callValue(ctx, fn, []value.Value{v})
				}
			}
			return value.String(v.String()), nil
		})
		s.Register("tonumber", func(ctx context.Context, args Args) (value.Value, error) {
			if args.Get(1) != value.Nil {
				base := int(args.Number(1))
				if base < 2 || base > 36 {
					return value.Nil, fmt.Errorf("bad base to tonumber")
				}
				parsed, err := strconv.ParseInt(strings.TrimSpace(args.String(0)), base, 64)
				if err != nil {
					return value.Nil, nil
				}
				return value.Number(parsed), nil
			}
			n, ok := value.ToNumber(args.Get(0))
			if !ok {
				return value.Nil, nil
			}
			return value.Number(n), nil
		})
		s.global.set("assert", &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			if !value.IsTruthy(args.Get(0)) {
				msg := args.String(1)
				if msg == "nil" {
					msg = "assertion failed!"
				}
				return nil, errors.New(msg)
			}
			return []value.Value(args), nil
		}})
		s.Register("error", func(ctx context.Context, args Args) (value.Value, error) {
			return value.Nil, &RuntimeError{Err: errors.New(args.String(0))}
		})
		s.Register("rawequal", func(ctx context.Context, args Args) (value.Value, error) {
			return value.Bool(value.Equal(args.Get(0), args.Get(1))), nil
		})
		s.Register("collectgarbage", func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(0), nil
		})
		s.Register("gcinfo", func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(0), nil
		})
		s.global.set("pcall", &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			fn := args.Get(0)
			values, err := s.callValueMulti(ctx, fn, []value.Value(args[1:]))
			if err != nil {
				var exitErr *ExitError
				if errors.As(err, &exitErr) {
					return nil, err
				}
				return []value.Value{value.Bool(false), value.String(err.Error())}, nil
			}
			return append([]value.Value{value.Bool(true)}, values...), nil
		}})
		s.global.set("xpcall", &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			fn := args.Get(0)
			handler := args.Get(1)
			values, err := s.callValueMulti(ctx, fn, nil)
			if err != nil {
				var exitErr *ExitError
				if errors.As(err, &exitErr) {
					return nil, err
				}
				handled, handlerErr := s.callValueMulti(ctx, handler, []value.Value{value.String(err.Error())})
				if handlerErr != nil {
					return []value.Value{value.Bool(false), value.String(handlerErr.Error())}, nil
				}
				return append([]value.Value{value.Bool(false)}, handled...), nil
			}
			return append([]value.Value{value.Bool(true)}, values...), nil
		}})
		s.global.set("select", &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			if len(args) == 0 {
				return nil, errors.New("select expects an index")
			}
			if args.Get(0).String() == "#" {
				return []value.Value{value.Number(len(args) - 1)}, nil
			}
			index := int(args.Number(0))
			count := len(args) - 1
			if index < 0 {
				index = count + index + 1
			}
			if index < 1 {
				return nil, errors.New("select index out of range")
			}
			if index > count {
				return nil, nil
			}
			return append([]value.Value(nil), args[index:]...), nil
		}})
		s.global.set("unpack", &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return nil, fmt.Errorf("unpack expects table")
			}
			start := int(args.Number(1))
			end := int(args.Number(2))
			return t.Values(start, end), nil
		}})
		s.global.set("loadstring", &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			source := args.String(0)
			name := args.String(1)
			if name == "nil" {
				name = "loadstring"
			}
			return s.loadedChunk(name, source), nil
		}})
		s.global.set("load", &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			name := args.String(1)
			if name == "nil" {
				name = "load"
			}
			first := args.Get(0)
			if str, ok := first.(value.String); ok {
				return s.loadedChunk(name, string(str)), nil
			}
			switch first.(type) {
			case *luaFunc, *goFunction, *multiGoFunction, *value.Table:
				source, err := s.readLoadSource(ctx, first)
				if err != nil {
					return []value.Value{value.Nil, value.String(err.Error())}, nil
				}
				return s.loadedChunk(name, source), nil
			default:
				return []value.Value{value.Nil, value.String("load expects a string or reader function")}, nil
			}
		}})
		s.global.set("loadfile", &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			if !s.stdlib.FileLoad {
				return nil, fmt.Errorf("loadfile is disabled by stdlib profile")
			}
			path := args.String(0)
			data, err := os.ReadFile(path)
			if err != nil {
				return []value.Value{value.Nil, value.String(err.Error())}, nil
			}
			source := string(data)
			return s.loadedChunk(path, source), nil
		}})
		s.global.set("dofile", &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			if !s.stdlib.FileLoad {
				return nil, fmt.Errorf("dofile is disabled by stdlib profile")
			}
			data, err := os.ReadFile(args.String(0))
			if err != nil {
				return nil, err
			}
			return s.DoChunkValues(ctx, args.String(0), string(data))
		}})
		s.global.set("next", &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return nil, fmt.Errorf("next expects table")
			}
			entries := t.Entries()
			if len(entries) == 0 {
				return []value.Value{value.Nil}, nil
			}
			current := args.Get(1)
			if _, ok := current.(value.NilType); ok {
				return []value.Value{entries[0][0], entries[0][1]}, nil
			}
			for i, entry := range entries {
				if value.Equal(entry[0], current) {
					if i+1 >= len(entries) {
						return []value.Value{value.Nil}, nil
					}
					return []value.Value{entries[i+1][0], entries[i+1][1]}, nil
				}
			}
			return []value.Value{value.Nil}, nil
		}})
		s.Register("setmetatable", func(ctx context.Context, args Args) (value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("setmetatable expects table")
			}
			if t.Metatable() != nil && t.Metatable().RawGet(value.String("__metatable")) != value.Nil {
				return value.Nil, fmt.Errorf("cannot change a protected metatable")
			}
			if _, ok := args.Get(1).(value.NilType); ok {
				t.SetMetatable(nil)
				return t, nil
			}
			mt, ok := args.Get(1).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("setmetatable expects table or nil")
			}
			t.SetMetatable(mt)
			return t, nil
		})
		s.Register("getmetatable", func(ctx context.Context, args Args) (value.Value, error) {
			mt := valueMetatable(args.Get(0))
			if mt == nil {
				return value.Nil, nil
			}
			if protected := mt.RawGet(value.String("__metatable")); protected != value.Nil {
				return protected, nil
			}
			return mt, nil
		})
		s.Register("newproxy", func(ctx context.Context, args Args) (value.Value, error) {
			p := &proxyUserData{}
			switch source := args.Get(0).(type) {
			case value.Bool:
				if bool(source) {
					p.metatable = value.NewTable()
				}
			case *proxyUserData:
				p.metatable = source.metatable
			case value.NilType:
			default:
				return value.Nil, fmt.Errorf("newproxy expects boolean or proxy userdata")
			}
			return p, nil
		})
		s.Register("getfenv", func(ctx context.Context, args Args) (value.Value, error) {
			target := args.Get(0)
			if _, ok := target.(value.NilType); ok {
				target = value.Number(1)
			}
			if n, ok := target.(value.Number); ok && n == 0 {
				return s.globalTable(), nil
			}
			if n, ok := target.(value.Number); ok {
				frame := s.envAtLevel(int(n))
				if frame == nil {
					return value.Nil, fmt.Errorf("invalid stack level")
				}
				return envToTable(frame.runtimeEnv(), s.global), nil
			}
			if fn, ok := target.(*luaFunc); ok {
				return envToTable(fn.env, s.global), nil
			}
			return s.globalTable(), nil
		})
		s.Register("setfenv", func(ctx context.Context, args Args) (value.Value, error) {
			t, ok := args.Get(1).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("setfenv expects table")
			}
			switch target := args.Get(0).(type) {
			case *luaFunc:
				target.env = tableToEnv(t)
				return target, nil
			case value.Number:
				if target == 0 {
					s.global = tableToEnv(t)
					s.installGlobalTable()
					return value.Number(0), nil
				}
				frame := s.envAtLevel(int(target))
				if frame == nil {
					return value.Nil, fmt.Errorf("invalid stack level")
				}
				frame.setRuntimeEnv(tableToEnv(t))
				return target, nil
			default:
				return value.Nil, fmt.Errorf("setfenv expects a Lua function or stack level")
			}
		})
		s.Register("rawget", func(ctx context.Context, args Args) (value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("rawget expects table")
			}
			return t.RawGet(args.Get(1)), nil
		})
		s.Register("rawset", func(ctx context.Context, args Args) (value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("rawset expects table")
			}
			t.RawSet(args.Get(1), args.Get(2))
			return t, nil
		})
		s.global.set("pairs", &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return nil, fmt.Errorf("pairs expects table")
			}
			iter := &multiGoFunction{fn: func(ctx context.Context, iterArgs Args) ([]value.Value, error) {
				table, ok := iterArgs.Get(0).(*value.Table)
				if !ok {
					return nil, fmt.Errorf("pairs iterator expects table")
				}
				current := iterArgs.Get(1)
				entries := table.Entries()
				if current == value.Nil {
					if len(entries) == 0 {
						return []value.Value{value.Nil}, nil
					}
					return []value.Value{entries[0][0], entries[0][1]}, nil
				}
				for i, entry := range entries {
					if value.Equal(entry[0], current) {
						if i+1 >= len(entries) {
							return []value.Value{value.Nil}, nil
						}
						return []value.Value{entries[i+1][0], entries[i+1][1]}, nil
					}
				}
				return []value.Value{value.Nil}, nil
			}}
			return []value.Value{iter, t, value.Nil}, nil
		}})
		s.global.set("ipairs", &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return nil, fmt.Errorf("ipairs expects table")
			}
			iter := &multiGoFunction{fn: func(ctx context.Context, iterArgs Args) ([]value.Value, error) {
				table, ok := iterArgs.Get(0).(*value.Table)
				if !ok {
					return nil, fmt.Errorf("ipairs iterator expects table")
				}
				next := int(iterArgs.Number(1)) + 1
				v := table.Get(value.Number(next))
				if v == value.Nil {
					return []value.Value{value.Nil}, nil
				}
				return []value.Value{value.Number(next), v}, nil
			}}
			return []value.Value{iter, t, value.Number(0)}, nil
		}})
	}
	if s.stdlib.Math {
		mathTable := value.NewTable()
		mathTable.Set(value.String("pi"), value.Number(math.Pi))
		mathTable.Set(value.String("huge"), value.Number(math.Inf(1)))
		mathTable.Set(value.String("sin"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Sin(args.Number(0))), nil
		}})
		mathTable.Set(value.String("cos"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Cos(args.Number(0))), nil
		}})
		mathTable.Set(value.String("tan"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Tan(args.Number(0))), nil
		}})
		mathTable.Set(value.String("asin"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Asin(args.Number(0))), nil
		}})
		mathTable.Set(value.String("acos"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Acos(args.Number(0))), nil
		}})
		mathTable.Set(value.String("atan"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Atan(args.Number(0))), nil
		}})
		mathTable.Set(value.String("atan2"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Atan2(args.Number(0), args.Number(1))), nil
		}})
		mathTable.Set(value.String("sinh"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Sinh(args.Number(0))), nil
		}})
		mathTable.Set(value.String("cosh"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Cosh(args.Number(0))), nil
		}})
		mathTable.Set(value.String("tanh"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Tanh(args.Number(0))), nil
		}})
		mathTable.Set(value.String("deg"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(args.Number(0) * 180 / math.Pi), nil
		}})
		mathTable.Set(value.String("rad"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(args.Number(0) * math.Pi / 180), nil
		}})
		mathTable.Set(value.String("exp"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Exp(args.Number(0))), nil
		}})
		mathTable.Set(value.String("log"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Log(args.Number(0))), nil
		}})
		mathTable.Set(value.String("log10"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Log10(args.Number(0))), nil
		}})
		mathTable.Set(value.String("pow"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Pow(args.Number(0), args.Number(1))), nil
		}})
		mathTable.Set(value.String("fmod"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Mod(args.Number(0), args.Number(1))), nil
		}})
		mathTable.Set(value.String("mod"), mathTable.Get(value.String("fmod")))
		mathTable.Set(value.String("ldexp"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Ldexp(args.Number(0), int(args.Number(1)))), nil
		}})
		mathTable.Set(value.String("frexp"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			frac, exp := math.Frexp(args.Number(0))
			return []value.Value{value.Number(frac), value.Number(exp)}, nil
		}})
		mathTable.Set(value.String("modf"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			intPart, frac := math.Modf(args.Number(0))
			return []value.Value{value.Number(intPart), value.Number(frac)}, nil
		}})
		mathTable.Set(value.String("floor"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Floor(args.Number(0))), nil
		}})
		mathTable.Set(value.String("ceil"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Ceil(args.Number(0))), nil
		}})
		mathTable.Set(value.String("sqrt"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Sqrt(args.Number(0))), nil
		}})
		mathTable.Set(value.String("abs"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(math.Abs(args.Number(0))), nil
		}})
		mathTable.Set(value.String("max"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if len(args) == 0 {
				return value.Nil, fmt.Errorf("math.max expects at least one argument")
			}
			max := args.Number(0)
			for i := 1; i < len(args); i++ {
				max = math.Max(max, args.Number(i))
			}
			return value.Number(max), nil
		}})
		mathTable.Set(value.String("min"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if len(args) == 0 {
				return value.Nil, fmt.Errorf("math.min expects at least one argument")
			}
			min := args.Number(0)
			for i := 1; i < len(args); i++ {
				min = math.Min(min, args.Number(i))
			}
			return value.Number(min), nil
		}})
		mathTable.Set(value.String("randomseed"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			rand.Seed(int64(args.Number(0)))
			return value.Nil, nil
		}})
		mathTable.Set(value.String("random"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if len(args) == 0 {
				return value.Number(rand.Float64()), nil
			}
			lo := 1
			hi := int(args.Number(0))
			if len(args) >= 2 {
				lo = int(args.Number(0))
				hi = int(args.Number(1))
			}
			if hi < lo {
				return value.Nil, fmt.Errorf("bad argument #2 to random")
			}
			return value.Number(lo + rand.Intn(hi-lo+1)), nil
		}})
		s.SetGlobal("math", mathTable)
	}
	if s.stdlib.String {
		str := value.NewTable()
		str.Set(value.String("len"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(len(args.String(0))), nil
		}})
		str.Set(value.String("upper"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.String(strings.ToUpper(args.String(0))), nil
		}})
		str.Set(value.String("lower"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.String(strings.ToLower(args.String(0))), nil
		}})
		str.Set(value.String("byte"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			text := args.String(0)
			start := 1
			if len(args) >= 2 {
				start = int(args.Number(1))
			}
			end := start
			if len(args) >= 3 {
				end = int(args.Number(2))
			}
			if start < 0 {
				start = len(text) + start + 1
			}
			if end < 0 {
				end = len(text) + end + 1
			}
			if start < 1 {
				start = 1
			}
			if end > len(text) {
				end = len(text)
			}
			if start > end || start > len(text) {
				return []value.Value{value.Nil}, nil
			}
			out := make([]value.Value, 0, end-start+1)
			for i := start - 1; i < end; i++ {
				out = append(out, value.Number(text[i]))
			}
			return out, nil
		}})
		str.Set(value.String("char"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			var b strings.Builder
			for _, arg := range args {
				n, ok := value.ToNumber(arg)
				if !ok || n < 0 || n > 255 || n != math.Trunc(n) {
					return value.Nil, fmt.Errorf("bad argument to string.char")
				}
				b.WriteByte(byte(n))
			}
			return value.String(b.String()), nil
		}})
		str.Set(value.String("reverse"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			text := []byte(args.String(0))
			for i, j := 0, len(text)-1; i < j; i, j = i+1, j-1 {
				text[i], text[j] = text[j], text[i]
			}
			return value.String(string(text)), nil
		}})
		str.Set(value.String("sub"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			text := args.String(0)
			start := int(args.Number(1))
			end := int(args.Number(2))
			if start < 0 {
				start = len(text) + start + 1
			}
			if end == 0 {
				end = len(text)
			}
			if end < 0 {
				end = len(text) + end + 1
			}
			if start < 1 {
				start = 1
			}
			if end > len(text) {
				end = len(text)
			}
			if start > end {
				return value.String(""), nil
			}
			return value.String(text[start-1 : end]), nil
		}})
		str.Set(value.String("rep"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.String(strings.Repeat(args.String(0), int(args.Number(1)))), nil
		}})
		str.Set(value.String("format"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			format, formatArgs := luaFormatArgs(args.String(0), []value.Value(args[1:]))
			return value.String(fmt.Sprintf(format, formatArgs...)), nil
		}})
		str.Set(value.String("dump"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			fn := args.Get(0)
			if _, ok := fn.(*luaFunc); !ok {
				return value.Nil, fmt.Errorf("string.dump expects Lua function")
			}
			s.dumpCounter++
			key := fmt.Sprintf("%s%d", dumpedFunctionPrefix, s.dumpCounter)
			s.dumpedFuncs[key] = fn
			return value.String(key), nil
		}})
		str.Set(value.String("match"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			source := args.String(0)
			pattern := args.String(1)
			init := luaStringStart(source, int(args.Number(2)))
			if indexes, ok, err := luaFrontierMatches(source, init, pattern); ok || err != nil {
				if err != nil {
					return nil, err
				}
				if len(indexes) == 0 {
					return []value.Value{value.Nil}, nil
				}
				match := indexes[0]
				return []value.Value{value.String(source[match[0]:match[1]])}, nil
			}
			if open, close, ok := parseBalancedPattern(pattern); ok {
				match := firstBalancedMatch(source, init, open, close)
				if match == nil {
					return []value.Value{value.Nil}, nil
				}
				return []value.Value{value.String(source[match[0]:match[1]])}, nil
			}
			re, err := compileLuaPattern(pattern)
			if err != nil {
				return nil, err
			}
			indexes := re.FindStringSubmatchIndex(source[init:])
			if indexes == nil {
				return []value.Value{value.Nil}, nil
			}
			if len(indexes) > 2 {
				out := make([]value.Value, 0, len(indexes)/2-1)
				for i := 2; i < len(indexes); i += 2 {
					if indexes[i] < 0 {
						out = append(out, value.Nil)
						continue
					}
					out = append(out, value.String(source[init+indexes[i]:init+indexes[i+1]]))
				}
				return out, nil
			}
			return []value.Value{value.String(source[init+indexes[0] : init+indexes[1]])}, nil
		}})
		str.Set(value.String("find"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			haystack := args.String(0)
			needle := args.String(1)
			init := luaStringStart(haystack, int(args.Number(2)))
			if value.IsTruthy(args.Get(3)) {
				idx := strings.Index(haystack[init:], needle)
				if idx < 0 {
					return []value.Value{value.Nil}, nil
				}
				start := init + idx
				return []value.Value{value.Number(start + 1), value.Number(start + len(needle))}, nil
			}
			if indexes, ok, err := luaFrontierMatches(haystack, init, needle); ok || err != nil {
				if err != nil {
					return nil, err
				}
				if len(indexes) == 0 {
					return []value.Value{value.Nil}, nil
				}
				match := indexes[0]
				return []value.Value{value.Number(match[0] + 1), value.Number(match[1])}, nil
			}
			if open, close, ok := parseBalancedPattern(needle); ok {
				match := firstBalancedMatch(haystack, init, open, close)
				if match == nil {
					return []value.Value{value.Nil}, nil
				}
				return []value.Value{value.Number(match[0] + 1), value.Number(match[1])}, nil
			}
			re, err := compileLuaPattern(needle)
			if err != nil {
				return nil, err
			}
			indexes := re.FindStringSubmatchIndex(haystack[init:])
			if indexes == nil {
				return []value.Value{value.Nil}, nil
			}
			out := []value.Value{value.Number(init + indexes[0] + 1), value.Number(init + indexes[1])}
			for i := 2; i < len(indexes); i += 2 {
				if indexes[i] < 0 {
					out = append(out, value.Nil)
					continue
				}
				out = append(out, value.String(haystack[init+indexes[i]:init+indexes[i+1]]))
			}
			return out, nil
		}})
		str.Set(value.String("gsub"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			source := args.String(0)
			pattern := args.String(1)
			repl := args.Get(2)
			limit := int(args.Number(3))
			if matches, ok, err := luaFrontierMatches(source, 0, pattern); ok || err != nil {
				if err != nil {
					return nil, err
				}
				if limit > 0 && len(matches) > limit {
					matches = matches[:limit]
				}
				var b strings.Builder
				last := 0
				for _, indexes := range matches {
					b.WriteString(source[last:indexes[0]])
					replacement, err := s.gsubReplacement(ctx, source, indexes, repl)
					if err != nil {
						return nil, err
					}
					b.WriteString(replacement)
					last = indexes[1]
				}
				b.WriteString(source[last:])
				return []value.Value{value.String(b.String()), value.Number(len(matches))}, nil
			}
			if open, close, ok := parseBalancedPattern(pattern); ok {
				matches := balancedMatches(source, 0, open, close)
				if limit > 0 && len(matches) > limit {
					matches = matches[:limit]
				}
				var b strings.Builder
				last := 0
				for _, indexes := range matches {
					b.WriteString(source[last:indexes[0]])
					replacement, err := s.gsubReplacement(ctx, source, indexes, repl)
					if err != nil {
						return nil, err
					}
					b.WriteString(replacement)
					last = indexes[1]
				}
				b.WriteString(source[last:])
				return []value.Value{value.String(b.String()), value.Number(len(matches))}, nil
			}
			re, err := compileLuaPattern(pattern)
			if err != nil {
				return nil, err
			}
			matches := re.FindAllStringSubmatchIndex(source, -1)
			if limit > 0 && len(matches) > limit {
				matches = matches[:limit]
			}
			var b strings.Builder
			last := 0
			for _, indexes := range matches {
				b.WriteString(source[last:indexes[0]])
				replacement, err := s.gsubReplacement(ctx, source, indexes, repl)
				if err != nil {
					return nil, err
				}
				b.WriteString(replacement)
				last = indexes[1]
			}
			b.WriteString(source[last:])
			return []value.Value{value.String(b.String()), value.Number(len(matches))}, nil
		}})
		str.Set(value.String("gmatch"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			source := args.String(0)
			if matches, ok, err := luaFrontierMatches(source, 0, args.String(1)); ok || err != nil {
				if err != nil {
					return value.Nil, err
				}
				index := 0
				return &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
					if index >= len(matches) {
						return []value.Value{value.Nil}, nil
					}
					current := matches[index]
					index++
					return []value.Value{value.String(source[current[0]:current[1]])}, nil
				}}, nil
			}
			if open, close, ok := parseBalancedPattern(args.String(1)); ok {
				matches := balancedMatches(source, 0, open, close)
				index := 0
				return &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
					if index >= len(matches) {
						return []value.Value{value.Nil}, nil
					}
					current := matches[index]
					index++
					return []value.Value{value.String(source[current[0]:current[1]])}, nil
				}}, nil
			}
			re, err := compileLuaPattern(args.String(1))
			if err != nil {
				return value.Nil, err
			}
			matches := re.FindAllStringSubmatchIndex(source, -1)
			index := 0
			return &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
				if index >= len(matches) {
					return []value.Value{value.Nil}, nil
				}
				current := matches[index]
				index++
				if len(current) > 2 {
					out := make([]value.Value, 0, len(current)/2-1)
					for i := 2; i < len(current); i += 2 {
						if current[i] < 0 {
							out = append(out, value.Nil)
							continue
						}
						out = append(out, value.String(source[current[i]:current[i+1]]))
					}
					return out, nil
				}
				return []value.Value{value.String(source[current[0]:current[1]])}, nil
			}}, nil
		}})
		s.SetGlobal("string", str)
	}
	if s.stdlib.Table {
		tab := value.NewTable()
		tab.Set(value.String("insert"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("table.insert expects table")
			}
			if len(args) >= 3 {
				pos := int(args.Number(1))
				if pos < 1 || pos > t.Len()+1 {
					return value.Nil, fmt.Errorf("bad argument #2 to table.insert")
				}
				t.Insert(pos, args.Get(2))
				return value.Nil, nil
			}
			t.Append(args.Get(1))
			return value.Nil, nil
		}})
		tab.Set(value.String("remove"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("table.remove expects table")
			}
			pos := 0
			if args.Get(1) != value.Nil {
				pos = int(args.Number(1))
				if pos < 1 || pos > t.Len() {
					return value.Nil, fmt.Errorf("bad argument #2 to table.remove")
				}
			}
			return t.Remove(pos), nil
		}})
		tab.Set(value.String("sort"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("table.sort expects table")
			}
			if args.Get(1) != value.Nil {
				var compareErr error
				t.SortFunc(func(a, b value.Value) bool {
					if compareErr != nil {
						return false
					}
					result, err := s.callValue(ctx, args.Get(1), []value.Value{a, b})
					if err != nil {
						compareErr = err
						return false
					}
					return value.IsTruthy(result)
				})
				if compareErr != nil {
					return value.Nil, compareErr
				}
				return value.Nil, nil
			}
			t.Sort()
			return value.Nil, nil
		}})
		tab.Set(value.String("concat"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("table.concat expects table")
			}
			sep := args.String(1)
			start := int(args.Number(2))
			if start <= 0 {
				start = 1
			}
			end := int(args.Number(3))
			if end <= 0 || end > t.Len() {
				end = t.Len()
			}
			for i := start; i <= end; i++ {
				v := t.Get(value.Number(i))
				switch v.(type) {
				case value.String, value.Number:
				default:
					return value.Nil, fmt.Errorf("invalid value (%s) at index %d in table for concat", v.Type(), i)
				}
			}
			return value.String(t.Concat(sep, start, end)), nil
		}})
		tab.Set(value.String("getn"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("table.getn expects table")
			}
			return value.Number(t.Len()), nil
		}})
		tab.Set(value.String("maxn"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("table.maxn expects table")
			}
			return value.Number(t.MaxN()), nil
		}})
		tab.Set(value.String("foreachi"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("table.foreachi expects table")
			}
			fn := args.Get(1)
			for i := 1; i <= t.Len(); i++ {
				result, err := s.callValue(ctx, fn, []value.Value{value.Number(i), t.Get(value.Number(i))})
				if err != nil {
					return value.Nil, err
				}
				if result != value.Nil {
					return result, nil
				}
			}
			return value.Nil, nil
		}})
		tab.Set(value.String("foreach"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			t, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("table.foreach expects table")
			}
			fn := args.Get(1)
			for _, entry := range t.Entries() {
				result, err := s.callValue(ctx, fn, []value.Value{entry[0], entry[1]})
				if err != nil {
					return value.Nil, err
				}
				if result != value.Nil {
					return result, nil
				}
			}
			return value.Nil, nil
		}})
		tab.Set(value.String("unpack"), s.global.get("unpack"))
		s.SetGlobal("table", tab)
	}
	if s.stdlib.OS {
		osTable := value.NewTable()
		osTable.Set(value.String("time"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if args.Get(0) == value.Nil {
				return value.Number(time.Now().Unix()), nil
			}
			table, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("os.time expects table")
			}
			year := luaDateTableInt(table, "year", 0)
			month := luaDateTableInt(table, "month", 0)
			day := luaDateTableInt(table, "day", 0)
			if year == 0 || month == 0 || day == 0 {
				return value.Nil, fmt.Errorf("os.time table requires year, month, and day")
			}
			hour := luaDateTableInt(table, "hour", 12)
			minute := luaDateTableInt(table, "min", 0)
			second := luaDateTableInt(table, "sec", 0)
			t := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.Local)
			return value.Number(t.Unix()), nil
		}})
		osTable.Set(value.String("date"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			format := args.String(0)
			if format == "nil" {
				format = "%c"
			}
			t := time.Now()
			if args.Get(1) != value.Nil {
				t = time.Unix(int64(args.Number(1)), 0)
			}
			if strings.HasPrefix(format, "!") {
				t = t.UTC()
				format = strings.TrimPrefix(format, "!")
			}
			if format == "*t" {
				return luaDateTable(t), nil
			}
			return value.String(formatLuaDate(format, t)), nil
		}})
		osTable.Set(value.String("clock"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(float64(time.Now().UnixNano()) / float64(time.Second)), nil
		}})
		osTable.Set(value.String("difftime"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Number(args.Number(0) - args.Number(1)), nil
		}})
		osTable.Set(value.String("getenv"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if v, ok := os.LookupEnv(args.String(0)); ok {
				return value.String(v), nil
			}
			return value.Nil, nil
		}})
		osTable.Set(value.String("tmpname"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			f, err := os.CreateTemp("", "higolua-*")
			if err != nil {
				return value.Nil, err
			}
			name := f.Name()
			_ = f.Close()
			_ = os.Remove(name)
			return value.String(name), nil
		}})
		osTable.Set(value.String("rename"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if err := os.Rename(args.String(0), args.String(1)); err != nil {
				return value.Nil, err
			}
			return value.Bool(true), nil
		}})
		osTable.Set(value.String("remove"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if err := os.Remove(args.String(0)); err != nil {
				return value.Nil, err
			}
			return value.Bool(true), nil
		}})
		osTable.Set(value.String("execute"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			cmd := args.String(0)
			if cmd == "nil" {
				return value.Bool(true), nil
			}
			err := exec.CommandContext(ctx, "sh", "-c", cmd).Run()
			if err == nil {
				return value.Number(0), nil
			}
			if exitErr, ok := err.(*exec.ExitError); ok {
				return value.Number(exitErr.ExitCode()), nil
			}
			return value.Nil, err
		}})
		osTable.Set(value.String("setlocale"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			locale := args.String(0)
			if locale == "nil" || locale == "" {
				return value.String("C"), nil
			}
			if locale == "C" {
				return value.String("C"), nil
			}
			return value.Nil, nil
		}})
		osTable.Set(value.String("exit"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			code := int(args.Number(0))
			return value.Nil, &ExitError{Code: code}
		}})
		s.SetGlobal("os", osTable)
	}
	if s.stdlib.IO {
		ioTable := value.NewTable()
		ioTable.Set(value.String("stdin"), s.input)
		ioTable.Set(value.String("stdout"), s.output)
		ioTable.Set(value.String("stderr"), s.stderr)
		ioTable.Set(value.String("open"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			path := args.String(0)
			mode := args.String(1)
			if mode == "nil" || mode == "" {
				mode = "r"
			}
			flag := os.O_RDONLY
			switch mode {
			case "r", "rb":
				flag = os.O_RDONLY
			case "w", "wb":
				flag = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
			case "a", "ab":
				flag = os.O_CREATE | os.O_APPEND | os.O_WRONLY
			case "r+", "r+b", "rb+":
				flag = os.O_RDWR
			case "w+", "w+b", "wb+":
				flag = os.O_CREATE | os.O_TRUNC | os.O_RDWR
			case "a+", "a+b", "ab+":
				flag = os.O_CREATE | os.O_APPEND | os.O_RDWR
			default:
				return []value.Value{value.Nil, value.String("unsupported io.open mode")}, nil
			}
			file, err := os.OpenFile(path, flag, 0o666)
			if err != nil {
				return []value.Value{value.Nil, value.String(err.Error())}, nil
			}
			handle := &fileHandle{file: file}
			if strings.Contains(mode, "r") || strings.Contains(mode, "+") {
				handle.reader = bufio.NewReader(file)
			}
			return []value.Value{handle}, nil
		}})
		ioTable.Set(value.String("lines"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if args.Get(0) == value.Nil {
				return fileLineIterator(s.input, false), nil
			}
			path := args.String(0)
			file, err := os.OpenFile(path, os.O_RDONLY, 0)
			if err != nil {
				return value.Nil, err
			}
			return fileLineIterator(&fileHandle{file: file, reader: bufio.NewReader(file)}, true), nil
		}})
		ioTable.Set(value.String("read"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			return s.fileMethod(s.input, "read").(*multiGoFunction).fn(ctx, append(Args{value.Nil}, args...))
		}})
		ioTable.Set(value.String("write"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return s.fileMethod(s.output, "write").(*goFunction).fn(ctx, append(Args{s.output}, args...))
		}})
		ioTable.Set(value.String("flush"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return s.fileMethod(s.output, "flush").(*goFunction).fn(ctx, Args{s.output})
		}})
		ioTable.Set(value.String("close"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			target := args.Get(0)
			if target == value.Nil {
				target = s.output
			}
			handle, ok := target.(*fileHandle)
			if !ok {
				return value.Nil, fmt.Errorf("io.close expects file")
			}
			return s.fileMethod(handle, "close").(*goFunction).fn(ctx, Args{handle})
		}})
		ioTable.Set(value.String("input"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if args.Get(0) == value.Nil {
				return s.input, nil
			}
			previous := s.input
			switch target := args.Get(0).(type) {
			case *fileHandle:
				s.input = target
			case value.String:
				file, err := os.OpenFile(string(target), os.O_RDONLY, 0)
				if err != nil {
					return value.Nil, err
				}
				s.input = &fileHandle{file: file, reader: bufio.NewReader(file)}
			default:
				return value.Nil, fmt.Errorf("io.input expects file or path")
			}
			return previous, nil
		}})
		ioTable.Set(value.String("output"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			if args.Get(0) == value.Nil {
				return s.output, nil
			}
			previous := s.output
			switch target := args.Get(0).(type) {
			case *fileHandle:
				s.output = target
			case value.String:
				file, err := os.OpenFile(string(target), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666)
				if err != nil {
					return value.Nil, err
				}
				s.output = &fileHandle{file: file}
			default:
				return value.Nil, fmt.Errorf("io.output expects file or path")
			}
			return previous, nil
		}})
		ioTable.Set(value.String("popen"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			command := args.String(0)
			mode := args.String(1)
			if mode == "nil" || mode == "" {
				mode = "r"
			}
			cmd := exec.CommandContext(ctx, "sh", "-c", command)
			switch mode {
			case "r":
				pipe, err := cmd.StdoutPipe()
				if err != nil {
					return []value.Value{value.Nil, value.String(err.Error())}, nil
				}
				if err := cmd.Start(); err != nil {
					return []value.Value{value.Nil, value.String(err.Error())}, nil
				}
				file, ok := pipe.(*os.File)
				if !ok {
					_ = cmd.Process.Kill()
					return []value.Value{value.Nil, value.String("popen stdout is not a file")}, nil
				}
				return []value.Value{&fileHandle{file: file, reader: bufio.NewReader(file), closer: pipe, wait: cmd.Wait}}, nil
			case "w":
				pipe, err := cmd.StdinPipe()
				if err != nil {
					return []value.Value{value.Nil, value.String(err.Error())}, nil
				}
				if err := cmd.Start(); err != nil {
					return []value.Value{value.Nil, value.String(err.Error())}, nil
				}
				file, ok := pipe.(*os.File)
				if !ok {
					_ = cmd.Process.Kill()
					return []value.Value{value.Nil, value.String("popen stdin is not a file")}, nil
				}
				return []value.Value{&fileHandle{file: file, closer: pipe, wait: cmd.Wait}}, nil
			default:
				return []value.Value{value.Nil, value.String("unsupported io.popen mode")}, nil
			}
		}})
		ioTable.Set(value.String("tmpfile"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			file, err := os.CreateTemp("", "higolua-*")
			if err != nil {
				return []value.Value{value.Nil, value.String(err.Error())}, nil
			}
			if err := os.Remove(file.Name()); err != nil {
				_ = file.Close()
				return []value.Value{value.Nil, value.String(err.Error())}, nil
			}
			return []value.Value{&fileHandle{file: file, reader: bufio.NewReader(file)}}, nil
		}})
		ioTable.Set(value.String("type"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			handle, ok := args.Get(0).(*fileHandle)
			if !ok {
				return value.Nil, nil
			}
			if handle.file == nil {
				return value.String("closed file"), nil
			}
			return value.String("file"), nil
		}})
		s.SetGlobal("io", ioTable)
	}
	if s.stdlib.Package {
		pkg := value.NewTable()
		loaded := value.NewTable()
		preload := value.NewTable()
		loaders := value.NewTable()
		pkg.Set(value.String("path"), value.String("./?.lua;./?/init.lua"))
		pkg.Set(value.String("cpath"), value.String(""))
		pkg.Set(value.String("config"), value.String("/\n;\n?\n!\n-\n"))
		pkg.Set(value.String("loaded"), loaded)
		pkg.Set(value.String("preload"), preload)
		pkg.Set(value.String("loaders"), loaders)
		pkg.Set(value.String("searchers"), loaders)
		pkg.Set(value.String("loadlib"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return value.Nil, fmt.Errorf("package.loadlib is not supported by pure Go HiGoLua")
		}})
		pkg.Set(value.String("seeall"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			moduleTable, ok := args.Get(0).(*value.Table)
			if !ok {
				return value.Nil, fmt.Errorf("package.seeall expects table")
			}
			mt := moduleTable.Metatable()
			if mt == nil {
				mt = value.NewTable()
				moduleTable.SetMetatable(mt)
			}
			mt.RawSet(value.String("__index"), s.globalTable())
			return value.Nil, nil
		}})
		loaders.Append(&goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			name := args.String(0)
			if loader := preload.Get(value.String(name)); loader != value.Nil {
				return loader, nil
			}
			return value.String("\n\tno field package.preload['" + name + "']"), nil
		}})
		if s.stdlib.FileLoad {
			loaders.Append(&goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
				name := args.String(0)
				path := strings.ReplaceAll(name, ".", string(filepath.Separator))
				for _, pattern := range strings.Split(pkg.Get(value.String("path")).String(), ";") {
					candidate := strings.Replace(pattern, "?", path, 1)
					if _, err := os.Stat(candidate); err == nil {
						source, err := os.ReadFile(candidate)
						if err != nil {
							return value.String("\n\t" + err.Error()), nil
						}
						return &multiGoFunction{fn: func(ctx context.Context, callArgs Args) ([]value.Value, error) {
							return s.DoChunkValues(ctx, candidate, string(source))
						}}, nil
					}
				}
				return value.String("\n\tno file module '" + name + "' in package.path"), nil
			}})
		}
		s.SetGlobal("package", pkg)
		s.Register("module", func(ctx context.Context, args Args) (value.Value, error) {
			name := args.String(0)
			moduleTable, err := s.ensureModuleTable(name, loaded)
			if err != nil {
				return value.Nil, err
			}
			moduleTable.RawSet(value.String("_M"), moduleTable)
			moduleTable.RawSet(value.String("_NAME"), value.String(name))
			moduleTable.RawSet(value.String("_PACKAGE"), value.String(modulePackageName(name)))
			for _, option := range args[1:] {
				if _, err := s.callValue(ctx, option, []value.Value{moduleTable}); err != nil {
					return value.Nil, err
				}
			}
			frame := s.envAtLevel(1)
			if frame == nil {
				return value.Nil, fmt.Errorf("module called outside Lua frame")
			}
			frame.setRuntimeEnv(tableToEnv(moduleTable))
			return moduleTable, nil
		})
		s.Register("require", func(ctx context.Context, args Args) (value.Value, error) {
			name := args.String(0)
			moduleKey := value.String(name)
			if cached := loaded.RawGet(moduleKey); value.IsTruthy(cached) {
				return cached, nil
			}
			activeLoaders, ok := pkg.Get(value.String("loaders")).(*value.Table)
			if searchers, searchersOK := pkg.Get(value.String("searchers")).(*value.Table); searchersOK && activeLoaders == loaders {
				activeLoaders = searchers
			}
			if !ok {
				return value.Nil, fmt.Errorf("package.loaders must be table")
			}
			messages := strings.Builder{}
			for i := 1; i <= activeLoaders.Len(); i++ {
				loaderFn := activeLoaders.Get(value.Number(i))
				if loaderFn == value.Nil {
					continue
				}
				loader, err := s.callValue(ctx, loaderFn, []value.Value{moduleKey})
				if err != nil {
					return value.Nil, err
				}
				switch candidate := loader.(type) {
				case *luaFunc, *goFunction, *multiGoFunction:
					results, err := s.callValueMulti(ctx, candidate, []value.Value{moduleKey})
					if err != nil {
						return value.Nil, err
					}
					module := first(results)
					if module == value.Nil {
						module = loaded.RawGet(moduleKey)
						if module == value.Nil {
							module = value.Bool(true)
						}
					}
					loaded.RawSet(moduleKey, module)
					return module, nil
				case value.String:
					messages.WriteString(candidate.String())
				}
			}
			if messages.Len() > 0 {
				return value.Nil, fmt.Errorf("module %s not found:%s", name, messages.String())
			}
			return value.Nil, fmt.Errorf("module %s not found", name)
		})
	}
	if s.stdlib.Debug {
		dbg := value.NewTable()
		dbg.Set(value.String("traceback"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			var b strings.Builder
			msg := args.String(0)
			if msg != "nil" {
				b.WriteString(msg)
			}
			for i := len(s.stack) - 1; i >= 0; i-- {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(s.stack[i])
			}
			return value.String(b.String()), nil
		}})
		dbg.Set(value.String("getmetatable"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			mt := valueMetatable(args.Get(0))
			if mt == nil {
				return value.Nil, nil
			}
			return mt, nil
		}})
		dbg.Set(value.String("setmetatable"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			target := args.Get(0)
			var mt *value.Table
			if args.Get(1) != value.Nil {
				var ok bool
				mt, ok = args.Get(1).(*value.Table)
				if !ok {
					return value.Nil, fmt.Errorf("debug.setmetatable expects table or nil")
				}
			}
			switch t := target.(type) {
			case *value.Table:
				t.SetMetatable(mt)
			case *value.UserData:
				t.SetMetatable(mt)
			case *proxyUserData:
				t.metatable = mt
			default:
				return value.Nil, fmt.Errorf("debug.setmetatable expects table or userdata")
			}
			return target, nil
		}})
		dbg.Set(value.String("getregistry"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			return s.registry, nil
		}})
		dbg.Set(value.String("getfenv"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			fn, ok := s.global.get("getfenv").(*goFunction)
			if !ok {
				return value.Nil, fmt.Errorf("debug.getfenv requires base getfenv")
			}
			return fn.fn(ctx, args)
		}})
		dbg.Set(value.String("setfenv"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			fn, ok := s.global.get("setfenv").(*goFunction)
			if !ok {
				return value.Nil, fmt.Errorf("debug.setfenv requires base setfenv")
			}
			return fn.fn(ctx, args)
		}})
		dbg.Set(value.String("getupvalue"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			fn, ok := args.Get(0).(*luaFunc)
			if !ok {
				return []value.Value{value.Nil}, nil
			}
			name, ok := luaUpvalueName(fn, int(args.Number(1)))
			if !ok {
				return []value.Value{value.Nil}, nil
			}
			return []value.Value{value.String(name), fn.env.get(name)}, nil
		}})
		dbg.Set(value.String("setupvalue"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			fn, ok := args.Get(0).(*luaFunc)
			if !ok {
				return value.Nil, nil
			}
			name, ok := luaUpvalueName(fn, int(args.Number(1)))
			if !ok {
				return value.Nil, nil
			}
			fn.env.set(name, args.Get(2))
			return value.String(name), nil
		}})
		dbg.Set(value.String("getlocal"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			frame := s.debugFrameEnv(int(args.Number(0)))
			if frame == nil {
				return []value.Value{value.Nil}, nil
			}
			name, ok := envLocalName(frame, int(args.Number(1)))
			if !ok {
				return []value.Value{value.Nil}, nil
			}
			return []value.Value{value.String(name), frame.get(name)}, nil
		}})
		dbg.Set(value.String("setlocal"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			frame := s.debugFrameEnv(int(args.Number(0)))
			if frame == nil {
				return value.Nil, nil
			}
			name, ok := envLocalName(frame, int(args.Number(1)))
			if !ok {
				return value.Nil, nil
			}
			frame.set(name, args.Get(2))
			return value.String(name), nil
		}})
		dbg.Set(value.String("sethook"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			hook := args.Get(0)
			if hook == value.Nil {
				s.debugHook = value.Nil
				s.debugHookMask = ""
				s.debugHookCount = 0
				s.debugHookTick = 0
				return value.Nil, nil
			}
			switch hook.(type) {
			case *luaFunc, *goFunction, *multiGoFunction:
			default:
				return value.Nil, fmt.Errorf("debug.sethook expects function or nil")
			}
			s.debugHook = hook
			s.debugHookMask = args.String(1)
			s.debugHookCount = int(args.Number(2))
			s.debugHookTick = 0
			return value.Nil, nil
		}})
		dbg.Set(value.String("gethook"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			if s.debugHook == nil || s.debugHook == value.Nil {
				return []value.Value{value.Nil}, nil
			}
			return []value.Value{s.debugHook, value.String(s.debugHookMask), value.Number(s.debugHookCount)}, nil
		}})
		dbg.Set(value.String("getinfo"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			info := value.NewTable()
			target := args.Get(0)
			switch target := target.(type) {
			case value.Number:
				info.Set(value.String("what"), value.String("Lua"))
				info.Set(value.String("source"), value.String("stack"))
				info.Set(value.String("short_src"), value.String("stack"))
				info.Set(value.String("currentline"), value.Number(-1))
				info.Set(value.String("linedefined"), value.Number(-1))
				info.Set(value.String("lastlinedefined"), value.Number(-1))
				info.Set(value.String("namewhat"), value.String(""))
				info.Set(value.String("nups"), value.Number(0))
			case *luaFunc:
				info.Set(value.String("what"), value.String("Lua"))
				source := target.source
				if source == "" {
					source = "function"
				}
				lineDefined := target.lineDefined
				if lineDefined <= 0 {
					lineDefined = -1
				}
				info.Set(value.String("source"), value.String(source))
				info.Set(value.String("short_src"), value.String(source))
				info.Set(value.String("currentline"), value.Number(-1))
				info.Set(value.String("linedefined"), value.Number(lineDefined))
				info.Set(value.String("lastlinedefined"), value.Number(lineDefined))
				info.Set(value.String("namewhat"), value.String(""))
				info.Set(value.String("nups"), value.Number(len(luaUpvalueNames(target))))
			case *goFunction, *multiGoFunction:
				info.Set(value.String("what"), value.String("C"))
				info.Set(value.String("source"), value.String("Go"))
				info.Set(value.String("short_src"), value.String("Go"))
				info.Set(value.String("currentline"), value.Number(-1))
				info.Set(value.String("linedefined"), value.Number(-1))
				info.Set(value.String("lastlinedefined"), value.Number(-1))
				info.Set(value.String("namewhat"), value.String(""))
				info.Set(value.String("nups"), value.Number(0))
			default:
				return value.Nil, nil
			}
			return info, nil
		}})
		s.SetGlobal("debug", dbg)
	}
	if s.stdlib.Coroutine {
		co := value.NewTable()
		co.Set(value.String("create"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			fn := args.Get(0)
			if !isCallableValue(fn) {
				return value.Nil, fmt.Errorf("coroutine.create expects function")
			}
			return newCoroutineThread(fn), nil
		}})
		co.Set(value.String("resume"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			thread, ok := args.Get(0).(*coroutineThread)
			if !ok {
				return []value.Value{value.Bool(false), value.String("coroutine.resume expects thread")}, nil
			}
			return thread.resume(ctx, s, []value.Value(args[1:]))
		}})
		co.Set(value.String("yield"), &multiGoFunction{fn: func(ctx context.Context, args Args) ([]value.Value, error) {
			thread, ok := ctx.Value(coroutineContextKey{}).(*coroutineThread)
			if !ok {
				return nil, fmt.Errorf("attempt to yield outside coroutine")
			}
			return thread.yield(ctx, []value.Value(args))
		}})
		co.Set(value.String("status"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			thread, ok := args.Get(0).(*coroutineThread)
			if !ok {
				return value.Nil, fmt.Errorf("coroutine.status expects thread")
			}
			return value.String(thread.statusValue()), nil
		}})
		co.Set(value.String("running"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			thread, ok := ctx.Value(coroutineContextKey{}).(*coroutineThread)
			if !ok {
				return value.Nil, nil
			}
			return thread, nil
		}})
		co.Set(value.String("wrap"), &goFunction{fn: func(ctx context.Context, args Args) (value.Value, error) {
			fn := args.Get(0)
			if !isCallableValue(fn) {
				return value.Nil, fmt.Errorf("coroutine.wrap expects function")
			}
			thread := newCoroutineThread(fn)
			return &multiGoFunction{fn: func(ctx context.Context, callArgs Args) ([]value.Value, error) {
				values, err := thread.resume(ctx, s, []value.Value(callArgs))
				if err != nil {
					return nil, err
				}
				if len(values) > 0 && !value.IsTruthy(values[0]) {
					if len(values) > 1 {
						return nil, fmt.Errorf(values[1].String())
					}
					return nil, fmt.Errorf("coroutine failed")
				}
				if len(values) <= 1 {
					return []value.Value{value.Nil}, nil
				}
				return values[1:], nil
			}}, nil
		}})
		s.SetGlobal("coroutine", co)
	}
}

func first(values []value.Value) value.Value {
	if len(values) == 0 || values[0] == nil {
		return value.Nil
	}
	return values[0]
}

func valueAt(values []value.Value, i int) value.Value {
	if i < 0 || i >= len(values) || values[i] == nil {
		return value.Nil
	}
	return values[i]
}

func isCallableValue(v value.Value) bool {
	switch v.(type) {
	case *luaFunc, *goFunction, *multiGoFunction:
		return true
	default:
		return false
	}
}

func valueMetatable(v value.Value) *value.Table {
	switch x := v.(type) {
	case *value.Table:
		return x.Metatable()
	case *value.UserData:
		return x.Metatable()
	case *proxyUserData:
		return x.metatable
	default:
		return nil
	}
}

func luaFormatArgs(format string, args []value.Value) (string, []any) {
	var out strings.Builder
	converted := make([]any, 0, len(args))
	argIndex := 0
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) {
			out.WriteByte(format[i])
			continue
		}
		start := i
		i++
		if format[i] == '%' {
			out.WriteString("%%")
			continue
		}
		for i < len(format) && !isLuaFormatVerb(format[i]) {
			i++
		}
		if i >= len(format) {
			out.WriteString(format[start:])
			break
		}
		verb := format[i]
		out.WriteString(format[start:i])
		if verb == 'i' || verb == 'u' {
			out.WriteByte('d')
		} else {
			out.WriteByte(verb)
		}
		if argIndex < len(args) {
			converted = append(converted, luaFormatArg(args[argIndex], verb))
			argIndex++
		}
	}
	for ; argIndex < len(args); argIndex++ {
		converted = append(converted, luaFormatArg(args[argIndex], 0))
	}
	return out.String(), converted
}

func isLuaFormatVerb(ch byte) bool {
	return strings.ContainsRune("cdiouxXeEfgGqs", rune(ch))
}

func luaFormatArg(arg value.Value, verb byte) any {
	if n, ok := arg.(value.Number); ok {
		switch verb {
		case 'u':
			return uint32(int64(n))
		case 'c', 'd', 'i', 'o', 'x', 'X':
			return int64(n)
		default:
			if float64(n) == math.Trunc(float64(n)) {
				return int64(n)
			}
			return float64(n)
		}
	}
	return arg.String()
}

func (s *State) callDebugHook(ctx context.Context, event string, line int) error {
	if s.debugHook == nil || s.debugHook == value.Nil || s.debugHookActive {
		return nil
	}
	switch event {
	case "call":
		if !strings.Contains(s.debugHookMask, "c") {
			return nil
		}
	case "return":
		if !strings.Contains(s.debugHookMask, "r") {
			return nil
		}
	case "line":
		if !strings.Contains(s.debugHookMask, "l") {
			return nil
		}
	case "count":
	default:
		return nil
	}
	s.debugHookActive = true
	defer func() { s.debugHookActive = false }()
	_, err := s.callValueMulti(ctx, s.debugHook, []value.Value{value.String(event), value.Number(line)})
	return err
}

func luaStringStart(source string, start int) int {
	if start == 0 {
		start = 1
	}
	if start < 0 {
		start = len(source) + start + 1
	}
	if start < 1 {
		start = 1
	}
	if start > len(source)+1 {
		start = len(source) + 1
	}
	return start - 1
}

func parseBalancedPattern(pattern string) (byte, byte, bool) {
	if len(pattern) != 4 || pattern[0] != '%' || pattern[1] != 'b' {
		return 0, 0, false
	}
	return pattern[2], pattern[3], true
}

func firstBalancedMatch(source string, init int, open, close byte) []int {
	matches := balancedMatches(source, init, open, close)
	if len(matches) == 0 {
		return nil
	}
	return matches[0]
}

func balancedMatches(source string, init int, open, close byte) [][]int {
	var matches [][]int
	for i := init; i < len(source); i++ {
		if source[i] != open {
			continue
		}
		depth := 1
		for j := i + 1; j < len(source); j++ {
			switch source[j] {
			case open:
				depth++
			case close:
				depth--
				if depth == 0 {
					matches = append(matches, []int{i, j + 1})
					i = j
					goto nextSearch
				}
			}
		}
	nextSearch:
	}
	return matches
}

func luaFrontierMatches(source string, init int, pattern string) ([][]int, bool, error) {
	if !strings.Contains(pattern, "%f[") {
		return nil, false, nil
	}
	var matches [][]int
	for start := init; start <= len(source); start++ {
		end, ok, err := matchFrontierAt(source, start, pattern)
		if err != nil {
			return nil, true, err
		}
		if ok {
			matches = append(matches, []int{start, end})
			if end > start {
				start = end - 1
			}
		}
	}
	return matches, true, nil
}

func matchFrontierAt(source string, start int, pattern string) (int, bool, error) {
	pos := start
	for i := 0; i < len(pattern); i++ {
		if strings.HasPrefix(pattern[i:], "%f[") {
			close := strings.IndexByte(pattern[i+3:], ']')
			if close < 0 {
				return 0, false, fmt.Errorf("malformed frontier pattern")
			}
			set := pattern[i+3 : i+3+close]
			if frontierContains(set, byteAt(source, pos-1)) || !frontierContains(set, byteAt(source, pos)) {
				return 0, false, nil
			}
			i += close + 3
			continue
		}
		matcher, next, err := frontierPatternAtom(pattern, i)
		if err != nil {
			return 0, false, err
		}
		repeat := byte(0)
		if next < len(pattern) && strings.ContainsRune("+*?-", rune(pattern[next])) {
			repeat = pattern[next]
			next++
		}
		count := 0
		for pos < len(source) && matcher(source[pos]) {
			pos++
			count++
			if repeat == 0 || repeat == '?' {
				break
			}
		}
		if repeat == 0 && count != 1 {
			return 0, false, nil
		}
		if repeat == '+' && count == 0 {
			return 0, false, nil
		}
		i = next - 1
	}
	return pos, true, nil
}

func frontierPatternAtom(pattern string, i int) (func(byte) bool, int, error) {
	if i >= len(pattern) {
		return nil, i, fmt.Errorf("empty pattern atom")
	}
	if pattern[i] != '%' {
		ch := pattern[i]
		return func(b byte) bool { return b == ch }, i + 1, nil
	}
	if i+1 >= len(pattern) {
		return nil, i, fmt.Errorf("malformed pattern escape")
	}
	ch := pattern[i+1]
	return func(b byte) bool { return luaPatternClassContains(ch, b) }, i + 2, nil
}

func byteAt(source string, pos int) byte {
	if pos < 0 || pos >= len(source) {
		return 0
	}
	return source[pos]
}

func frontierContains(set string, b byte) bool {
	for i := 0; i < len(set); i++ {
		if set[i] == '%' && i+1 < len(set) {
			i++
			if luaPatternClassContains(set[i], b) {
				return true
			}
			continue
		}
		if i+2 < len(set) && set[i+1] == '-' {
			if b >= set[i] && b <= set[i+2] {
				return true
			}
			i += 2
			continue
		}
		if b == set[i] {
			return true
		}
	}
	return false
}

func luaPatternClassContains(ch byte, b byte) bool {
	switch ch {
	case 'a':
		return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
	case 'A':
		return !luaPatternClassContains('a', b)
	case 'd':
		return b >= '0' && b <= '9'
	case 'D':
		return !luaPatternClassContains('d', b)
	case 'l':
		return b >= 'a' && b <= 'z'
	case 'L':
		return !luaPatternClassContains('l', b)
	case 'u':
		return b >= 'A' && b <= 'Z'
	case 'U':
		return !luaPatternClassContains('u', b)
	case 'w':
		return luaPatternClassContains('a', b) || luaPatternClassContains('d', b) || b == '_'
	case 'W':
		return !luaPatternClassContains('w', b)
	case 's':
		return b == ' ' || b == '\f' || b == '\n' || b == '\r' || b == '\t' || b == '\v'
	case 'S':
		return !luaPatternClassContains('s', b)
	case 'x':
		return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
	case 'X':
		return !luaPatternClassContains('x', b)
	case 'z':
		return b == 0
	default:
		return b == ch
	}
}

func compileLuaPattern(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	for i := 0; i < len(pattern); i++ {
		ch := pattern[i]
		switch ch {
		case '%':
			i++
			if i >= len(pattern) {
				b.WriteString(regexp.QuoteMeta("%"))
				break
			}
			b.WriteString(luaPatternEscape(pattern[i]))
		case '.':
			b.WriteByte('.')
		case '+', '*', '?':
			b.WriteByte(ch)
		case '-':
			b.WriteByte('*')
		case '^', '$':
			b.WriteByte(ch)
		case '(', ')':
			b.WriteByte(ch)
		case '[':
			end := strings.IndexByte(pattern[i+1:], ']')
			if end < 0 {
				b.WriteString(regexp.QuoteMeta("["))
				break
			}
			content := pattern[i+1 : i+1+end]
			b.WriteByte('[')
			b.WriteString(strings.ReplaceAll(content, "%", "\\"))
			b.WriteByte(']')
			i += end + 1
		default:
			b.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	return regexp.Compile(b.String())
}

func luaPatternEscape(ch byte) string {
	switch ch {
	case 'a':
		return `[A-Za-z]`
	case 'A':
		return `[^A-Za-z]`
	case 'd':
		return `\d`
	case 'D':
		return `\D`
	case 'l':
		return `[a-z]`
	case 'L':
		return `[^a-z]`
	case 'u':
		return `[A-Z]`
	case 'U':
		return `[^A-Z]`
	case 'w':
		return `[A-Za-z0-9_]`
	case 'W':
		return `[^A-Za-z0-9_]`
	case 's':
		return `\s`
	case 'S':
		return `\S`
	case 'x':
		return `[A-Fa-f0-9]`
	case 'X':
		return `[^A-Fa-f0-9]`
	case 'p':
		return `[\pP]`
	case 'P':
		return `[^\pP]`
	case 'c':
		return `[\x00-\x1F\x7F]`
	case 'C':
		return `[^\x00-\x1F\x7F]`
	case 'z':
		return `\x00`
	default:
		return regexp.QuoteMeta(string(ch))
	}
}

func luaReplacement(source string, indexes []int, replacement string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(replacement); i++ {
		if replacement[i] != '%' || i+1 >= len(replacement) {
			b.WriteByte(replacement[i])
			continue
		}
		i++
		if replacement[i] == '%' {
			b.WriteByte('%')
			continue
		}
		if replacement[i] >= '0' && replacement[i] <= '9' {
			group := int(replacement[i] - '0')
			idx := group * 2
			if idx+1 < len(indexes) && indexes[idx] >= 0 {
				b.WriteString(source[indexes[idx]:indexes[idx+1]])
			} else {
				return "", fmt.Errorf("invalid capture index")
			}
			continue
		}
		b.WriteByte(replacement[i])
	}
	return b.String(), nil
}

func luaUpvalueName(fn *luaFunc, index int) (string, bool) {
	names := luaUpvalueNames(fn)
	if index < 1 || index > len(names) {
		return "", false
	}
	return names[index-1], true
}

func luaUpvalueNames(fn *luaFunc) []string {
	if fn == nil {
		return nil
	}
	return append([]string(nil), fn.upvalueNames...)
}

func (s *State) debugFrameEnv(level int) *env {
	if level < 1 || level > len(s.envStack) {
		return nil
	}
	return s.envStack[len(s.envStack)-level]
}

func (s *State) currentChunkName() string {
	if len(s.chunkStack) == 0 {
		return "string"
	}
	return s.chunkStack[len(s.chunkStack)-1]
}

func envLocalName(e *env, index int) (string, bool) {
	names := envLocalNames(e)
	if index < 1 || index > len(names) {
		return "", false
	}
	return names[index-1], true
}

func envLocalNames(e *env) []string {
	if e == nil {
		return nil
	}
	names := make([]string, 0, len(e.values))
	for name := range e.values {
		if name == "_G" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type env struct {
	parent  *env
	values  map[string]value.Value
	varargs []value.Value
	table   *value.Table
}

func newEnv(parent *env) *env { return &env{parent: parent, values: map[string]value.Value{}} }

func (e *env) define(name string, v value.Value) { e.values[name] = v }

func (e *env) get(name string) value.Value {
	var root *env
	for cur := e; cur != nil; cur = cur.parent {
		root = cur
		if v, ok := cur.values[name]; ok {
			return v
		}
		if cur.table != nil {
			if v := cur.table.Get(value.String(name)); v != value.Nil {
				return v
			}
		}
	}
	if root != nil {
		if globals, ok := root.values["_G"].(*value.Table); ok {
			if v := globals.RawGet(value.String(name)); v != value.Nil {
				return v
			}
		}
	}
	return value.Nil
}

func (e *env) set(name string, v value.Value) {
	for cur := e; cur != nil; cur = cur.parent {
		if _, ok := cur.values[name]; ok {
			cur.values[name] = v
			if cur.parent == nil {
				cur.syncGlobalTable(name, v)
			}
			return
		}
	}
	root := e
	for root.parent != nil {
		root = root.parent
	}
	root.values[name] = v
	root.syncGlobalTable(name, v)
}

func (e *env) syncGlobalTable(name string, v value.Value) {
	if globals, ok := e.values["_G"].(*value.Table); ok {
		globals.RawSet(value.String(name), v)
	}
	if e.table != nil {
		e.table.RawSet(value.String(name), v)
	}
}

func (e *env) runtimeEnv() *env {
	if e == nil {
		return nil
	}
	if e.parent != nil {
		return e.parent
	}
	return e
}

func (e *env) setRuntimeEnv(runtime *env) {
	if e.parent != nil {
		e.parent = runtime
		return
	}
	if runtime == nil {
		return
	}
	e.values = runtime.values
	e.varargs = runtime.varargs
	e.table = runtime.table
}

func (e *env) varargValues() []value.Value {
	for cur := e; cur != nil; cur = cur.parent {
		if cur.varargs != nil {
			return append([]value.Value(nil), cur.varargs...)
		}
	}
	return nil
}

func _strconvUse() { _ = strconv.IntSize }
