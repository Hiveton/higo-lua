package higolua_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hiveton/higolua"
	"github.com/hiveton/higolua/state"
	"github.com/hiveton/higolua/stdlib"
	"github.com/hiveton/higolua/value"
)

func TestRuntimeDoStringReturnsExpression(t *testing.T) {
	rt := higolua.NewRuntime()

	result, err := rt.DoString(context.Background(), `return 1 + 2 * 3`)
	if err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	if result.String() != "7" {
		t.Fatalf("result = %s, want 7", result.String())
	}
}

func TestLua51PowerOperatorIsRightAssociative(t *testing.T) {
	result, err := higolua.NewRuntime().DoString(context.Background(), `return 2 ^ 3 ^ 2`)
	if err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	if result.String() != "512" {
		t.Fatalf("result = %s, want 512", result.String())
	}
}

func TestRuntimeDoFileExecutesScript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.lua")
	if err := os.WriteFile(path, []byte(`return "hello " .. "higolua"`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := higolua.NewRuntime().DoFile(context.Background(), path)
	if err != nil {
		t.Fatalf("DoFile() error = %v", err)
	}
	if result.String() != "hello higolua" {
		t.Fatalf("result = %q, want hello higolua", result.String())
	}
}

func TestRuntimeDoFileRequiresSiblingModule(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.lua")
	helperPath := filepath.Join(dir, "helper.lua")
	if err := os.WriteFile(helperPath, []byte(`return {suffix = "module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte(`
local helper = require("helper")
return "sibling:" .. helper.suffix
`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := higolua.NewRuntime().DoFile(context.Background(), mainPath)
	if err != nil {
		t.Fatalf("DoFile() error = %v", err)
	}
	if result.String() != "sibling:module" {
		t.Fatalf("result = %q, want sibling:module", result.String())
	}
}

func TestStateDoFileRequiresSiblingModule(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.lua")
	helperPath := filepath.Join(dir, "helper.lua")
	if err := os.WriteFile(helperPath, []byte(`return {suffix = "state"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mainPath, []byte(`
local helper = require("helper")
return "sibling:" .. helper.suffix
`), 0o600); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	defer st.Close()
	result, err := st.DoFile(context.Background(), mainPath)
	if err != nil {
		t.Fatalf("DoFile() error = %v", err)
	}
	if result.String() != "sibling:state" {
		t.Fatalf("result = %q, want sibling:state", result.String())
	}
}

func TestRuntimeDoReaderExecutesScript(t *testing.T) {
	result, err := higolua.NewRuntime().DoReader(context.Background(), "reader.lua", strings.NewReader(`return "reader"`))
	if err != nil {
		t.Fatalf("DoReader() error = %v", err)
	}
	if result.String() != "reader" {
		t.Fatalf("result = %q, want reader", result.String())
	}
}

func TestRuntimeWithSafeStdlibDisablesUnsafeLibraries(t *testing.T) {
	rt := higolua.NewRuntime(state.WithStdlib(stdlib.Safe()))
	result, err := rt.DoString(context.Background(), `return type(io) .. ":" .. type(os) .. ":" .. type(debug) .. ":" .. tostring(1 + 2)`)
	if err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	if result.String() != "nil:nil:nil:3" {
		t.Fatalf("result = %q, want safe stdlib without io/os/debug", result.String())
	}
}

func TestSafeStdlibDisablesFilesystemLoading(t *testing.T) {
	dir := t.TempDir()
	modulePath := filepath.Join(dir, "safe_mod.lua")
	if err := os.WriteFile(modulePath, []byte(`return {value = "from-file"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	st := state.New(state.WithStdlib(stdlib.Safe()))
	defer st.Close()
	if err := st.DoString(context.Background(), fmt.Sprintf(`
package.path = %q
package.preload.memory_mod = function()
  return {value = "from-preload"}
end
local loadfile_ok = pcall(function() return loadfile(%q) end)
local dofile_ok = pcall(function() return dofile(%q) end)
local require_file_ok = pcall(function() return require("safe_mod") end)
local preload = require("memory_mod")
result = tostring(loadfile_ok) .. ":" .. tostring(dofile_ok) .. ":" .. tostring(require_file_ok) .. ":" .. preload.value
`, filepath.Join(dir, "?.lua"), modulePath, modulePath)); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "false:false:false:from-preload" {
		t.Fatalf("result = %q, want safe stdlib to block filesystem loaders and keep preload", got.String())
	}
}

func TestStateRegistersGoFunctionAndReadsGlobal(t *testing.T) {
	st := state.New()
	defer st.Close()
	st.Register("add", func(ctx context.Context, args state.Args) (value.Value, error) {
		return value.Number(args.Number(0) + args.Number(1)), nil
	})

	if err := st.DoString(context.Background(), `result = add(20, 22)`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, err := st.GetGlobal("result")
	if err != nil {
		t.Fatalf("GetGlobal() error = %v", err)
	}
	if got.String() != "42" {
		t.Fatalf("result = %s, want 42", got.String())
	}
}

func TestGoModuleCanBeRequiredFromLua(t *testing.T) {
	st := state.New()
	defer st.Close()

	err := st.RegisterModule("host", map[string]state.GoFunc{
		"upper": func(ctx context.Context, args state.Args) (value.Value, error) {
			return value.String(strings.ToUpper(args.String(0))), nil
		},
	})
	if err != nil {
		t.Fatalf("RegisterModule() error = %v", err)
	}

	if err := st.DoString(context.Background(), `
local host = require("host")
result = host.upper("lua")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "LUA" {
		t.Fatalf("result = %q, want LUA", got.String())
	}
}

func TestStateCallConvertsPanicToRuntimeError(t *testing.T) {
	st := state.New()
	defer st.Close()
	st.Register("boom", func(ctx context.Context, args state.Args) (value.Value, error) {
		panic("panic from host")
	})

	_, err := st.Call(context.Background(), "boom")
	if err == nil {
		t.Fatal("Call() error = nil, want panic converted to error")
	}
	var runtimeErr *state.RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("Call() error = %T %v, want *state.RuntimeError", err, err)
	}
	if !strings.Contains(err.Error(), "panic from host") {
		t.Fatalf("Call() error = %v, want panic message", err)
	}
}

func TestLuaPCallCapturesGoFunctionPanic(t *testing.T) {
	st := state.New()
	defer st.Close()
	st.Register("boom", func(ctx context.Context, args state.Args) (value.Value, error) {
		panic("panic from pcall")
	})

	if err := st.DoString(context.Background(), `
local ok, err = pcall(boom)
result = tostring(ok) .. ":" .. type(err) .. ":" .. tostring(string.find(err, "panic from pcall", 1, true) ~= nil)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "false:string:true" {
		t.Fatalf("result = %q, want pcall to capture Go panic", got.String())
	}
}

func TestStateBytecodePathCallsRegisteredGoFunctionDirectly(t *testing.T) {
	st := state.New()
	defer st.Close()
	var calls int
	st.Register("add", func(ctx context.Context, args state.Args) (value.Value, error) {
		calls++
		return value.Number(args.Number(0) + args.Number(1)), nil
	})

	if err := st.DoString(context.Background(), `result = add(20, 22)`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, err := st.GetGlobal("result")
	if err != nil {
		t.Fatalf("GetGlobal() error = %v", err)
	}
	if got.String() != "42" || calls != 1 {
		t.Fatalf("result = %s calls = %d, want bytecode Go bridge result 42 with one call", got.String(), calls)
	}
}

func TestStateBytecodePathCallsRegisteredGoFunctionViaLocalAlias(t *testing.T) {
	st := state.New()
	defer st.Close()
	st.Register("add", func(ctx context.Context, args state.Args) (value.Value, error) {
		return value.Number(args.Number(0) + args.Number(1)), nil
	})

	if err := st.DoString(context.Background(), `
local f = add
result = f(20, 22)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, err := st.GetGlobal("result")
	if err != nil {
		t.Fatalf("GetGlobal() error = %v", err)
	}
	if got.String() != "42" {
		t.Fatalf("result = %s, want 42", got.String())
	}
}

func TestLua51FunctionCallSugarWithoutParentheses(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local seen
function capture(value)
  seen = value
end
capture "hello"
local t
function captureTable(value)
  t = value
end
captureTable {name = "lua"}
result = seen .. ":" .. t.name
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "hello:lua" {
		t.Fatalf("result = %q, want call sugar output", got.String())
	}
}

func TestLua51DottedFunctionDeclarationsUseFullPath(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
mod = {sub = {name = "module"}}
function mod.sub.answer()
  return 42
end
function mod.sub:greet(suffix)
  return self.name .. suffix
end
result = mod.sub.answer() .. ":" .. mod.sub:greet(" ok")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "42:module ok" {
		t.Fatalf("result = %q, want dotted function declarations to use full path", got.String())
	}
}

func TestLua51SemicolonEmptyStatements(t *testing.T) {
	result, err := higolua.NewRuntime().DoString(context.Background(), `
;;;
local a = 1;;
local b = 2;
; result = a + b ;;
return result;
`)
	if err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	if result.String() != "3" {
		t.Fatalf("result = %q, want 3", result.String())
	}
}

func TestLua51DoEndBlockScopesLocals(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local x = "outer"
do
  local x = "inner"
  result = x
end
result = result .. ":" .. x
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "inner:outer" {
		t.Fatalf("result = %q, want do-end lexical scope", got.String())
	}
}

func TestLua51IfElseBranchesScopeLocals(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local marker = "outer"
if false then
  local marker = "then"
else
  local marker = "else"
  result = marker
end
result = result .. ":" .. marker
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "else:outer" {
		t.Fatalf("result = %q, want if/else branch lexical scope", got.String())
	}
}

func TestLua51RepeatUntilConditionSeesLoopLocals(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local total = 0
repeat
  local done = total >= 2
  total = total + 1
until done
result = total .. ":" .. tostring(done)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "3:nil" {
		t.Fatalf("result = %q, want repeat condition to see loop-local done without leaking it", got.String())
	}
}

func TestStateCallsLuaFunctionFromGo(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
function greet(name)
  return "hello " .. name
end
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, err := st.Call(context.Background(), "greet", value.String("Go"))
	if err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if got.String() != "hello Go" {
		t.Fatalf("Call() = %q, want hello Go", got.String())
	}
}

func TestStateCallValuesReturnsAllLuaResults(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
function split()
  return "left", "right"
end
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, err := st.CallValues(context.Background(), "split")
	if err != nil {
		t.Fatalf("CallValues() error = %v", err)
	}
	if len(got) != 2 || got[0].String() != "left" || got[1].String() != "right" {
		t.Fatalf("CallValues() = %#v, want left/right", got)
	}
}

func TestStateRegisterMultiReturnsAllValuesToLua(t *testing.T) {
	st := state.New()
	defer st.Close()
	st.RegisterMulti("splitGo", func(ctx context.Context, args state.Args) ([]value.Value, error) {
		return []value.Value{value.String("go"), value.String("lua")}, nil
	})

	if err := st.DoString(context.Background(), `
local a, b = splitGo()
result = a .. ":" .. b
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "go:lua" {
		t.Fatalf("result = %q, want go:lua", got.String())
	}
}

func TestStateRunsControlFlowAndTables(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local sum = 0
local t = {1, 2, 3}
for i = 1, 3 do
  sum = sum + t[i]
end
if sum == 6 then
  result = "ok"
else
  result = "bad"
end
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, err := st.GetGlobal("result")
	if err != nil {
		t.Fatalf("GetGlobal() error = %v", err)
	}
	if got.String() != "ok" {
		t.Fatalf("result = %q, want ok", got.String())
	}
}

func TestStateRunsGenericForWithIPairs(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local total = 0
for i, v in ipairs({2, 4, 6}) do
  total = total + i + v
end
result = total
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "18" {
		t.Fatalf("result = %q, want 18", got.String())
	}
}

func TestGenericForUsesIteratorProtocol(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local function iter(state, control)
  local next = control + 1
  if next > state.limit then
    return nil
  end
  return next, state.prefix .. next
end
local out = ""
for i, v in iter, {limit = 3, prefix = "v"}, 0 do
  out = out .. i .. v
end
result = out
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "1v12v23v3" {
		t.Fatalf("result = %q, want iterator protocol output", got.String())
	}
}

func TestPairsAndIPairsReturnIterators(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local sum = 0
for i, v in ipairs({2, 4, 6}) do
  sum = sum + i + v
end
local key, val
for k, v in pairs({name = "lua"}) do
  key = k
  val = v
end
result = sum .. ":" .. key .. ":" .. val
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "18:name:lua" {
		t.Fatalf("result = %q, want pairs/ipairs iterator output", got.String())
	}
}

func TestStateBytecodePathRunsGenericForWithIPairs(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local sum = 0
for i, v in ipairs({2, 4, 6}) do
  sum = sum + i * v
end
result = sum
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "28" {
		t.Fatalf("result = %q, want 28", got.String())
	}
}

func TestLuaFunctionMultipleReturnValuesPropagateThroughAssignments(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
function triple()
  return 1, 2, 3
end
local a, b, c = triple()
result = a + b + c
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "6" {
		t.Fatalf("result = %q, want 6", got.String())
	}
}

func TestMetatableIndexAndRawAccess(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local fallback = {missing = "from-meta"}
local t = {present = "direct"}
setmetatable(t, {__index = fallback})
rawset(t, "raw", 99)
result = t.present .. ":" .. t.missing .. ":" .. rawget(t, "missing") .. ":" .. t.raw
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "direct:from-meta:nil:99" {
		t.Fatalf("result = %q, want direct:from-meta:nil:99", got.String())
	}
}

func TestMetatableNewIndexAndNilDeletesTableField(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local shadow = {}
local t = {existing = "kept", remove_me = "gone"}
setmetatable(t, {__newindex = shadow})
t.missing = "proxied"
t.existing = "changed"
t.remove_me = nil
result = t.existing .. ":" .. shadow.missing .. ":" .. tostring(rawget(t, "missing")) .. ":" .. tostring(rawget(t, "remove_me"))
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "changed:proxied:nil:nil" {
		t.Fatalf("result = %q, want changed:proxied:nil:nil", got.String())
	}
}

func TestMetatableNewIndexFunctionReceivesWrites(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local sink = {}
local t = setmetatable({}, {
  __newindex = function(self, key, value)
    sink[key] = value .. ":seen"
  end
})
t.name = "lua"
result = sink.name .. ":" .. tostring(rawget(t, "name"))
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "lua:seen:nil" {
		t.Fatalf("result = %q, want lua:seen:nil", got.String())
	}
}

func TestMetatableBinaryArithmeticAndConcat(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local mt = {
  __add = function(a, b)
    return {value = a.value + b.value}
  end,
  __concat = function(a, b)
    return a.label .. ":" .. b
  end
}
local a = setmetatable({value = 10, label = "left"}, mt)
local b = setmetatable({value = 5, label = "right"}, mt)
local c = a + b
result = c.value .. ":" .. (a .. "suffix")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "15:left:suffix" {
		t.Fatalf("result = %q, want 15:left:suffix", got.String())
	}
}

func TestMetatableComparisonMethods(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local mt = {
  __lt = function(a, b) return true end,
  __le = function(a, b) return false end
}
local a = setmetatable({value = 3}, mt)
if a < a and not (a <= a) then
  result = "ok"
else
  result = "bad"
end
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "ok" {
		t.Fatalf("result = %q, want ok", got.String())
	}
}

func TestMetatableEqAndToString(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local mt = {
  __eq = function(a, b) return a.id == b.id end,
  __tostring = function(v) return "object:" .. v.id end
}
local a = setmetatable({id = "same"}, mt)
local b = setmetatable({id = "same"}, mt)
local c = setmetatable({id = "other"}, mt)
if a == b and a ~= c then
  result = tostring(a)
else
  result = "bad"
end
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "object:same" {
		t.Fatalf("result = %q, want object:same", got.String())
	}
}

func TestLua51PrintUsesToStringMetamethod(t *testing.T) {
	original := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writePipe
	t.Cleanup(func() {
		os.Stdout = original
		readPipe.Close()
		writePipe.Close()
	})

	st := state.New()
	defer st.Close()
	runErr := st.DoString(context.Background(), `
local value = setmetatable({name = "HiGo"}, {
  __tostring = function(v) return "object:" .. v.name end
})
print(value)
`)
	writePipe.Close()
	os.Stdout = original
	output, readErr := io.ReadAll(readPipe)
	if runErr != nil {
		t.Fatalf("DoString() error = %v", runErr)
	}
	if readErr != nil {
		t.Fatalf("ReadAll() error = %v", readErr)
	}
	if strings.TrimSpace(string(output)) != "object:HiGo" {
		t.Fatalf("print output = %q, want __tostring output", string(output))
	}
}

func TestMetatableLenMethod(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = setmetatable({}, {
  __len = function(self)
    return 42
  end
})
result = #t
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "42" {
		t.Fatalf("result = %q, want __len result", got.String())
	}
}

func TestMetatableUnaryMinusMethod(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = setmetatable({value = 7}, {
  __unm = function(self)
    return self.value * 10
  end
})
result = -t
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "70" {
		t.Fatalf("result = %q, want __unm result", got.String())
	}
}

func TestLua51MetatableProtectionAndDebugBypass(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local mt = {__metatable = "locked", tag = "real"}
local t = setmetatable({}, mt)
local public = getmetatable(t)
local ok, err = pcall(function()
  setmetatable(t, {})
end)
debug.setmetatable(t, {tag = "debug"})
local raw = debug.getmetatable(t)
result = public .. ":" .. tostring(ok) .. ":" .. err .. ":" .. raw.tag
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "locked:false:cannot change a protected metatable:debug" {
		t.Fatalf("result = %q, want protected metatable behavior", got.String())
	}
}

func TestLua51NewproxyCreatesUserdataWithMetatable(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local u = newproxy(true)
local mt = getmetatable(u)
mt.__tostring = function() return "proxy" end
local v = newproxy(u)
result = type(u) .. ":" .. tostring(u) .. ":" .. tostring(getmetatable(v) == mt)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "userdata:proxy:true" {
		t.Fatalf("result = %q, want newproxy userdata behavior", got.String())
	}
}

func TestPublicUserDataValueSupportsLuaMetatable(t *testing.T) {
	st := state.New()
	defer st.Close()
	st.SetGlobal("ud", value.NewUserData("host-object"))

	if err := st.DoString(context.Background(), `
debug.setmetatable(ud, {
  __tostring = function(v) return "userdata-ok" end
})
result = type(ud) .. ":" .. tostring(ud)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "userdata:userdata-ok" {
		t.Fatalf("result = %q, want public userdata with metatable", got.String())
	}
}

func TestMetatableCallAllowsTableAsFunction(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local callable = setmetatable({prefix = "hi"}, {
  __call = function(self, name)
    return self.prefix .. ":" .. name
  end
})
result = callable("lua")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "hi:lua" {
		t.Fatalf("result = %q, want hi:lua", got.String())
	}
}

func TestPCallAssertAndError(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local ok1 = pcall(function() assert(true, "no") end)
local ok2 = pcall(function() error("boom") end)
if ok1 and not ok2 then
  result = "ok"
else
  result = "bad"
end
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "ok" {
		t.Fatalf("result = %q, want ok", got.String())
	}
}

func TestLua51AssertReturnsAllArgumentsOnSuccess(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local a, b, c = assert("ok", "left", "right")
result = a .. ":" .. b .. ":" .. c
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "ok:left:right" {
		t.Fatalf("result = %q, want assert to return all arguments", got.String())
	}
}

func TestPCallReturnsStatusAndFunctionResults(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local ok, a, b = pcall(function()
  return "x", "y"
end)
local bad, msg = pcall(function()
  error("boom")
end)
result = tostring(ok) .. ":" .. a .. b .. ":" .. tostring(bad) .. ":" .. msg
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "true:xy:false:boom" {
		t.Fatalf("result = %q, want true:xy:false:boom", got.String())
	}
}

func TestXpcallLoadAndGcinfoBaseFunctions(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local ok, handled = xpcall(function()
  error("boom")
end, function(err)
  return "handled:" .. err
end)
local f = assert(load("return 'loaded'"))
result = tostring(ok) .. ":" .. handled .. ":" .. f() .. ":" .. type(gcinfo())
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "false:handled:boom:loaded:number" {
		t.Fatalf("result = %q, want xpcall/load/gcinfo behavior", got.String())
	}
}

func TestProtectedCallsHandleNonFunctions(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local pOk, pErr = pcall(123)
local xOk, xHandled = xpcall("bad", function(err)
  return "handled:" .. err
end)
result = tostring(pOk) .. ":" .. type(pErr) .. ":" .. tostring(xOk) .. ":" .. xHandled
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "false:string:false:handled:attempt to call string" {
		t.Fatalf("result = %q, want pcall/xpcall non-function errors", got.String())
	}
}

func TestLua51XpcallDoesNotPassExtraArguments(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local ok, got = xpcall(function(arg)
  return tostring(arg)
end, function(err)
  return "handled:" .. err
end, "extra")
result = tostring(ok) .. ":" .. got
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "true:nil" {
		t.Fatalf("result = %q, want Lua 5.1 xpcall to ignore extra arguments", got.String())
	}
}

func TestLua51LoadAcceptsReaderFunction(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local chunks = {"return ", "'read", "er'"}
local index = 0
local f = assert(load(function()
  index = index + 1
  return chunks[index]
end, "reader-chunk"))
result = f()
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "reader" {
		t.Fatalf("result = %q, want load reader function output", got.String())
	}
}

func TestLua51LoadSyntaxErrorReturnsNilAndMessage(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local f, err = loadstring("return (")
local used = false
local g, err2 = load(function()
  if used then
    return nil
  end
  used = true
  return "return ("
end)
result = tostring(f) .. ":" .. type(err) .. ":" .. tostring(g) .. ":" .. type(err2)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "nil:string:nil:string" {
		t.Fatalf("result = %q, want load syntax errors as nil,message", got.String())
	}
}

func TestLua51TonumberBaseArgument(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
result = tonumber("ff", 16) .. ":" .. tonumber("101", 2) .. ":" .. tostring(tonumber("z", 10))
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "255:5:nil" {
		t.Fatalf("result = %q, want tonumber base conversion", got.String())
	}
}

func TestVarargExpressionAndSelect(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
function tail(...)
  return select(2, ...)
end
function count(...)
  return select("#", ...)
end
local a, b = tail("drop", "A", "B")
result = a .. b .. ":" .. count("x", "y", "z")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "AB:3" {
		t.Fatalf("result = %q, want AB:3", got.String())
	}
}

func TestLua51SelectOutOfRangeReturnsNoValues(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local function count(...)
  return select("#", ...)
end
result = count(select(4, "a", "b")) .. ":" .. tostring((select(4, "a", "b")))
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "0:nil" {
		t.Fatalf("result = %q, want select out of range to return zero values", got.String())
	}
}

func TestLua51VarargArgTableIncludesN(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local function collect(...)
  return arg.n .. ":" .. arg[1] .. ":" .. arg[2]
end
result = collect("left", "right")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "2:left:right" {
		t.Fatalf("result = %q, want Lua 5.1 arg.n", got.String())
	}
}

func TestTableConstructorExpandsLastMultipleReturn(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
function triple()
  return 1, 2, 3
end
local t = {0, triple()}
result = #t .. ":" .. t[1] .. t[2] .. t[3] .. t[4]
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "4:0123" {
		t.Fatalf("result = %q, want 4:0123", got.String())
	}
}

func TestUnpackReturnsMultipleValues(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local a, b, c = unpack({"A", "B", "C"})
local x, y = table.unpack({9, 8, 7}, 2)
result = a .. b .. c .. ":" .. x .. y
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "ABC:87" {
		t.Fatalf("result = %q, want ABC:87", got.String())
	}
}

func TestLoadStringLoadFileAndDoFile(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "loaded.lua")
	if err := os.WriteFile(script, []byte(`return "file"`), 0o600); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	defer st.Close()
	source := fmt.Sprintf(`
local a = loadstring("return 'str'")()
local b = loadfile(%q)()
local c = dofile(%q)
result = a .. ":" .. b .. ":" .. c
`, script, script)
	if err := st.DoString(context.Background(), source); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "str:file:file" {
		t.Fatalf("result = %q, want str:file:file", got.String())
	}
}

func TestLoadedChunksReturnMultipleValues(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local a, b = loadstring("return 'A', 'B'")()
result = a .. b
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "AB" {
		t.Fatalf("result = %q, want AB", got.String())
	}
}

func TestNextReturnsTableEntries(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {first = "value"}
local key, val = next(t)
result = key .. ":" .. val
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "first:value" {
		t.Fatalf("result = %q, want first:value", got.String())
	}
}

func TestLua51TableKeysPreserveValueTypes(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {}
t[true] = "bool"
t["true"] = "string"
t[1.5] = "number"
t["1.5"] = "string-number"
result = t[true] .. ":" .. t["true"] .. ":" .. t[1.5] .. ":" .. t["1.5"]
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "bool:string:number:string-number" {
		t.Fatalf("result = %q, want table keys to preserve value types", got.String())
	}
}

func TestLua51NextContinuesAfterTypedKeys(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {}
t[true] = "bool"
t["true"] = "string"
local first = next(t)
local second = next(t, first)
local third = next(t, second)
result = type(first) .. ":" .. type(second) .. ":" .. tostring(third)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "boolean:string:nil" && got.String() != "string:boolean:nil" {
		t.Fatalf("result = %q, want next to continue after typed keys", got.String())
	}
}

func TestLua51NextSkipsNilArraySlots(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {}
t[2] = "two"
t[1] = nil
local key, value = next(t)
local done = next(t, key)
result = key .. ":" .. value .. ":" .. tostring(done)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "2:two:nil" {
		t.Fatalf("result = %q, want next to skip nil array slots", got.String())
	}
}

func TestLua51TableRejectsNilKeysOnWrite(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {}
local assign_ok, assign_err = pcall(function()
  t[nil] = "bad"
end)
local rawset_ok, rawset_err = pcall(function()
  rawset(t, nil, "bad")
end)
result = tostring(assign_ok) .. ":" .. type(assign_err) .. ":" .. tostring(rawset_ok) .. ":" .. type(rawset_err) .. ":" .. tostring(next(t))
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "false:string:false:string:nil" {
		t.Fatalf("result = %q, want nil table writes rejected", got.String())
	}
}

func TestLua51TableRejectsNaNKeysOnWrite(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {}
local nan = 0 / 0
local assign_ok, assign_err = pcall(function()
  t[nan] = "bad"
end)
local rawset_ok, rawset_err = pcall(function()
  rawset(t, nan, "bad")
end)
result = tostring(assign_ok) .. ":" .. type(assign_err) .. ":" .. tostring(rawset_ok) .. ":" .. type(rawset_err) .. ":" .. tostring(next(t))
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "false:string:false:string:nil" {
		t.Fatalf("result = %q, want NaN table writes rejected", got.String())
	}
}

func TestLua51TableLengthIgnoresTrailingNilSlots(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {"a", "b", "c"}
t[3] = nil
result = #t .. ":" .. table.getn(t) .. ":" .. table.concat(t, "")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "2:2:ab" {
		t.Fatalf("result = %q, want trailing nil slots ignored by sequence length", got.String())
	}
}

func TestLua51TableInsertAppendUsesSequenceLength(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {"a", "b", "c"}
t[3] = nil
table.insert(t, "d")
result = #t .. ":" .. table.concat(t, "")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "3:abd" {
		t.Fatalf("result = %q, want table.insert append to use sequence length", got.String())
	}
}

func TestLua51TableRemoveUsesSequenceLength(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {"a", "b", "c"}
t[3] = nil
local removed = table.remove(t)
result = removed .. ":" .. #t .. ":" .. table.concat(t, "")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "b:1:a" {
		t.Fatalf("result = %q, want table.remove default to use sequence length", got.String())
	}
}

func TestLua51TableSortIgnoresTrailingNilSlots(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {3, 1, 2}
t[3] = nil
table.sort(t)
result = #t .. ":" .. table.concat(t, ",")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "2:1,3" {
		t.Fatalf("result = %q, want table.sort to ignore trailing nil slots", got.String())
	}
}

func TestCoreStringTableAndMathLibraryFunctions(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {"b", "a"}
table.insert(t, "c")
table.sort(t)
local removed = table.remove(t, 2)
local joined = table.concat(t, "-")
local text = string.sub("higolua", 1, 4) .. ":" .. string.rep("x", 3)
local n = math.max(1, 9, 3) + math.min(4, 2)
result = joined .. ":" .. removed .. ":" .. text .. ":" .. n
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "a-c:b:higo:xxx:11" {
		t.Fatalf("result = %q, want a-c:b:higo:xxx:11", got.String())
	}
}

func TestAdditionalLua51TableFunctions(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {1, 2}
rawset(t, 5, "x")
local seq = ""
table.foreachi({"a", "b"}, function(i, v)
  seq = seq .. i .. v
end)
local keySeq = ""
table.foreach({name = "lua"}, function(k, v)
  keySeq = k .. ":" .. v
end)
result = table.getn(t) .. ":" .. table.maxn(t) .. ":" .. seq .. ":" .. keySeq
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "5:5:1a2b:name:lua" {
		t.Fatalf("result = %q, want Lua 5.1 table helpers", got.String())
	}
}

func TestLua51TableForeachIRawReadsArrayValues(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {}
t[1] = "a"
t[3] = "c"
setmetatable(t, {__index = {[2] = "meta"}})
local seen = ""
table.foreachi(t, function(i, v)
  seen = seen .. i .. ":" .. tostring(v) .. ";"
end)
result = seen
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "1:a;2:nil;3:c;" {
		t.Fatalf("result = %q, want table.foreachi to raw-read array values", got.String())
	}
}

func TestLua51TableMaxNSkipsDeletedNumericSlots(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {}
t[5] = "present"
local before = table.maxn(t)
t[5] = nil
t[3] = "left"
result = before .. ":" .. table.maxn(t)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "5:3" {
		t.Fatalf("result = %q, want table.maxn to ignore deleted numeric slots", got.String())
	}
}

func TestLua51TableMaxNIncludesNonIntegerNumericKeys(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {[1] = "one"}
t[2.5] = "half"
t["9"] = "string-key"
t[-8] = "negative"
result = table.maxn(t)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "2.5" {
		t.Fatalf("result = %q, want table.maxn to include non-integer numeric keys", got.String())
	}
}

func TestLua51TableInsertPositionAndSortComparator(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {"b", "d"}
table.insert(t, 1, "a")
table.insert(t, 3, "c")
table.sort(t, function(left, right)
  return left > right
end)
result = table.concat(t, "")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "dcba" {
		t.Fatalf("result = %q, want positional insert and comparator sort", got.String())
	}
}

func TestLua51TableSortDefaultComparesNumbersNumerically(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {10, 2, 1}
table.sort(t)
result = table.concat(t, ",")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "1,2,10" {
		t.Fatalf("result = %q, want numeric table.sort order", got.String())
	}
}

func TestLua51TableConcatRejectsNonStringNumberElements(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local defaultSep = table.concat({"a", "b"})
local okBool, errBool = pcall(function()
  return table.concat({"a", true}, "")
end)
local okTable, errTable = pcall(function()
  return table.concat({"a", {}}, "")
end)
local okNil, errNil = pcall(function()
  return table.concat({"a"}, "", 1, 3)
end)
local indexed = {}
indexed[-1] = "x"
indexed[0] = "y"
indexed[1] = "z"
local explicitRange = table.concat(indexed, "", -1, 1)
local okMeta, errMeta = pcall(function()
  return table.concat(setmetatable({}, {__index = {[1] = "meta"}}), "", 1, 1)
end)
result = defaultSep .. ":" .. tostring(okBool) .. ":" .. type(errBool) .. ":" .. tostring(okTable) .. ":" .. type(errTable) .. ":" .. tostring(okNil) .. ":" .. type(errNil) .. ":" .. explicitRange .. ":" .. tostring(okMeta) .. ":" .. type(errMeta)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "ab:false:string:false:string:false:string:xyz:false:string" {
		t.Fatalf("result = %q, want concat to reject non-string/number elements", got.String())
	}
}

func TestLua51TableRemoveIgnoresExplicitZeroPosition(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {"a", "b"}
local ok, removedZero = pcall(function()
  return table.remove(t, 0)
end)
local removed = table.remove(t)
result = tostring(ok) .. ":" .. tostring(removedZero) .. ":" .. removed .. ":" .. #t
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "true:nil:b:1" {
		t.Fatalf("result = %q, want explicit zero ignored and omitted position removes last", got.String())
	}
}

func TestLua51TableRemoveIgnoresOutOfRangeExplicitPosition(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {"a", "b"}
local okFar, removedFar = pcall(function()
  return table.remove(t, 3)
end)
local okEmpty, removedEmpty = pcall(function()
  return table.remove({}, 1)
end)
result = tostring(okFar) .. ":" .. tostring(removedFar) .. ":" .. tostring(okEmpty) .. ":" .. tostring(removedEmpty) .. ":" .. table.concat(t, "")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "true:nil:true:nil:ab" {
		t.Fatalf("result = %q, want out-of-range explicit remove positions ignored", got.String())
	}
}

func TestLua51TableInsertAllowsSparseExplicitPosition(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {"a", "b"}
local okTooFar, errTooFar = pcall(function()
  table.insert(t, 5, "far")
end)
table.insert(t, "c")
result = tostring(okTooFar) .. ":" .. tostring(errTooFar) .. ":" .. tostring(t[3]) .. ":" .. tostring(t[4]) .. ":" .. t[5] .. ":" .. t[6]
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "true:nil:nil:nil:far:c" {
		t.Fatalf("result = %q, want sparse explicit insert position and append preserved", got.String())
	}
}

func TestAdditionalBaseStringAndMathFunctions(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local s, e = string.find("higolua runtime", "lua")
local replaced, count = string.gsub("go go lua", "go", "hi")
local formatted = string.format("%s:%02d", "v", 7)
math.randomseed(1)
local randomValue = math.random(5, 5)
local absValue = math.abs(-9)
local same = {}
local other = {}
local raw = rawequal(same, same) and not rawequal(same, other)
local gc = collectgarbage("count")
result = s .. "-" .. e .. ":" .. replaced .. ":" .. count .. ":" .. formatted .. ":" .. randomValue .. ":" .. absValue .. ":" .. tostring(raw) .. ":" .. type(gc)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "5-7:hi hi lua:2:v:07:5:9:true:number" {
		t.Fatalf("result = %q, want standard library aggregate", got.String())
	}
}

func TestAdditionalLua51MathFunctions(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local int, frac = math.modf(3.75)
local mant, exp = math.frexp(8)
result = math.sin(0) .. ":" .. math.cos(0) .. ":" .. math.deg(math.pi) .. ":" .. math.rad(180) .. ":" .. math.pow(2, 3) .. ":" .. math.fmod(7, 3) .. ":" .. int .. ":" .. string.format("%.2f", frac) .. ":" .. mant .. ":" .. exp .. ":" .. type(math.huge)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "0:1:180:3.141592653589793:8:1:3:0.75:0.5:4:number" {
		t.Fatalf("result = %q, want Lua 5.1 math helpers", got.String())
	}
}

func TestAdditionalLua51StringFunctions(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local a, b, c = string.byte("ABC", 1, 3)
local made = string.char(72, 105)
local reversed = string.reverse("abc")
local repPositive = string.rep("go", 2)
local repZero = string.rep("go", 0)
local repNegative = string.rep("go", -2)
result = a .. ":" .. b .. ":" .. c .. ":" .. made .. ":" .. reversed .. ":" .. repPositive .. ":" .. repZero .. ":" .. repNegative
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "65:66:67:Hi:cba:gogo::" {
		t.Fatalf("result = %q, want byte/char/reverse behavior", got.String())
	}
}

func TestLua51StringByteEmptyRangeReturnsNoValues(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local countPastEnd = select("#", string.byte("A", 2))
local countEmptyRange = select("#", string.byte("ABC", 3, 2))
local first = string.byte("A", 2)
result = countPastEnd .. ":" .. countEmptyRange .. ":" .. tostring(first)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "0:0:nil" {
		t.Fatalf("result = %q, want string.byte empty ranges to return no values", got.String())
	}
}

func TestLua51StringSubDistinguishesExplicitZeroEnd(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local omittedEnd = string.sub("abc", 2)
local zeroEnd = string.sub("abc", 1, 0)
local negativeEnd = string.sub("abc", 1, -1)
result = omittedEnd .. ":" .. zeroEnd .. ":" .. negativeEnd
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "bc::abc" {
		t.Fatalf("result = %q, want string.sub explicit zero end to be empty", got.String())
	}
}

func TestLua51StringPatternMatchFindAndGsub(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local word, digits = string.match("abc-123", "(%a+)%-(%d+)")
local s, e, captured = string.find("id=42;", "id=(%d+)")
local replaced, count = string.gsub("a1 b22", "%d+", "#")
local plainStart = string.find("a.b", ".", 1, true)
result = word .. ":" .. digits .. ":" .. s .. ":" .. e .. ":" .. captured .. ":" .. replaced .. ":" .. count .. ":" .. plainStart
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "abc:123:1:5:42:a# b#:2:2" {
		t.Fatalf("result = %q, want Lua pattern matching behavior", got.String())
	}
}

func TestLua51StringGMatchReturnsPatternIterator(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local out = ""
for word, digits in string.gmatch("a1 b22", "(%a+)(%d+)") do
  out = out .. word .. ":" .. digits .. ";"
end
local replaced = string.gsub("x=42", "(%d+)", "[%1]")
result = out .. replaced
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "a:1;b:22;x=[42]" {
		t.Fatalf("result = %q, want gmatch iterator and capture replacement", got.String())
	}
}

func TestLua51StringGsubRejectsInvalidCaptureReplacement(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local ok, err = pcall(function()
  return string.gsub("abc", "a", "%1")
end)
local replaced = string.gsub("abc", "a", "[%0]%%")
result = tostring(ok) .. ":" .. type(err) .. ":" .. replaced
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "false:string:[a]%bc" {
		t.Fatalf("result = %q, want invalid capture rejected and %%0/%%%% preserved", got.String())
	}
}

func TestLua51BalancedStringPatternBasic(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local s, e = string.find("a (b(c)d) z", "%b()")
local m = string.match("a <x<y>z> b", "%b<>")
local replaced, count = string.gsub("x [one] y [two]", "%b[]", "#")
local parts = {}
for item in string.gmatch("(a) (b(c))", "%b()") do
  parts[#parts + 1] = item
end
result = s .. ":" .. e .. ":" .. m .. ":" .. replaced .. ":" .. count .. ":" .. table.concat(parts, "|")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "3:9:<x<y>z>:x # y #:2:(a)|(b(c))" {
		t.Fatalf("result = %q, want balanced pattern behavior", got.String())
	}
}

func TestLua51StringGsubFunctionAndTableReplacement(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local called = {}
local f, fc = string.gsub("a1 b22", "(%a+)(%d+)", function(word, digits)
  called[#called + 1] = word .. digits
  return word:upper() .. ":" .. digits
end)
local t, tc = string.gsub("x y z", "%a", {x = "1", y = "2"})
result = f .. ":" .. fc .. ":" .. table.concat(called, ",") .. ":" .. t .. ":" .. tc
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "A:1 B:22:2:a1,b22:1 2 z:3" {
		t.Fatalf("result = %q, want gsub function/table replacement", got.String())
	}
}

func TestLua51FrontierStringPatternBasic(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local s, e = string.find("hi lua_51 lua", "%f[%a]lua%f[%A]")
local words = {}
for word in string.gmatch("one,two 3four", "%f[%a]%a+%f[%A]") do
  words[#words + 1] = word
end
result = s .. ":" .. e .. ":" .. table.concat(words, "|")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "4:6:one|two|four" {
		t.Fatalf("result = %q, want frontier pattern behavior", got.String())
	}
}

func TestLua51StringDumpAndLoadString(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local secret = "dumped"
local function f(name)
  return secret .. ":" .. name
end
local dumped = string.dump(f)
local loaded = assert(loadstring(dumped))
result = type(dumped) .. ":" .. loaded("fn")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "string:dumped:fn" {
		t.Fatalf("result = %q, want string.dump/loadstring behavior", got.String())
	}
}

func TestLua51LongBracketLevelsExecute(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
--[=[ comment [[ ignored ]] ]=]
result = [==[hello [=[ inner ]=] lua]==]
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "hello [=[ inner ]=] lua" {
		t.Fatalf("result = %q, want long bracket level string", got.String())
	}
}

func TestLua51LongStringDropsInitialNewline(t *testing.T) {
	result, err := higolua.NewRuntime().DoString(context.Background(), "return [[\nhello]]")
	if err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	if result.String() != "hello" {
		t.Fatalf("result = %q, want initial newline dropped from long string", result.String())
	}
}

func TestLua51NumberExponentAndDecimalStringEscapesExecute(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
result = string.format("%.3f:%s", 1e-3, "A\10\097\tz")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "0.001:A\na\tz" {
		t.Fatalf("result = %q, want exponent number and decimal escapes", got.String())
	}
}

func TestLua51LeadingDotNumberLiteral(t *testing.T) {
	result, err := higolua.NewRuntime().DoString(context.Background(), `return .5 + 1`)
	if err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	if result.String() != "1.5" {
		t.Fatalf("result = %q, want leading dot number literal", result.String())
	}
}

func TestLua51StringFormatIntegerSpecifiersCoerceNumbers(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
result = string.format("%d:%i:%x:%X:%o:%c", 7.9, -3.2, 255, 255, 9, 65)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "7:-3:ff:FF:11:A" {
		t.Fatalf("result = %q, want Lua integer format coercion", got.String())
	}
}

func TestLua51GlobalEnvironmentAndSetfenv(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
rawset(_G, "fromRaw", "ok")
newGlobal = 21
local f = function() return secret end
setfenv(f, {secret = "changed"})
result = _G.newGlobal .. ":" .. fromRaw .. ":" .. tostring(_G == getfenv(0)) .. ":" .. f()
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "21:ok:true:changed" {
		t.Fatalf("result = %q, want _G/getfenv/setfenv behavior", got.String())
	}
}

func TestLua51StackLevelSetfenvAndGetfenv(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
global_name = "outer"
function swap()
  local gf = getfenv
  local sf = setfenv
  local before = getfenv(1).global_name
  sf(1, {global_name = "inner"})
  return before .. ":" .. global_name .. ":" .. gf(1).global_name
end
result = swap() .. ":" .. global_name
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "outer:inner:inner:outer" {
		t.Fatalf("result = %q, want stack-level setfenv/getfenv behavior", got.String())
	}
}

func TestColonMethodDefinitionAndCallPassesSelf(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local obj = {name = "HiGo"}
function obj:greet(suffix)
  return self.name .. suffix
end
result = obj:greet("Lua")
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "HiGoLua" {
		t.Fatalf("result = %q, want HiGoLua", got.String())
	}
}

func TestStringColonMethodsUseStringLibrary(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
result = ("higolua"):sub(1, 4) .. ":" .. ("go"):rep(2)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "higo:gogo" {
		t.Fatalf("result = %q, want higo:gogo", got.String())
	}
}

func TestStateCancelsInfiniteLoop(t *testing.T) {
	st := state.New()
	defer st.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	if err := st.DoString(ctx, `while true do end`); err == nil {
		t.Fatal("DoString() error = nil, want context deadline error")
	}
}

func TestStableErrorTypes(t *testing.T) {
	if _, err := higolua.NewRuntime().DoString(context.Background(), `if`); err == nil {
		t.Fatal("DoString() syntax error = nil")
	} else {
		var syntaxErr *state.SyntaxError
		if !errors.As(err, &syntaxErr) {
			t.Fatalf("syntax error type = %T, want *state.SyntaxError", err)
		}
		if syntaxErr.Chunk != "string" || syntaxErr.Line != 1 || syntaxErr.Column == 0 {
			t.Fatalf("syntax error location = %q:%d:%d, want structured chunk/line/column", syntaxErr.Chunk, syntaxErr.Line, syntaxErr.Column)
		}
	}

	st := state.New()
	defer st.Close()
	if _, err := st.DoChunk(context.Background(), "runtime.lua", `
local function fail()
  return missing.field
end
fail()
`); err == nil {
		t.Fatal("DoString() runtime error = nil")
	} else {
		var runtimeErr *state.RuntimeError
		if !errors.As(err, &runtimeErr) {
			t.Fatalf("runtime error type = %T, want *state.RuntimeError", err)
		}
		if runtimeErr.Chunk != "runtime.lua" || runtimeErr.Line == 0 || runtimeErr.Column == 0 || len(runtimeErr.Stack) == 0 {
			t.Fatalf("runtime error location = %q:%d:%d, want structured chunk/line/column", runtimeErr.Chunk, runtimeErr.Line, runtimeErr.Column)
		}
	}

	st.Register("boom", func(ctx context.Context, args state.Args) (value.Value, error) {
		return value.Nil, fmt.Errorf("bridge failed")
	})
	if err := st.DoString(context.Background(), `boom()`); err == nil {
		t.Fatal("DoString() bridge error = nil")
	} else {
		var bridgeErr *state.BridgeError
		if !errors.As(err, &bridgeErr) {
			t.Fatalf("bridge error type = %T, want *state.BridgeError", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := state.New().DoString(ctx, `while true do end`); err == nil {
		t.Fatal("DoString() context error = nil")
	} else {
		var contextErr *state.ContextError
		if !errors.As(err, &contextErr) {
			t.Fatalf("context error type = %T, want *state.ContextError", err)
		}
	}
}

func TestRuntimeErrorIncludesChunkAndLuaStack(t *testing.T) {
	st := state.New()
	defer st.Close()

	_, err := st.DoChunk(context.Background(), "stack.lua", `
function inner()
  error("boom")
end
function outer()
  inner()
end
outer()
`)
	if err == nil {
		t.Fatal("DoChunk() error = nil")
	}
	var runtimeErr *state.RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("error type = %T, want *state.RuntimeError", err)
	}
	if runtimeErr.Chunk != "stack.lua" {
		t.Fatalf("RuntimeError.Chunk = %q, want stack.lua", runtimeErr.Chunk)
	}
	stack := strings.Join(runtimeErr.Stack, " > ")
	if !strings.Contains(stack, "outer") || !strings.Contains(stack, "inner") {
		t.Fatalf("RuntimeError.Stack = %#v, want outer and inner", runtimeErr.Stack)
	}
}

func TestDebugTracebackIncludesLuaStack(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
function inner()
  result = debug.traceback("boom")
end
function outer()
  inner()
end
outer()
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	text := got.String()
	if !strings.Contains(text, "boom") || !strings.Contains(text, "outer") || !strings.Contains(text, "inner") {
		t.Fatalf("traceback = %q, want message plus outer/inner stack", text)
	}
}

func TestDefaultStdlibProvidesStringMathAndPackageLoadlibError(t *testing.T) {
	st := state.New()
	defer st.Close()

	err := st.DoString(context.Background(), `
result = string.upper("hi") .. ":" .. math.floor(3.8)
bad = package.loadlib("x", "y")
`)
	if err == nil {
		t.Fatal("DoString() error = nil, want package.loadlib unsupported error")
	}

	if err := st.DoString(context.Background(), `result = string.upper("hi") .. ":" .. math.floor(3.8)`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "HI:3" {
		t.Fatalf("result = %q, want HI:3", got.String())
	}
}

func TestLua51OSLibraryCoreFunctions(t *testing.T) {
	dir := t.TempDir()
	from := filepath.Join(dir, "from.txt")
	to := filepath.Join(dir, "to.txt")
	if err := os.WriteFile(from, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	defer st.Close()
	if err := st.DoString(context.Background(), fmt.Sprintf(`
local before = os.getenv("PATH")
local diff = os.difftime(10, 4)
local clockType = type(os.clock())
local tmp = os.tmpname()
local renamed = os.rename(%q, %q)
local removed = os.remove(%q)
local locale = os.setlocale("C")
result = type(before) .. ":" .. diff .. ":" .. clockType .. ":" .. type(tmp) .. ":" .. tostring(renamed) .. ":" .. tostring(removed) .. ":" .. locale
`, from, to, to)); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "string:6:number:string:true:true:C" {
		t.Fatalf("result = %q, want os library core behavior", got.String())
	}
}

func TestLua51OSDateFormatsExplicitTime(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local formatted = os.date("!%Y-%m-%d %H:%M:%S", 0)
local t = os.date("!*t", 0)
result = formatted .. ":" .. t.year .. ":" .. t.month .. ":" .. t.day .. ":" .. t.hour .. ":" .. t.min .. ":" .. t.sec .. ":" .. tostring(t.isdst)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "1970-01-01 00:00:00:1970:1:1:0:0:0:false" {
		t.Fatalf("result = %q, want os.date explicit UTC formatting and table fields", got.String())
	}
}

func TestLua51IOLibraryOpenReadWriteAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.txt")

	st := state.New()
	defer st.Close()
	if err := st.DoString(context.Background(), fmt.Sprintf(`
local writer = assert(io.open(%q, "w"))
writer:write("hello", "\n", "lua")
writer:flush()
writer:close()
local reader = assert(io.open(%q, "r"))
local line = reader:read("*l")
local rest = reader:read("*a")
reader:close()
result = line .. ":" .. rest
`, path, path)); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "hello:lua" {
		t.Fatalf("result = %q, want io.open file read/write behavior", got.String())
	}
}

func TestLua51IOReadNumberFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "numbers.txt")
	if err := os.WriteFile(path, []byte("  -12.5 1.25e2 0x10 rest\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	defer st.Close()
	if err := st.DoString(context.Background(), fmt.Sprintf(`
local reader = assert(io.open(%q, "r"))
local n = reader:read("*n")
local exp = reader:read("*n")
local hex = reader:read("*n")
local rest = reader:read("*l")
reader:close()
result = n .. ":" .. exp .. ":" .. hex .. ":" .. rest
`, path)); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "-12.5:125:16: rest" {
		t.Fatalf("result = %q, want file:read(\"*n\") numeric scan", got.String())
	}
}

func TestLua51IOReadNumberFormatFailureDoesNotConsumeText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-number.txt")
	if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	defer st.Close()
	if err := st.DoString(context.Background(), fmt.Sprintf(`
local reader = assert(io.open(%q, "r"))
local n = reader:read("*n")
local rest = reader:read("*a")
reader:close()
result = tostring(n) .. ":" .. rest
`, path)); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "nil:abc" {
		t.Fatalf("result = %q, want failed numeric read to preserve text", got.String())
	}
}

func TestLua51IOReadAllPreservesTrailingNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "all.txt")
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	defer st.Close()
	if err := st.DoString(context.Background(), fmt.Sprintf(`
local reader = assert(io.open(%q, "r"))
local all = reader:read("*a")
reader:close()
result = string.gsub(all, "\n", "\\n")
`, path)); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "a\\nb\\n" {
		t.Fatalf("result = %q, want read all to preserve trailing newline", got.String())
	}
}

func TestLua51IOLinesIterators(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lines.txt")
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	defer st.Close()
	if err := st.DoString(context.Background(), fmt.Sprintf(`
local out = ""
for line in io.lines(%q) do
  out = out .. line
end
local f = assert(io.open(%q, "r"))
for line in f:lines() do
  out = out .. ":" .. line
end
f:close()
result = out
`, path, path)); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "ab:a:b" {
		t.Fatalf("result = %q, want io.lines and file:lines iteration", got.String())
	}
}

func TestLua51IOTmpfileTypeAndSeek(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local f = assert(io.tmpfile())
local before = io.type(f)
f:write("abcdef")
local size = f:seek("end")
local pos = f:seek("set", 2)
local text = f:read(2)
f:close()
local after = io.type(f)
result = before .. ":" .. size .. ":" .. pos .. ":" .. text .. ":" .. after
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "file:6:2:cd:closed file" {
		t.Fatalf("result = %q, want io.tmpfile/io.type/file:seek behavior", got.String())
	}
}

func TestLua51IODefaultInputOutputWriteAndFlush(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "input.txt")
	outputPath := filepath.Join(t.TempDir(), "output.txt")
	if err := os.WriteFile(inputPath, []byte("first\nsecond\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), fmt.Sprintf(`
local input = assert(io.open(%q, "r"))
local previousInput = io.input(input)
local first = io.read("*l")
local currentInput = io.input()
local output = assert(io.open(%q, "w+"))
local previousOutput = io.output(output)
io.write(first, ":", io.read("*l"))
io.flush()
io.output():seek("set", 0)
local written = io.output():read("*a")
local currentType = io.type(currentInput)
io.input(previousInput)
io.output(previousOutput)
input:close()
output:close()
result = first .. ":" .. written .. ":" .. currentType
`, inputPath, outputPath)); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "first:first:second:file" {
		t.Fatalf("result = %q, want default io input/output behavior", got.String())
	}
}

func TestLua51IOPopenAndSetvbuf(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local p = assert(io.popen("printf 'hi\\nthere\\n'", "r"))
local before = io.type(p)
local first = p:read("*l")
local rest = p:read("*a")
local setvbufResult = p:setvbuf("no")
p:close()
local after = io.type(p)
result = before .. ":" .. first .. ":" .. rest .. ":" .. tostring(setvbufResult) .. ":" .. after
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "file:hi:there\n:true:closed file" {
		t.Fatalf("result = %q, want io.popen and file:setvbuf behavior", got.String())
	}
}

func TestLua51IOCloseAndStandardStreams(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local f = assert(io.tmpfile())
local status = io.close(f)
result = type(io.stdin) .. ":" .. type(io.stdout) .. ":" .. type(io.stderr) .. ":" .. tostring(status) .. ":" .. io.type(f)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "userdata:userdata:userdata:true:closed file" {
		t.Fatalf("result = %q, want io.close and standard stream fields", got.String())
	}
}

func TestLua51DebugLibraryCoreFunctions(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local t = {}
local mt = {tag = "meta"}
debug.setmetatable(t, mt)
local got = debug.getmetatable(t)
local info = debug.getinfo(function() return 1 end)
local current = debug.getinfo(1)
result = got.tag .. ":" .. info.what .. ":" .. info.source .. ":" .. current.what .. ":" .. type(debug.traceback("boom"))
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "meta:Lua:string:Lua:string" {
		t.Fatalf("result = %q, want debug core library behavior", got.String())
	}
}

func TestLua51DebugGetInfoReportsLuaFunctionSourcePosition(t *testing.T) {
	st := state.New()
	defer st.Close()

	if _, err := st.DoChunk(context.Background(), "info.lua", `
function named()
  return 1
end
local info = debug.getinfo(named)
result = info.source .. ":" .. info.short_src .. ":" .. info.linedefined .. ":" .. info.lastlinedefined
`); err != nil {
		t.Fatalf("DoChunk() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "info.lua:info.lua:2:2" {
		t.Fatalf("result = %q, want debug.getinfo source position", got.String())
	}
}

func TestLua51DebugRegistryAndEnvWrappers(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local registry = debug.getregistry()
registry.marker = "ok"
local f = function() return secret end
debug.setfenv(f, {secret = "debug-env"})
local env = debug.getfenv(f)
result = tostring(registry == debug.getregistry()) .. ":" .. registry.marker .. ":" .. env.secret .. ":" .. f()
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "true:ok:debug-env:debug-env" {
		t.Fatalf("result = %q, want debug registry and env wrappers", got.String())
	}
}

func TestLua51DebugUpvalueAccess(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local secret = "before"
local other = "left"
local function f()
  return secret .. ":" .. other
end
local name, value = debug.getupvalue(f, 1)
local changed = debug.setupvalue(f, 1, "after")
local missing = debug.getupvalue(f, 99)
result = name .. ":" .. value .. ":" .. changed .. ":" .. tostring(missing) .. ":" .. f()
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "other:left:other:nil:before:after" && got.String() != "secret:before:secret:nil:after:left" {
		t.Fatalf("result = %q, want debug upvalue get/setup behavior", got.String())
	}
}

func TestLua51DebugLocalAccess(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
function probe()
  local alpha = "before"
  local beta = "left"
  local name, value = debug.getlocal(1, 1)
  local changed = debug.setlocal(1, 1, "after")
  local missing = debug.getlocal(1, 99)
  return name .. ":" .. value .. ":" .. changed .. ":" .. tostring(missing) .. ":" .. alpha .. ":" .. beta
end
result = probe()
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "alpha:before:alpha:nil:after:left" && got.String() != "beta:left:beta:nil:before:after" {
		t.Fatalf("result = %q, want debug local get/set behavior", got.String())
	}
}

func TestLua51DebugHookAccessors(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local function hook() return "hook" end
debug.sethook(hook, "cr", 7)
local gotHook, mask, count = debug.gethook()
debug.sethook()
local cleared = debug.gethook()
result = tostring(gotHook == hook) .. ":" .. mask .. ":" .. count .. ":" .. tostring(cleared)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "true:cr:7:nil" {
		t.Fatalf("result = %q, want debug hook accessor behavior", got.String())
	}
}

func TestLua51DebugHookDispatchesEvents(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local events = {}
debug.sethook(function(event, line)
  events[#events + 1] = event .. ":" .. type(line)
end, "crl", 2)
local function work()
  local n = 1
  n = n + 1
  return n
end
work()
debug.sethook()
local counts = {call = 0, ["return"] = 0, line = 0, count = 0}
for _, item in ipairs(events) do
  local event = string.match(item, "^[^:]+")
  counts[event] = counts[event] + 1
end
result = tostring(counts.call > 0) .. ":" .. tostring(counts["return"] > 0) .. ":" .. tostring(counts.line > 0) .. ":" .. tostring(counts.count > 0)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "true:true:true:true" {
		t.Fatalf("result = %q, want debug hook call/return/line/count events", got.String())
	}
}

func TestLua51DebugHookAppliesToBytecodeCompatibleChunks(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
events = {}
debug.sethook(function(event)
  events[event] = (events[event] or 0) + 1
end, "l", 0)
`); err != nil {
		t.Fatalf("DoString(sethook) error = %v", err)
	}
	if err := st.DoString(context.Background(), `
x = 1
x = x + 1
`); err != nil {
		t.Fatalf("DoString(bytecode-compatible chunk) error = %v", err)
	}
	if err := st.DoString(context.Background(), `debug.sethook()`); err != nil {
		t.Fatalf("DoString(clear hook) error = %v", err)
	}
	got, _ := st.GetGlobal("events")
	events, ok := got.(*value.Table)
	if !ok {
		t.Fatalf("events = %T, want table", got)
	}
	lineCount, _ := value.ToNumber(events.Get(value.String("line")))
	if lineCount == 0 {
		t.Fatalf("line hook count = 0, want hook events for bytecode-compatible chunk")
	}
}

func TestRequireUsesPackagePreloadAndStoresLoadedModule(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
package.preload.demo = function(name)
  return {value = "preloaded:" .. name}
end
local first = require("demo")
local second = require("demo")
result = first.value .. ":" .. second.value .. ":" .. tostring(package.loaded.demo == first)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "preloaded:demo:preloaded:demo:true" {
		t.Fatalf("result = %q, want preload module cached in package.loaded", got.String())
	}
}

func TestLua51PackageConfigAndCPath(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local slash = string.sub(package.config, 1, 1)
local sep = string.sub(package.config, 3, 3)
result = slash .. ":" .. sep .. ":" .. type(package.cpath) .. ":" .. package.cpath
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "/:;:string:" {
		t.Fatalf("result = %q, want package config/cpath fields", got.String())
	}
}

func TestRequireCachesFileModulesInPackageLoaded(t *testing.T) {
	dir := t.TempDir()
	modulePath := filepath.Join(dir, "demo.lua")
	if err := os.WriteFile(modulePath, []byte(`
load_count = (load_count or 0) + 1
return {count = load_count}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), fmt.Sprintf(`
package.path = %q
local first = require("demo")
local second = require("demo")
result = first.count .. ":" .. second.count .. ":" .. load_count .. ":" .. tostring(package.loaded.demo == first)
`, filepath.ToSlash(filepath.Join(dir, "?.lua")))); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "1:1:1:true" {
		t.Fatalf("result = %q, want file module loaded once and cached", got.String())
	}
}

func TestRequireUsesPackageLoadersChain(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local original = package.loaders
package.loaders = {
  function(name)
    if name == "custom" then
      return function(moduleName)
        return {value = "loaded:" .. moduleName}
      end
    end
    return "\n\tcustom loader missed"
  end
}
local mod = require("custom")
result = mod.value .. ":" .. tostring(package.loaded.custom == mod) .. ":" .. type(original[1])
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "loaded:custom:true:function" {
		t.Fatalf("result = %q, want require to use package.loaders chain", got.String())
	}
}

func TestLua51ModuleCreatesPackageLoadedTableAndEnvironment(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
shared = "visible"
module("demo.mod", package.seeall)
value = "inside"
_G.result = _NAME .. ":" .. _PACKAGE .. ":" .. _M.value .. ":" .. shared .. ":" .. tostring(package.loaded["demo.mod"] == _M)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "demo.mod:demo.:inside:visible:true" {
		t.Fatalf("result = %q, want module environment behavior", got.String())
	}
}

func TestLua51RequireModuleWithoutReturnUsesPackageLoadedTable(t *testing.T) {
	dir := t.TempDir()
	modulePath := filepath.Join(dir, "legacy.lua")
	if err := os.WriteFile(modulePath, []byte(`
module("legacy", package.seeall)
value = prefix .. ":ok"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), fmt.Sprintf(`
prefix = "module"
package.path = %q
local mod = require("legacy")
result = mod.value .. ":" .. tostring(package.loaded.legacy == mod)
`, filepath.ToSlash(filepath.Join(dir, "?.lua")))); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "module:ok:true" {
		t.Fatalf("result = %q, want require(module()) package.loaded behavior", got.String())
	}
}

func TestCoroutineResumeYieldStatusAndReturn(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local co = coroutine.create(function(prefix)
  local resumed = coroutine.yield(prefix .. "1", "pause")
  return resumed .. "2"
end)
local ok, first, marker = coroutine.resume(co, "go")
local mid = coroutine.status(co)
local ok2, final = coroutine.resume(co, "back")
result = tostring(ok) .. ":" .. first .. ":" .. marker .. ":" .. mid .. ":" .. tostring(ok2) .. ":" .. final .. ":" .. coroutine.status(co)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "true:go1:pause:suspended:true:back2:dead" {
		t.Fatalf("result = %q, want coroutine yield/resume lifecycle", got.String())
	}
}

func TestLua51CoroutineCreateRequiresFunction(t *testing.T) {
	st := state.New()
	defer st.Close()

	if err := st.DoString(context.Background(), `
local okCreate, createErr = pcall(function()
  return coroutine.create(123)
end)
local okWrap, wrapErr = pcall(function()
  return coroutine.wrap("bad")
end)
result = tostring(okCreate) .. ":" .. type(createErr) .. ":" .. tostring(okWrap) .. ":" .. type(wrapErr)
`); err != nil {
		t.Fatalf("DoString() error = %v", err)
	}
	got, _ := st.GetGlobal("result")
	if got.String() != "false:string:false:string" {
		t.Fatalf("result = %q, want coroutine create/wrap to reject non-functions", got.String())
	}
}
