package state

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hiveton/higolua/internal/parser"
)

func TestBytecodeHostCallsIncludesMetatablePrimitives(t *testing.T) {
	st := New()
	defer st.Close()

	got := map[string]bool{}
	for _, name := range st.bytecodeHostCalls() {
		got[name] = true
	}
	for _, name := range []string{"setmetatable", "getmetatable", "rawget", "rawset", "type", "tostring", "tonumber", "assert", "error", "rawequal", "collectgarbage", "gcinfo", "next"} {
		if !got[name] {
			t.Fatalf("bytecodeHostCalls missing %s", name)
		}
	}
}

func TestBytecodeHostTablesIncludesLoadedStdlibTables(t *testing.T) {
	st := New()
	defer st.Close()

	got := map[string]bool{}
	for _, name := range st.bytecodeHostTables() {
		got[name] = true
	}
	for _, name := range []string{"string", "math", "table", "package", "coroutine", "io", "os", "debug"} {
		if !got[name] {
			t.Fatalf("bytecodeHostTables missing %s", name)
		}
	}
}

func TestBytecodePathCallsBaseTypeAndToString(t *testing.T) {
	st := New()
	defer st.Close()

	chunk, err := parser.Parse("bytecode.lua", `return tostring(123) .. ":" .. type({})`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	results, ok, err := st.tryBytecode(context.Background(), chunk)
	if err != nil {
		t.Fatalf("tryBytecode() error = %v", err)
	}
	if !ok {
		t.Fatal("tryBytecode() ok = false, want bytecode execution")
	}
	if len(results) != 1 || results[0].String() != "123:table" {
		t.Fatalf("results = %v, want 123:table", results)
	}
}

func TestBytecodePathCallsStringColonMethods(t *testing.T) {
	st := New()
	defer st.Close()

	chunk, err := parser.Parse("bytecode.lua", `return ("higolua"):sub(1, 4) .. ":" .. ("go"):rep(2)`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	results, ok, err := st.tryBytecode(context.Background(), chunk)
	if err != nil {
		t.Fatalf("tryBytecode() error = %v", err)
	}
	if !ok {
		t.Fatal("tryBytecode() ok = false, want bytecode execution")
	}
	if len(results) != 1 || results[0].String() != "higo:gogo" {
		t.Fatalf("results = %v, want higo:gogo", results)
	}
}

func TestBytecodePathCallsLocalStringColonMethods(t *testing.T) {
	st := New()
	defer st.Close()

	chunk, err := parser.Parse("bytecode.lua", `
local name = "higolua"
local repeated = "go"
return name:sub(1, 4) .. ":" .. repeated:rep(2)
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	results, ok, err := st.tryBytecode(context.Background(), chunk)
	if err != nil {
		t.Fatalf("tryBytecode() error = %v", err)
	}
	if !ok {
		t.Fatal("tryBytecode() ok = false, want bytecode execution")
	}
	if len(results) != 1 || results[0].String() != "higo:gogo" {
		t.Fatalf("results = %v, want higo:gogo", results)
	}
}

func TestBytecodePathCallsStandardLibraryTableFunctions(t *testing.T) {
	st := New()
	defer st.Close()

	chunk, err := parser.Parse("bytecode.lua", `
local t = {"b", "a"}
table.insert(t, "c")
table.sort(t)
local removed = table.remove(t, 2)
local joined = table.concat(t, "-")
local text = string.sub("higolua", 1, 4) .. ":" .. string.rep("x", 3)
local n = math.max(1, 9, 3) + math.min(4, 2)
return joined .. ":" .. removed .. ":" .. text .. ":" .. n
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	results, ok, err := st.tryBytecode(context.Background(), chunk)
	if err != nil {
		t.Fatalf("tryBytecode() error = %v", err)
	}
	if !ok {
		t.Fatal("tryBytecode() ok = false, want bytecode execution")
	}
	if len(results) != 1 || results[0].String() != "a-c:b:higo:xxx:11" {
		t.Fatalf("results = %v, want a-c:b:higo:xxx:11", results)
	}
}

func TestBytecodePathCallsBaseToNumber(t *testing.T) {
	st := New()
	defer st.Close()

	chunk, err := parser.Parse("bytecode.lua", `return tonumber("ff", 16) .. ":" .. tonumber("101", 2) .. ":" .. tostring(tonumber("z", 10))`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	results, ok, err := st.tryBytecode(context.Background(), chunk)
	if err != nil {
		t.Fatalf("tryBytecode() error = %v", err)
	}
	if !ok {
		t.Fatal("tryBytecode() ok = false, want bytecode execution")
	}
	if len(results) != 1 || results[0].String() != "255:5:nil" {
		t.Fatalf("results = %v, want 255:5:nil", results)
	}
}

func TestBytecodePathCallsRawEqual(t *testing.T) {
	st := New()
	defer st.Close()

	chunk, err := parser.Parse("bytecode.lua", `
local same = {}
local other = {}
return tostring(rawequal(same, same)) .. ":" .. tostring(rawequal(same, other))
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	results, ok, err := st.tryBytecode(context.Background(), chunk)
	if err != nil {
		t.Fatalf("tryBytecode() error = %v", err)
	}
	if !ok {
		t.Fatal("tryBytecode() ok = false, want bytecode execution")
	}
	if len(results) != 1 || results[0].String() != "true:false" {
		t.Fatalf("results = %v, want true:false", results)
	}
}

func TestBytecodePathCallsNext(t *testing.T) {
	st := New()
	defer st.Close()

	chunk, err := parser.Parse("bytecode.lua", `
local t = {first = "value"}
local key, val = next(t)
return key .. ":" .. val
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	results, ok, err := st.tryBytecode(context.Background(), chunk)
	if err != nil {
		t.Fatalf("tryBytecode() error = %v", err)
	}
	if !ok {
		t.Fatal("tryBytecode() ok = false, want bytecode execution")
	}
	if len(results) != 1 || results[0].String() != "first:value" {
		t.Fatalf("results = %v, want first:value", results)
	}
}

func TestBytecodePathCallsGarbageCollectionBaseFunctions(t *testing.T) {
	st := New()
	defer st.Close()

	chunk, err := parser.Parse("bytecode.lua", `return collectgarbage("count") .. ":" .. type(gcinfo())`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	results, ok, err := st.tryBytecode(context.Background(), chunk)
	if err != nil {
		t.Fatalf("tryBytecode() error = %v", err)
	}
	if !ok {
		t.Fatal("tryBytecode() ok = false, want bytecode execution")
	}
	if len(results) != 1 || results[0].String() != "0:number" {
		t.Fatalf("results = %v, want 0:number", results)
	}
}

func TestBytecodePathCallsAssert(t *testing.T) {
	st := New()
	defer st.Close()

	chunk, err := parser.Parse("bytecode.lua", `return assert("ok", "no")`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	results, ok, err := st.tryBytecode(context.Background(), chunk)
	if err != nil {
		t.Fatalf("tryBytecode() error = %v", err)
	}
	if !ok {
		t.Fatal("tryBytecode() ok = false, want bytecode execution")
	}
	if len(results) != 2 || results[0].String() != "ok" || results[1].String() != "no" {
		t.Fatalf("results = %v, want ok,no", results)
	}
}

func TestBytecodePathAssertReturnsAllArguments(t *testing.T) {
	st := New()
	defer st.Close()

	chunk, err := parser.Parse("bytecode.lua", `return assert("ok", "left", "right")`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	results, ok, err := st.tryBytecode(context.Background(), chunk)
	if err != nil {
		t.Fatalf("tryBytecode() error = %v", err)
	}
	if !ok {
		t.Fatal("tryBytecode() ok = false, want bytecode execution")
	}
	if len(results) != 3 || results[0].String() != "ok" || results[1].String() != "left" || results[2].String() != "right" {
		t.Fatalf("results = %v, want ok,left,right", results)
	}
}

func TestBytecodePathPropagatesAssertError(t *testing.T) {
	st := New()
	defer st.Close()

	chunk, err := parser.Parse("bytecode.lua", `return assert(false, "boom")`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	_, ok, err := st.tryBytecode(context.Background(), chunk)
	if !ok {
		t.Fatal("tryBytecode() ok = false, want bytecode execution")
	}
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("tryBytecode() error = %v, want boom", err)
	}
}

func TestBytecodePathPropagatesErrorFunction(t *testing.T) {
	st := New()
	defer st.Close()

	chunk, err := parser.Parse("bytecode.lua", `return error("boom")`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	_, ok, err := st.tryBytecode(context.Background(), chunk)
	if !ok {
		t.Fatal("tryBytecode() ok = false, want bytecode execution")
	}
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Error() != "boom" {
		t.Fatalf("tryBytecode() error = %T %v, want RuntimeError boom", err, err)
	}
}
