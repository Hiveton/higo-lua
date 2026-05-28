package vm

import (
	"context"
	"fmt"
	"math"

	"github.com/Hiveton/higo-lua/internal/bytecode"
	"github.com/Hiveton/higo-lua/value"
)

type Engine interface {
	Execute(context.Context, *bytecode.Prototype) (value.Value, error)
}

type Globals interface {
	Get(string) value.Value
	Set(string, value.Value)
}

type Caller interface {
	Call(context.Context, value.Value, []value.Value) ([]value.Value, error)
}

type Option func(*VM)

func WithGlobals(globals Globals) Option {
	return func(v *VM) { v.globals = globals }
}

func WithCaller(caller Caller) Option {
	return func(v *VM) { v.caller = caller }
}

type VM struct {
	globals Globals
	caller  Caller
}

type closure struct {
	proto    *bytecode.Prototype
	globals  Globals
	upvalues []*cell
}

func (c *closure) Type() value.Type { return value.TypeFunction }
func (c *closure) String() string   { return fmt.Sprintf("function: %p", c) }

type cell struct {
	v           value.Value
	resultCount int
}

func New(options ...Option) *VM {
	v := &VM{}
	for _, option := range options {
		option(v)
	}
	return v
}

func (v *VM) Execute(ctx context.Context, proto *bytecode.Prototype) (value.Value, error) {
	values, err := v.ExecuteValues(ctx, proto)
	if err != nil {
		return value.Nil, err
	}
	return first(values), nil
}

func (v *VM) ExecuteValues(ctx context.Context, proto *bytecode.Prototype) ([]value.Value, error) {
	return v.execute(ctx, proto, nil, nil)
}

func (v *VM) execute(ctx context.Context, proto *bytecode.Prototype, args []value.Value, upvalues []*cell) ([]value.Value, error) {
	registers := make([]*cell, proto.Registers+1)
	for i := range registers {
		registers[i] = &cell{v: value.Nil}
	}
	varargs := []value.Value(nil)
	for i, arg := range args {
		if i < len(proto.Params) && i < len(registers) {
			registers[i].v = arg
			continue
		}
		if proto.Vararg {
			varargs = append(varargs, arg)
		}
	}
	if proto.Vararg && proto.ArgRegister >= 0 && proto.ArgRegister < len(registers) {
		argTable := value.NewTable()
		for _, arg := range varargs {
			argTable.Append(arg)
		}
		argTable.Set(value.String("n"), value.Number(len(varargs)))
		registers[proto.ArgRegister].v = argTable
	}
	for pc := 0; pc < len(proto.Instructions); pc++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ins := proto.Instructions[pc]
		switch ins.Op {
		case bytecode.OpMove:
			registers[ins.A].v = registers[ins.B].v
			registers[ins.A].resultCount = registers[ins.B].resultCount
		case bytecode.OpLoadConst:
			registers[ins.A].v = proto.Constants[ins.B]
			registers[ins.A].resultCount = 1
		case bytecode.OpGetGlobal:
			if v.globals == nil {
				registers[ins.A].v = value.Nil
				continue
			}
			registers[ins.A].v = v.globals.Get(proto.Constants[ins.B].String())
		case bytecode.OpSetGlobal:
			if v.globals == nil {
				return nil, fmt.Errorf("bytecode: globals are not configured")
			}
			v.globals.Set(proto.Constants[ins.A].String(), registers[ins.B].v)
		case bytecode.OpReturn:
			if ins.A < 0 {
				return []value.Value{value.Nil}, nil
			}
			count := 1
			if ins.C > 0 {
				count = ins.C
			} else if ins.C < 0 {
				fixed := ins.B - ins.A
				lastCount := registers[ins.B].resultCount
				if lastCount <= 0 {
					lastCount = 1
				}
				count = fixed + lastCount
			}
			results := make([]value.Value, 0, count)
			for i := 0; i < count; i++ {
				results = append(results, registers[ins.A+i].v)
			}
			return results, nil
		case bytecode.OpAdd, bytecode.OpSub, bytecode.OpMul, bytecode.OpDiv, bytecode.OpMod, bytecode.OpPow:
			result, err := v.binaryArithmetic(ctx, ins.Op, registers[ins.B].v, registers[ins.C].v)
			if err != nil {
				return nil, err
			}
			registers[ins.A].v = result
		case bytecode.OpConcat:
			result, err := v.concat(ctx, registers[ins.B].v, registers[ins.C].v)
			if err != nil {
				return nil, err
			}
			registers[ins.A].v = result
		case bytecode.OpEq, bytecode.OpNE, bytecode.OpLT, bytecode.OpLE, bytecode.OpGT, bytecode.OpGE:
			result, err := v.compare(ctx, ins.Op, registers[ins.B].v, registers[ins.C].v)
			if err != nil {
				return nil, err
			}
			registers[ins.A].v = value.Bool(result)
		case bytecode.OpNeg:
			result, err := v.neg(ctx, registers[ins.B].v)
			if err != nil {
				return nil, err
			}
			registers[ins.A].v = result
		case bytecode.OpNot:
			registers[ins.A].v = value.Bool(!value.IsTruthy(registers[ins.B].v))
		case bytecode.OpLen:
			result, err := v.len(ctx, registers[ins.B].v)
			if err != nil {
				return nil, err
			}
			registers[ins.A].v = result
		case bytecode.OpJump:
			pc = ins.B - 1
		case bytecode.OpJumpIfFalse:
			if !value.IsTruthy(registers[ins.A].v) {
				pc = ins.B - 1
			}
		case bytecode.OpJumpIfNil:
			if _, ok := registers[ins.A].v.(value.NilType); ok {
				pc = ins.B - 1
			}
		case bytecode.OpForPrep:
			index, limit, step, ok := numericForRegisters(registers, ins)
			if !ok {
				return nil, fmt.Errorf("bytecode: numeric for expects numbers")
			}
			if (step >= 0 && index > limit) || (step < 0 && index < limit) {
				pc = ins.D - 1
			}
		case bytecode.OpForLoop:
			index, limit, step, ok := numericForRegisters(registers, ins)
			if !ok {
				return nil, fmt.Errorf("bytecode: numeric for expects numbers")
			}
			index += step
			registers[ins.A].v = value.Number(index)
			if (step >= 0 && index <= limit) || (step < 0 && index >= limit) {
				pc = ins.D - 1
			}
		case bytecode.OpClosure:
			if ins.B < 0 || ins.B >= len(proto.Prototypes) {
				return nil, fmt.Errorf("bytecode: invalid prototype index")
			}
			childProto := proto.Prototypes[ins.B]
			captured := make([]*cell, 0, len(childProto.Upvalues))
			for _, upvalue := range childProto.Upvalues {
				if upvalue.InStack {
					captured = append(captured, registers[upvalue.Index])
					continue
				}
				if upvalue.Index < 0 || upvalue.Index >= len(upvalues) {
					return nil, fmt.Errorf("bytecode: invalid upvalue index")
				}
				captured = append(captured, upvalues[upvalue.Index])
			}
			registers[ins.A].v = &closure{proto: childProto, globals: v.globals, upvalues: captured}
		case bytecode.OpCall:
			callArgs := make([]value.Value, 0, ins.D)
			for i := 0; i < ins.D; i++ {
				callArgs = append(callArgs, registers[ins.C+i].v)
			}
			result, err := v.call(ctx, registers[ins.B].v, callArgs)
			if err != nil {
				return nil, err
			}
			registers[ins.A].v = first(result)
			registers[ins.A].resultCount = 1
		case bytecode.OpCallMulti:
			callArgs := make([]value.Value, 0, ins.D)
			for i := 0; i < ins.D; i++ {
				callArgs = append(callArgs, registers[ins.C+i].v)
			}
			results, err := v.call(ctx, registers[ins.B].v, callArgs)
			if err != nil {
				return nil, err
			}
			if len(results) == 0 {
				registers[ins.A].v = value.Nil
				registers[ins.A].resultCount = 1
				continue
			}
			for i, result := range results {
				for ins.A+i >= len(registers) {
					registers = append(registers, &cell{v: value.Nil})
				}
				if result == nil {
					result = value.Nil
				}
				registers[ins.A+i].v = result
				registers[ins.A+i].resultCount = 1
			}
			registers[ins.A].resultCount = len(results)
		case bytecode.OpNewTable:
			registers[ins.A].v = value.NewTable()
		case bytecode.OpGetTable:
			switch base := registers[ins.B].v.(type) {
			case *value.Table:
				registers[ins.A].v = base.Get(registers[ins.C].v)
			case value.String:
				if v.globals == nil {
					return nil, fmt.Errorf("bytecode: string library is not configured")
				}
				stringTable, ok := v.globals.Get("string").(*value.Table)
				if !ok {
					return nil, fmt.Errorf("bytecode: string library is not configured")
				}
				registers[ins.A].v = stringTable.Get(registers[ins.C].v)
			default:
				return nil, fmt.Errorf("bytecode: attempt to index %s", base.Type())
			}
		case bytecode.OpSetTable:
			table, ok := registers[ins.A].v.(*value.Table)
			if !ok {
				return nil, fmt.Errorf("bytecode: attempt to index %s", registers[ins.A].v.Type())
			}
			if err := v.setTable(ctx, table, registers[ins.B].v, registers[ins.C].v); err != nil {
				return nil, err
			}
		case bytecode.OpAppendTableMulti:
			table, ok := registers[ins.A].v.(*value.Table)
			if !ok {
				return nil, fmt.Errorf("bytecode: attempt to index %s", registers[ins.A].v.Type())
			}
			count := registers[ins.B].resultCount
			if count <= 0 {
				count = 1
			}
			for i := 0; i < count; i++ {
				table.Append(registers[ins.B+i].v)
			}
		case bytecode.OpVararg:
			if len(varargs) == 0 {
				registers[ins.A].v = value.Nil
			} else {
				registers[ins.A].v = varargs[0]
			}
		case bytecode.OpGetUpvalue:
			if ins.B < 0 || ins.B >= len(upvalues) {
				return nil, fmt.Errorf("bytecode: invalid upvalue index")
			}
			registers[ins.A].v = upvalues[ins.B].v
		case bytecode.OpSetUpvalue:
			if ins.A < 0 || ins.A >= len(upvalues) {
				return nil, fmt.Errorf("bytecode: invalid upvalue index")
			}
			upvalues[ins.A].v = registers[ins.B].v
		case bytecode.OpGenericForNext:
			results, err := v.call(ctx, registers[ins.B].v, []value.Value{registers[ins.B+1].v, registers[ins.B+2].v})
			if err != nil {
				return nil, err
			}
			for i := 0; i < ins.C; i++ {
				registers[ins.A+i].v = value.Nil
				if i < len(results) && results[i] != nil {
					registers[ins.A+i].v = results[i]
				}
			}
			registers[ins.B+2].v = registers[ins.A].v
		default:
			return nil, fmt.Errorf("bytecode: unsupported opcode %d", ins.Op)
		}
	}
	return []value.Value{value.Nil}, nil
}

func (v *VM) call(ctx context.Context, fn value.Value, args []value.Value) ([]value.Value, error) {
	if closureFn, ok := fn.(*closure); ok {
		child := &VM{globals: closureFn.globals, caller: v.caller}
		return child.execute(ctx, closureFn.proto, args, closureFn.upvalues)
	}
	if table, ok := fn.(*value.Table); ok && table.Metatable() != nil {
		call := table.Metatable().RawGet(value.String("__call"))
		if call != value.Nil {
			callArgs := make([]value.Value, 0, len(args)+1)
			callArgs = append(callArgs, table)
			callArgs = append(callArgs, args...)
			return v.call(ctx, call, callArgs)
		}
	}
	if v.caller != nil {
		return v.caller.Call(ctx, fn, args)
	}
	return nil, fmt.Errorf("bytecode: attempt to call %s", fn.Type())
}

func (v *VM) setTable(ctx context.Context, table *value.Table, key, val value.Value) error {
	if table.RawGet(key) != value.Nil {
		table.RawSet(key, val)
		return nil
	}
	if table.Metatable() != nil {
		newIndex := table.Metatable().RawGet(value.String("__newindex"))
		switch target := newIndex.(type) {
		case *value.Table:
			return v.setTable(ctx, target, key, val)
		case value.NilType:
		default:
			if target.Type() == value.TypeFunction {
				_, err := v.call(ctx, target, []value.Value{table, key, val})
				return err
			}
			return fmt.Errorf("bytecode: __newindex must be table or function")
		}
	}
	table.RawSet(key, val)
	return nil
}

func (v *VM) binaryArithmetic(ctx context.Context, op bytecode.Opcode, left, right value.Value) (value.Value, error) {
	ln, lok := value.ToNumber(left)
	rn, rok := value.ToNumber(right)
	if lok && rok {
		switch op {
		case bytecode.OpAdd:
			return value.Number(ln + rn), nil
		case bytecode.OpSub:
			return value.Number(ln - rn), nil
		case bytecode.OpMul:
			return value.Number(ln * rn), nil
		case bytecode.OpDiv:
			return value.Number(ln / rn), nil
		case bytecode.OpMod:
			return value.Number(math.Mod(ln, rn)), nil
		case bytecode.OpPow:
			return value.Number(math.Pow(ln, rn)), nil
		}
	}
	name := arithmeticMetamethodName(op)
	if name == "" {
		return value.Nil, fmt.Errorf("bytecode: unsupported arithmetic opcode %d", op)
	}
	if fn := binaryMetamethod(name, left, right); fn != value.Nil {
		results, err := v.call(ctx, fn, []value.Value{left, right})
		return first(results), err
	}
	return value.Nil, fmt.Errorf("bytecode: arithmetic on non-number")
}

func (v *VM) compare(ctx context.Context, op bytecode.Opcode, left, right value.Value) (bool, error) {
	switch op {
	case bytecode.OpEq, bytecode.OpNE:
		if result, ok, err := v.eqMetamethod(ctx, left, right); ok || err != nil {
			if err != nil {
				return false, err
			}
			if op == bytecode.OpNE {
				return !result, nil
			}
			return result, nil
		}
		result := value.Equal(left, right)
		if op == bytecode.OpNE {
			return !result, nil
		}
		return result, nil
	case bytecode.OpLT, bytecode.OpLE, bytecode.OpGT, bytecode.OpGE:
		name := "__lt"
		metaLeft, metaRight := left, right
		if op == bytecode.OpLE || op == bytecode.OpGE {
			name = "__le"
		}
		if op == bytecode.OpGT || op == bytecode.OpGE {
			metaLeft, metaRight = right, left
		}
		if fn := binaryMetamethod(name, metaLeft, metaRight); fn != value.Nil {
			results, err := v.call(ctx, fn, []value.Value{metaLeft, metaRight})
			return value.IsTruthy(first(results)), err
		}
		ln, lok := value.ToNumber(left)
		rn, rok := value.ToNumber(right)
		if lok && rok {
			switch op {
			case bytecode.OpLT:
				return ln < rn, nil
			case bytecode.OpLE:
				return ln <= rn, nil
			case bytecode.OpGT:
				return ln > rn, nil
			case bytecode.OpGE:
				return ln >= rn, nil
			}
		}
		switch op {
		case bytecode.OpLT:
			return left.String() < right.String(), nil
		case bytecode.OpLE:
			return left.String() <= right.String(), nil
		case bytecode.OpGT:
			return left.String() > right.String(), nil
		case bytecode.OpGE:
			return left.String() >= right.String(), nil
		}
	}
	return false, fmt.Errorf("bytecode: unsupported comparison opcode %d", op)
}

func (v *VM) concat(ctx context.Context, left, right value.Value) (value.Value, error) {
	if fn := binaryMetamethod("__concat", left, right); fn != value.Nil {
		results, err := v.call(ctx, fn, []value.Value{left, right})
		return first(results), err
	}
	return value.String(left.String() + right.String()), nil
}

func (v *VM) neg(ctx context.Context, x value.Value) (value.Value, error) {
	if n, ok := value.ToNumber(x); ok {
		return value.Number(-n), nil
	}
	if table, ok := x.(*value.Table); ok && table.Metatable() != nil {
		unm := table.Metatable().RawGet(value.String("__unm"))
		if unm != value.Nil {
			results, err := v.call(ctx, unm, []value.Value{table})
			return first(results), err
		}
	}
	return value.Nil, fmt.Errorf("bytecode: operand is not number")
}

func (v *VM) len(ctx context.Context, x value.Value) (value.Value, error) {
	if table, ok := x.(*value.Table); ok {
		if table.Metatable() != nil {
			lenFn := table.Metatable().RawGet(value.String("__len"))
			if lenFn != value.Nil {
				results, err := v.call(ctx, lenFn, []value.Value{table})
				return first(results), err
			}
		}
		return value.Number(table.Len()), nil
	}
	return value.Number(len(x.String())), nil
}

func (v *VM) eqMetamethod(ctx context.Context, left, right value.Value) (bool, bool, error) {
	leftTable, lok := left.(*value.Table)
	rightTable, rok := right.(*value.Table)
	if !lok || !rok || leftTable.Metatable() == nil || rightTable.Metatable() == nil {
		return false, false, nil
	}
	leftFn := leftTable.Metatable().RawGet(value.String("__eq"))
	rightFn := rightTable.Metatable().RawGet(value.String("__eq"))
	if leftFn == value.Nil || rightFn == value.Nil || leftFn != rightFn {
		return false, false, nil
	}
	results, err := v.call(ctx, leftFn, []value.Value{left, right})
	return value.IsTruthy(first(results)), true, err
}

func arithmeticMetamethodName(op bytecode.Opcode) string {
	switch op {
	case bytecode.OpAdd:
		return "__add"
	case bytecode.OpSub:
		return "__sub"
	case bytecode.OpMul:
		return "__mul"
	case bytecode.OpDiv:
		return "__div"
	case bytecode.OpMod:
		return "__mod"
	case bytecode.OpPow:
		return "__pow"
	default:
		return ""
	}
}

func binaryMetamethod(name string, values ...value.Value) value.Value {
	for _, v := range values {
		if table, ok := v.(*value.Table); ok && table.Metatable() != nil {
			fn := table.Metatable().RawGet(value.String(name))
			if fn != value.Nil {
				return fn
			}
		}
	}
	return value.Nil
}

func first(values []value.Value) value.Value {
	if len(values) == 0 {
		return value.Nil
	}
	if values[0] == nil {
		return value.Nil
	}
	return values[0]
}

func numericForRegisters(registers []*cell, ins bytecode.Instruction) (float64, float64, float64, bool) {
	index, ok := value.ToNumber(registers[ins.A].v)
	if !ok {
		return 0, 0, 0, false
	}
	limit, ok := value.ToNumber(registers[ins.B].v)
	if !ok {
		return 0, 0, 0, false
	}
	step, ok := value.ToNumber(registers[ins.C].v)
	if !ok {
		return 0, 0, 0, false
	}
	return index, limit, step, true
}
