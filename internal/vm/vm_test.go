package vm_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Hiveton/higo-lua/internal/bytecode"
	"github.com/Hiveton/higo-lua/internal/parser"
	"github.com/Hiveton/higo-lua/internal/vm"
	"github.com/Hiveton/higo-lua/value"
)

type testGlobals map[string]value.Value

func (g testGlobals) Get(name string) value.Value {
	if v, ok := g[name]; ok {
		return v
	}
	return value.Nil
}

func (g testGlobals) Set(name string, v value.Value) { g[name] = v }

type externalFunc struct {
	fn func(context.Context, []value.Value) ([]value.Value, error)
}

func (f externalFunc) Type() value.Type { return value.TypeFunction }
func (f externalFunc) String() string   { return "function: external" }

type testCaller struct{}

func (testCaller) Call(ctx context.Context, fn value.Value, args []value.Value) ([]value.Value, error) {
	f, ok := fn.(externalFunc)
	if !ok {
		return nil, nil
	}
	return f.fn(ctx, args)
}

func valuesString(values []value.Value) string {
	parts := make([]string, len(values))
	for i, v := range values {
		if v == nil {
			v = value.Nil
		}
		parts[i] = v.String()
	}
	return strings.Join(parts, ":")
}

func TestBytecodeVMExecutesCompiledArithmeticReturn(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `return 1 + 2 * 3`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "7" {
		t.Fatalf("result = %s, want 7", result.String())
	}
}

func TestBytecodeVMCallsExternalFunction(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `return add(20, 22)`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.CompileWithHostCalls(chunk, []string{"add"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	globals := testGlobals{
		"add": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			left, _ := value.ToNumber(args[0])
			right, _ := value.ToNumber(args[1])
			return []value.Value{value.Number(left + right)}, nil
		}},
	}
	result, err := vm.New(vm.WithGlobals(globals), vm.WithCaller(testCaller{})).Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "42" {
		t.Fatalf("result = %s, want 42", result.String())
	}
}

func TestBytecodeVMExpandsGenericForExpressionCall(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local sum = 0
for i, v in pairs({2, 4, 6}) do
  sum = sum + i * v
end
return sum
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.CompileWithHostCalls(chunk, []string{"pairs"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	pairsIter := externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
		tbl := args[0].(*value.Table)
		current, _ := value.ToNumber(args[1])
		next := int(current) + 1
		v := tbl.Get(value.Number(next))
		if v == value.Nil {
			return []value.Value{value.Nil}, nil
		}
		return []value.Value{value.Number(next), v}, nil
	}}
	globals := testGlobals{
		"pairs": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			return []value.Value{pairsIter, args[0], value.Number(0)}, nil
		}},
	}
	result, err := vm.New(vm.WithGlobals(globals), vm.WithCaller(testCaller{})).Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "28" {
		t.Fatalf("result = %s, want 28", result.String())
	}
}

func TestBytecodeCompilerSupportsMultiReturnForVM(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `return "A", "B"`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	results, err := vm.New().ExecuteValues(context.Background(), proto)
	if err != nil {
		t.Fatalf("ExecuteValues() error = %v", err)
	}
	if len(results) != 2 || results[0].String() != "A" || results[1].String() != "B" {
		t.Fatalf("results = %#v, want A/B", results)
	}
}

func TestBytecodeVMExecutesMultipleReturnValues(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `return "A", 2, "C"`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	results, err := vm.New().ExecuteValues(context.Background(), proto)
	if err != nil {
		t.Fatalf("ExecuteValues() error = %v", err)
	}
	if len(results) != 3 || results[0].String() != "A" || results[1].String() != "2" || results[2].String() != "C" {
		t.Fatalf("results = %#v, want A/2/C", results)
	}
}

func TestBytecodeVMExpandsFinalCallInReturnList(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local function pair()
  return "B", "C"
end

return "A", pair()
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	results, err := vm.New().ExecuteValues(context.Background(), proto)
	if err != nil {
		t.Fatalf("ExecuteValues() error = %v", err)
	}
	if len(results) != 3 || results[0].String() != "A" || results[1].String() != "B" || results[2].String() != "C" {
		t.Fatalf("results = %#v, want A/B/C", results)
	}
}

func TestBytecodeVMExpandsFinalCallInLocalAssignment(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local function pair()
  return "B", "C"
end

local a, b, c = "A", pair()
return a .. ":" .. b .. ":" .. c
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "A:B:C" {
		t.Fatalf("result = %s, want A:B:C", result.String())
	}
}

func TestBytecodeVMExpandsFinalCallInTableConstructor(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local function pair()
  return "B", "C"
end

local t = {"A", pair()}
return t[1] .. ":" .. t[2] .. ":" .. t[3]
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "A:B:C" {
		t.Fatalf("result = %s, want A:B:C", result.String())
	}
}

func TestBytecodeVMExpandsFinalCallInAssignment(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local function pair()
  return "B", "C"
end

local a
local b
local c
a, b, c = "A", pair()
return a .. ":" .. b .. ":" .. c
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "A:B:C" {
		t.Fatalf("result = %s, want A:B:C", result.String())
	}
}

func TestBytecodeVMExecutesGlobalReadWrite(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
answer = seed + 1
return answer
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	globals := testGlobals{"seed": value.Number(41)}
	result, err := vm.New(vm.WithGlobals(globals)).Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "42" || globals["answer"].String() != "42" {
		t.Fatalf("result = %s globals = %#v, want global answer 42", result.String(), globals)
	}
}

func TestBytecodeVMExecutesLocalAssignments(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local a = 10
local b = a + 5
return b .. ":ok"
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "15:ok" {
		t.Fatalf("result = %s, want 15:ok", result.String())
	}
}

func TestBytecodeVMExecutesPowerOperator(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `return 2 ^ 3 ^ 2`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "512" {
		t.Fatalf("result = %s, want 512", result.String())
	}
}

func TestBytecodeVMExecutesComparisonOperators(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `return (1 < 2) .. ":" .. (2 <= 2) .. ":" .. (3 > 4) .. ":" .. ("a" ~= "b") .. ":" .. ("x" == "x")`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "true:true:false:true:true" {
		t.Fatalf("result = %s, want comparison results", result.String())
	}
}

func TestBytecodeVMHonorsComparisonMetamethods(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local mt = {
  __lt = function(a, b) return true end,
  __le = function(a, b) return false end
}
local a = setmetatable({value = 3}, mt)
return (a < a), (a <= a), (a > a), (a >= a)
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.CompileWithHostCalls(chunk, []string{"setmetatable"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	globals := testGlobals{
		"setmetatable": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			t, _ := args[0].(*value.Table)
			mt, _ := args[1].(*value.Table)
			t.SetMetatable(mt)
			return []value.Value{t}, nil
		}},
	}
	results, err := vm.New(vm.WithGlobals(globals), vm.WithCaller(testCaller{})).ExecuteValues(context.Background(), proto)
	if err != nil {
		t.Fatalf("ExecuteValues() error = %v", err)
	}
	got := valuesString(results)
	if got != "true:false:true:false" {
		t.Fatalf("results = %s, want true:false:true:false", got)
	}
}

func TestBytecodeVMHonorsEqMetamethod(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local mt = {
  __eq = function(a, b) return a.id == b.id end
}
local a = setmetatable({id = "same"}, mt)
local b = setmetatable({id = "same"}, mt)
local c = setmetatable({id = "other"}, mt)
return a == b, a ~= c
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.CompileWithHostCalls(chunk, []string{"setmetatable"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	globals := testGlobals{
		"setmetatable": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			t, _ := args[0].(*value.Table)
			mt, _ := args[1].(*value.Table)
			t.SetMetatable(mt)
			return []value.Value{t}, nil
		}},
	}
	results, err := vm.New(vm.WithGlobals(globals), vm.WithCaller(testCaller{})).ExecuteValues(context.Background(), proto)
	if err != nil {
		t.Fatalf("ExecuteValues() error = %v", err)
	}
	got := valuesString(results)
	if got != "true:true" {
		t.Fatalf("results = %s, want true:true", got)
	}
}

func TestBytecodeVMExecutesUnaryOperators(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `return (-5) .. ":" .. (not false) .. ":" .. #"higo"`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "-5:true:4" {
		t.Fatalf("result = %s, want unary results", result.String())
	}
}

func TestBytecodeVMHonorsLenMetamethod(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local t = setmetatable({}, {
  __len = function(self)
    return 42
  end
})
return #t
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.CompileWithHostCalls(chunk, []string{"setmetatable"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	globals := testGlobals{
		"setmetatable": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			t, _ := args[0].(*value.Table)
			mt, _ := args[1].(*value.Table)
			t.SetMetatable(mt)
			return []value.Value{t}, nil
		}},
	}
	result, err := vm.New(vm.WithGlobals(globals), vm.WithCaller(testCaller{})).Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "42" {
		t.Fatalf("result = %s, want 42", result.String())
	}
}

func TestBytecodeVMHonorsUnaryMinusMetamethod(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local t = setmetatable({value = 7}, {
  __unm = function(self)
    return self.value * 10
  end
})
return -t
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.CompileWithHostCalls(chunk, []string{"setmetatable"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	globals := testGlobals{
		"setmetatable": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			t, _ := args[0].(*value.Table)
			mt, _ := args[1].(*value.Table)
			t.SetMetatable(mt)
			return []value.Value{t}, nil
		}},
	}
	result, err := vm.New(vm.WithGlobals(globals), vm.WithCaller(testCaller{})).Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "70" {
		t.Fatalf("result = %s, want 70", result.String())
	}
}

func TestBytecodeVMExecutesIfElseControlFlow(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local label = "unset"
if seed > 10 then
  label = "big"
else
  label = "small"
end
return label
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New(vm.WithGlobals(testGlobals{"seed": value.Number(11)})).Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "big" {
		t.Fatalf("result = %s, want big", result.String())
	}

	result, err = vm.New(vm.WithGlobals(testGlobals{"seed": value.Number(9)})).Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "small" {
		t.Fatalf("result = %s, want small", result.String())
	}
}

func TestBytecodeVMExecutesWhileLoop(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local i = 0
local sum = 0
while i < 4 do
  i = i + 1
  sum = sum + i
end
return sum
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "10" {
		t.Fatalf("result = %s, want 10", result.String())
	}
}

func TestBytecodeVMExecutesNumericForLoop(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local sum = 0
for i = 1, 5, 2 do
  sum = sum + i
end
return sum
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "9" {
		t.Fatalf("result = %s, want 9", result.String())
	}
}

func TestBytecodeVMExecutesNegativeStepNumericForLoop(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local out = ""
for i = 3, 1, -1 do
  out = out .. i
end
return out
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "321" {
		t.Fatalf("result = %s, want 321", result.String())
	}
}

func TestBytecodeVMExecutesRepeatUntilAndBreak(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local i = 0
local out = ""
repeat
  i = i + 1
  if i == 4 then
    break
  end
  out = out .. i
until i > 10
return out
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "123" {
		t.Fatalf("result = %s, want 123", result.String())
	}
}

func TestBytecodeVMExecutesLuaFunctionCalls(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local function add(a, b)
  return a + b
end

function twice(x)
  return x * 2
end

return add(20, twice(11))
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New(vm.WithGlobals(testGlobals{})).Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "42" {
		t.Fatalf("result = %s, want 42", result.String())
	}
}

func TestBytecodeVMExecutesTableConstructorIndexAndAssignment(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local t = {10, label = "hi", [3] = 30}
t.extra = t[1] + t[3]
return t.label .. ":" .. t.extra
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "hi:40" {
		t.Fatalf("result = %s, want hi:40", result.String())
	}
}

func TestBytecodeVMExecutesTableMethodCall(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local t = {base = 20}
t.add = function(self, value)
  return self.base + value
end
return t:add(22)
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "42" {
		t.Fatalf("result = %s, want 42", result.String())
	}
}

func TestBytecodeVMExecutesDottedAndMethodFunctionDeclarations(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local mod = {sub = {name = "module"}}
function mod.sub.answer()
  return 42
end
function mod.sub:greet(suffix)
  return self.name .. suffix
end
return mod.sub.answer() .. ":" .. mod.sub:greet(" ok")
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "42:module ok" {
		t.Fatalf("result = %s, want 42:module ok", result.String())
	}
}

func TestBytecodeVMExecutesGlobalDottedAndMethodFunctionDeclarations(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
mod = {sub = {name = "global"}}
function mod.sub.answer()
  return 42
end
function mod.sub:greet(suffix)
  return self.name .. suffix
end
return mod.sub.answer() .. ":" .. mod.sub:greet(" ok")
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New(vm.WithGlobals(testGlobals{})).Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "42:global ok" {
		t.Fatalf("result = %s, want 42:global ok", result.String())
	}
}

func TestBytecodeVMHonorsNewIndexTableMetamethod(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local shadow = {}
local t = {existing = "kept", remove_me = "gone"}
setmetatable(t, {__newindex = shadow})
t.missing = "proxied"
t.existing = "changed"
t.remove_me = nil
return t.existing, shadow.missing, rawget(t, "missing"), rawget(t, "remove_me")
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.CompileWithHostCalls(chunk, []string{"setmetatable", "rawget"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	globals := testGlobals{
		"setmetatable": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			t, _ := args[0].(*value.Table)
			mt, _ := args[1].(*value.Table)
			t.SetMetatable(mt)
			return []value.Value{t}, nil
		}},
		"rawget": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			t, _ := args[0].(*value.Table)
			return []value.Value{t.RawGet(args[1])}, nil
		}},
	}
	results, err := vm.New(vm.WithGlobals(globals), vm.WithCaller(testCaller{})).ExecuteValues(context.Background(), proto)
	if err != nil {
		t.Fatalf("ExecuteValues() error = %v", err)
	}
	got := valuesString(results)
	if got != "changed:proxied:nil:nil" {
		t.Fatalf("results = %s, want changed:proxied:nil:nil", got)
	}
}

func TestBytecodeVMHonorsNewIndexFunctionMetamethod(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local sink = {}
local t = setmetatable({}, {
  __newindex = function(self, key, value)
    sink[key] = value .. ":seen"
  end
})
t.name = "lua"
return sink.name, rawget(t, "name")
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.CompileWithHostCalls(chunk, []string{"setmetatable", "rawget"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	globals := testGlobals{
		"setmetatable": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			t, _ := args[0].(*value.Table)
			mt, _ := args[1].(*value.Table)
			t.SetMetatable(mt)
			return []value.Value{t}, nil
		}},
		"rawget": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			t, _ := args[0].(*value.Table)
			return []value.Value{t.RawGet(args[1])}, nil
		}},
	}
	results, err := vm.New(vm.WithGlobals(globals), vm.WithCaller(testCaller{})).ExecuteValues(context.Background(), proto)
	if err != nil {
		t.Fatalf("ExecuteValues() error = %v", err)
	}
	got := valuesString(results)
	if got != "lua:seen:nil" {
		t.Fatalf("results = %s, want lua:seen:nil", got)
	}
}

func TestBytecodeVMHonorsCallTableMetamethod(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local callable = setmetatable({prefix = "hi"}, {
  __call = function(self, name)
    return self.prefix .. ":" .. name
  end
})
return callable("lua")
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.CompileWithHostCalls(chunk, []string{"setmetatable"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	globals := testGlobals{
		"setmetatable": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			t, _ := args[0].(*value.Table)
			mt, _ := args[1].(*value.Table)
			t.SetMetatable(mt)
			return []value.Value{t}, nil
		}},
	}
	result, err := vm.New(vm.WithGlobals(globals), vm.WithCaller(testCaller{})).Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "hi:lua" {
		t.Fatalf("result = %s, want hi:lua", result.String())
	}
}

func TestBytecodeVMHonorsArithmeticMetamethod(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local mt = {
  __add = function(a, b)
    return {value = a.value + b.value}
  end
}
local a = setmetatable({value = 10}, mt)
local b = setmetatable({value = 5}, mt)
local c = a + b
return c.value
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.CompileWithHostCalls(chunk, []string{"setmetatable"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	globals := testGlobals{
		"setmetatable": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			t, _ := args[0].(*value.Table)
			mt, _ := args[1].(*value.Table)
			t.SetMetatable(mt)
			return []value.Value{t}, nil
		}},
	}
	result, err := vm.New(vm.WithGlobals(globals), vm.WithCaller(testCaller{})).Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "15" {
		t.Fatalf("result = %s, want 15", result.String())
	}
}

func TestBytecodeVMHonorsConcatMetamethod(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local mt = {
  __concat = function(a, b)
    return a.label .. ":" .. b
  end
}
local a = setmetatable({label = "left"}, mt)
return a .. "suffix"
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.CompileWithHostCalls(chunk, []string{"setmetatable"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	globals := testGlobals{
		"setmetatable": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			t, _ := args[0].(*value.Table)
			mt, _ := args[1].(*value.Table)
			t.SetMetatable(mt)
			return []value.Value{t}, nil
		}},
	}
	result, err := vm.New(vm.WithGlobals(globals), vm.WithCaller(testCaller{})).Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "left:suffix" {
		t.Fatalf("result = %s, want left:suffix", result.String())
	}
}

func TestBytecodeVMHonorsArithmeticMetamethodFamily(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local mt = {
  __sub = function(a, b) return {value = a.value - b.value} end,
  __mul = function(a, b) return {value = a.value * b.value} end,
  __div = function(a, b) return {value = a.value / b.value} end,
  __mod = function(a, b) return {value = a.value % b.value} end,
  __pow = function(a, b) return {value = a.value ^ b.value} end,
}
local a = setmetatable({value = 10}, mt)
local b = setmetatable({value = 3}, mt)
return (a - b).value, (a * b).value, (a / b).value, (a % b).value, (a ^ b).value
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.CompileWithHostCalls(chunk, []string{"setmetatable"})
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	globals := testGlobals{
		"setmetatable": externalFunc{fn: func(ctx context.Context, args []value.Value) ([]value.Value, error) {
			t, _ := args[0].(*value.Table)
			mt, _ := args[1].(*value.Table)
			t.SetMetatable(mt)
			return []value.Value{t}, nil
		}},
	}
	results, err := vm.New(vm.WithGlobals(globals), vm.WithCaller(testCaller{})).ExecuteValues(context.Background(), proto)
	if err != nil {
		t.Fatalf("ExecuteValues() error = %v", err)
	}
	got := valuesString(results)
	if got != "7:30:3.3333333333333335:1:1000" {
		t.Fatalf("results = %s, want arithmetic metamethod family output", got)
	}
}

func TestBytecodeVMExecutesAndOrShortCircuitWithLuaValues(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local t = {value = "kept"}
local a = false and missing.value
local b = t or missing.value
local c = nil or "fallback"
return a .. ":" .. b.value .. ":" .. c
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "false:kept:fallback" {
		t.Fatalf("result = %s, want false:kept:fallback", result.String())
	}
}

func TestBytecodeVMExecutesVarargFunction(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local function pick(prefix, ...)
  return prefix .. ":" .. (...) .. ":" .. arg[2]
end
return pick("v", "first", "second")
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "v:first:second" {
		t.Fatalf("result = %s, want v:first:second", result.String())
	}
}

func TestBytecodeVMExecutesMutableClosureUpvalues(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local function make_counter(start)
  local count = start
  return function(step)
    count = count + step
    return count
  end
end

local counter = make_counter(10)
return counter(2) .. ":" .. counter(3)
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "12:15" {
		t.Fatalf("result = %s, want 12:15", result.String())
	}
}

func TestBytecodeVMExecutesGenericForWithClosureIterator(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local function iter(state, current)
  local next = current + 1
  if next <= state.limit then
    return next, state.values[next]
  end
end

local sum = 0
for i, v in iter, {limit = 3, values = {2, 4, 6}}, 0 do
  sum = sum + i * v
end
return sum
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "28" {
		t.Fatalf("result = %s, want 28", result.String())
	}
}

func TestBytecodeVMExecutesDoBlockLexicalScope(t *testing.T) {
	chunk, err := parser.Parse("bytecode.lua", `
local outer = "outer"
local seen = ""
do
  local outer = "inner"
  seen = outer
end
return seen .. ":" .. outer
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	proto, err := bytecode.Compile(chunk)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	result, err := vm.New().Execute(context.Background(), proto)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.String() != "inner:outer" {
		t.Fatalf("result = %s, want inner:outer", result.String())
	}
}
