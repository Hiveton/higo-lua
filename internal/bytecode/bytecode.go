package bytecode

import (
	"fmt"
	"strings"

	"github.com/Hiveton/higo-lua/internal/ast"
	"github.com/Hiveton/higo-lua/internal/lexer"
	"github.com/Hiveton/higo-lua/value"
)

type Opcode byte

const (
	OpMove Opcode = iota
	OpLoadConst
	OpGetGlobal
	OpSetGlobal
	OpCall
	OpReturn
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpPow
	OpConcat
	OpEq
	OpNE
	OpLT
	OpLE
	OpGT
	OpGE
	OpNeg
	OpNot
	OpLen
	OpJump
	OpJumpIfFalse
	OpForPrep
	OpForLoop
	OpClosure
	OpNewTable
	OpGetTable
	OpSetTable
	OpVararg
	OpGetUpvalue
	OpSetUpvalue
	OpGenericForNext
	OpJumpIfNil
	OpCallMulti
	OpAppendTableMulti
)

type Instruction struct {
	Op Opcode
	A  int
	B  int
	C  int
	D  int
}

type Prototype struct {
	Source       string
	Constants    []value.Value
	Prototypes   []*Prototype
	Upvalues     []Upvalue
	Instructions []Instruction
	Registers    int
	Params       []string
	Vararg       bool
	ArgRegister  int
}

type Upvalue struct {
	Name    string
	Index   int
	InStack bool
}

func Compile(chunk *ast.Chunk) (*Prototype, error) {
	return compile(chunk, nil, nil)
}

func CompileWithHostCalls(chunk *ast.Chunk, hostCalls []string) (*Prototype, error) {
	return compile(chunk, hostCalls, nil)
}

func CompileWithHostCallsAndTables(chunk *ast.Chunk, hostCalls []string, hostTables []string) (*Prototype, error) {
	return compile(chunk, hostCalls, hostTables)
}

func compile(chunk *ast.Chunk, hostCalls []string, hostTables []string) (*Prototype, error) {
	c := &compiler{proto: &Prototype{}, scopes: []map[string]int{{}}, tableScopes: []map[string]bool{{}}, stringScopes: []map[string]bool{{}}, globalFuncs: map[string]bool{}, globalTables: map[string]bool{}, hostCalls: map[string]bool{}}
	for _, name := range hostCalls {
		c.hostCalls[name] = true
	}
	for _, name := range hostTables {
		c.globalTables[name] = true
	}
	if err := c.block(chunk.Block); err != nil {
		return nil, err
	}
	c.proto.Registers = c.nextReg
	return c.proto, nil
}

type compiler struct {
	proto         *Prototype
	nextReg       int
	scopes        []map[string]int
	tableScopes   []map[string]bool
	stringScopes  []map[string]bool
	loops         []loopContext
	globalFuncs   map[string]bool
	globalTables  map[string]bool
	hostCalls     map[string]bool
	parent        *compiler
	upvalueByName map[string]int
}

type loopContext struct {
	breaks []int
}

func (c *compiler) block(stmts []ast.Stmt) error {
	for _, stmt := range stmts {
		if err := c.stmt(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) stmt(stmt ast.Stmt) error {
	switch st := stmt.(type) {
	case *ast.LocalAssignStmt:
		localRegs := make([]int, len(st.Names))
		for i := range st.Names {
			localRegs[i] = c.reg()
		}
		for i, valueExpr := range st.Values {
			if i >= len(localRegs) {
				break
			}
			if i == len(st.Values)-1 {
				if call, ok := valueExpr.(*ast.CallExpr); ok {
					if err := c.callMulti(localRegs[i], call); err != nil {
						return err
					}
					for j := i + 1; j < len(localRegs); j++ {
						c.emit(OpMove, localRegs[j], localRegs[j-1]+1, 0)
					}
					for j, name := range st.Names {
						c.bindLocal(name, localRegs[j])
						c.setLocalKinds(name, false, false)
					}
					return nil
				}
			}
			valueReg, err := c.expr(st.Values[i])
			if err != nil {
				return err
			}
			c.emit(OpMove, localRegs[i], valueReg, 0)
		}
		for i := len(st.Values); i < len(localRegs); i++ {
			nilReg := c.constant(value.Nil)
			c.emit(OpMove, localRegs[i], nilReg, 0)
		}
		for i, name := range st.Names {
			c.bindLocal(name, localRegs[i])
			c.setLocalKinds(name, i < len(st.Values) && isTableLiteral(st.Values[i]), i < len(st.Values) && isStringLiteral(st.Values[i]))
		}
	case *ast.FunctionStmt:
		valueReg, err := c.function(st.Params, st.Vararg, st.Body)
		if err != nil {
			return err
		}
		if err := c.assignFunction(st.Name, valueReg); err != nil {
			return err
		}
	case *ast.CallStmt:
		_, err := c.expr(st.Call)
		return err
	case *ast.DoStmt:
		return c.scopedBlock(st.Body)
	case *ast.AssignStmt:
		valueRegs, err := c.assignmentValueRegs(len(st.Targets), st.Values)
		if err != nil {
			return err
		}
		for i, target := range st.Targets {
			valueReg := valueRegs[i]
			switch target := target.(type) {
			case *ast.NameExpr:
				reg, ok := c.lookupLocal(target.Name)
				if ok {
					c.emit(OpMove, reg, valueReg, 0)
					c.setLocalKinds(target.Name, i < len(st.Values) && isTableLiteral(st.Values[i]), i < len(st.Values) && isStringLiteral(st.Values[i]))
					continue
				}
				if upvalue, ok := c.resolveUpvalue(target.Name); ok {
					c.emit(OpSetUpvalue, upvalue, valueReg, 0)
					continue
				}
				c.emit(OpSetGlobal, c.name(target.Name), valueReg, 0)
				c.globalTables[target.Name] = i < len(st.Values) && isTableLiteral(st.Values[i])
			case *ast.IndexExpr:
				tableReg, err := c.expr(target.X)
				if err != nil {
					return err
				}
				keyReg, err := c.expr(target.Key)
				if err != nil {
					return err
				}
				c.emit(OpSetTable, tableReg, keyReg, valueReg)
			default:
				return fmt.Errorf("bytecode: unsupported assignment target %T", target)
			}
		}
	case *ast.IfStmt:
		cond, err := c.expr(st.Cond)
		if err != nil {
			return err
		}
		jumpToElse := c.emit(OpJumpIfFalse, cond, 0, 0)
		if err := c.scopedBlock(st.Then); err != nil {
			return err
		}
		jumpToEnd := c.emit(OpJump, 0, 0, 0)
		c.patch(jumpToElse, len(c.proto.Instructions))
		if err := c.scopedBlock(st.Else); err != nil {
			return err
		}
		c.patch(jumpToEnd, len(c.proto.Instructions))
	case *ast.WhileStmt:
		loopStart := len(c.proto.Instructions)
		cond, err := c.expr(st.Cond)
		if err != nil {
			return err
		}
		jumpToEnd := c.emit(OpJumpIfFalse, cond, 0, 0)
		c.pushLoop()
		if err := c.scopedBlock(st.Body); err != nil {
			c.popLoop(0)
			return err
		}
		c.emit(OpJump, 0, loopStart, 0)
		c.patch(jumpToEnd, len(c.proto.Instructions))
		c.popLoop(len(c.proto.Instructions))
	case *ast.RepeatStmt:
		loopStart := len(c.proto.Instructions)
		c.pushLoop()
		c.scopes = append(c.scopes, map[string]int{})
		if err := c.block(st.Body); err != nil {
			c.scopes = c.scopes[:len(c.scopes)-1]
			c.popLoop(0)
			return err
		}
		cond, err := c.expr(st.Cond)
		if err != nil {
			c.scopes = c.scopes[:len(c.scopes)-1]
			c.popLoop(0)
			return err
		}
		c.emit(OpJumpIfFalse, cond, loopStart, 0)
		c.scopes = c.scopes[:len(c.scopes)-1]
		c.popLoop(len(c.proto.Instructions))
	case *ast.ForStmt:
		start, err := c.expr(st.Start)
		if err != nil {
			return err
		}
		limit, err := c.expr(st.End)
		if err != nil {
			return err
		}
		var step int
		if st.Step == nil {
			step = c.constant(value.Number(1))
		} else {
			step, err = c.expr(st.Step)
			if err != nil {
				return err
			}
		}
		c.scopes = append(c.scopes, map[string]int{})
		index := c.defineLocal(st.Name)
		c.emit(OpMove, index, start, 0)
		jumpToEnd := c.emit(OpForPrep, index, limit, step)
		c.pushLoop()
		if err := c.block(st.Body); err != nil {
			c.scopes = c.scopes[:len(c.scopes)-1]
			c.popLoop(0)
			return err
		}
		loop := c.emit(OpForLoop, index, limit, step)
		c.patchD(loop, jumpToEnd+1)
		c.patchD(jumpToEnd, len(c.proto.Instructions))
		c.popLoop(len(c.proto.Instructions))
		c.scopes = c.scopes[:len(c.scopes)-1]
	case *ast.GenericForStmt:
		if len(st.Names) == 0 {
			return fmt.Errorf("bytecode: generic for requires at least one name")
		}
		iterBase := c.reg()
		c.reg()
		c.reg()
		iterRegs, err := c.assignmentValueRegs(3, st.Exprs)
		if err != nil {
			return err
		}
		for i := 0; i < 3; i++ {
			c.emit(OpMove, iterBase+i, iterRegs[i], 0)
		}
		c.scopes = append(c.scopes, map[string]int{})
		varStart := c.defineLocal(st.Names[0])
		for _, name := range st.Names[1:] {
			c.defineLocal(name)
		}
		loopStart := len(c.proto.Instructions)
		c.emit(OpGenericForNext, varStart, iterBase, len(st.Names))
		jumpToEnd := c.emit(OpJumpIfNil, varStart, 0, 0)
		c.pushLoop()
		if err := c.block(st.Body); err != nil {
			c.scopes = c.scopes[:len(c.scopes)-1]
			c.popLoop(0)
			return err
		}
		c.emit(OpJump, 0, loopStart, 0)
		c.patch(jumpToEnd, len(c.proto.Instructions))
		c.popLoop(len(c.proto.Instructions))
		c.scopes = c.scopes[:len(c.scopes)-1]
	case *ast.BreakStmt:
		if len(c.loops) == 0 {
			return fmt.Errorf("bytecode: break outside loop")
		}
		jump := c.emit(OpJump, 0, 0, 0)
		c.loops[len(c.loops)-1].breaks = append(c.loops[len(c.loops)-1].breaks, jump)
	case *ast.ReturnStmt:
		if len(st.Values) == 0 {
			c.emit(OpReturn, -1, 0, 0)
			return nil
		}
		retStart := c.reg()
		for i, expr := range st.Values {
			if i > 0 {
				c.reg()
			}
			if i == len(st.Values)-1 {
				if call, ok := expr.(*ast.CallExpr); ok {
					callOut := retStart + i
					if err := c.callMulti(callOut, call); err != nil {
						return err
					}
					c.emit(OpReturn, retStart, callOut, -1)
					return nil
				}
			}
			reg, err := c.expr(expr)
			if err != nil {
				return err
			}
			c.emit(OpMove, retStart+i, reg, 0)
		}
		c.emit(OpReturn, retStart, 0, len(st.Values))
	default:
		return fmt.Errorf("bytecode: unsupported statement %T", stmt)
	}
	return nil
}

func (c *compiler) expr(expr ast.Expr) (int, error) {
	switch ex := expr.(type) {
	case *ast.LiteralExpr:
		switch ex.Kind {
		case "number":
			n, err := lexer.ParseNumber(ex.Value)
			if err != nil {
				return 0, err
			}
			return c.constant(value.Number(n)), nil
		case "string":
			return c.constant(value.String(ex.Value)), nil
		case "true":
			return c.constant(value.Bool(true)), nil
		case "false":
			return c.constant(value.Bool(false)), nil
		default:
			return c.constant(value.Nil), nil
		}
	case *ast.NameExpr:
		reg, ok := c.lookupLocal(ex.Name)
		if ok {
			return reg, nil
		}
		if upvalue, ok := c.resolveUpvalue(ex.Name); ok {
			out := c.reg()
			c.emit(OpGetUpvalue, out, upvalue, 0)
			return out, nil
		}
		out := c.reg()
		c.emit(OpGetGlobal, out, c.name(ex.Name), 0)
		return out, nil
	case *ast.BinaryExpr:
		if ex.Op == "and" || ex.Op == "or" {
			return c.logical(ex)
		}
		left, err := c.expr(ex.Left)
		if err != nil {
			return 0, err
		}
		right, err := c.expr(ex.Right)
		if err != nil {
			return 0, err
		}
		out := c.reg()
		switch ex.Op {
		case "+":
			c.emit(OpAdd, out, left, right)
		case "-":
			c.emit(OpSub, out, left, right)
		case "*":
			c.emit(OpMul, out, left, right)
		case "/":
			c.emit(OpDiv, out, left, right)
		case "%":
			c.emit(OpMod, out, left, right)
		case "^":
			c.emit(OpPow, out, left, right)
		case "..":
			c.emit(OpConcat, out, left, right)
		case "==":
			c.emit(OpEq, out, left, right)
		case "~=":
			c.emit(OpNE, out, left, right)
		case "<":
			c.emit(OpLT, out, left, right)
		case "<=":
			c.emit(OpLE, out, left, right)
		case ">":
			c.emit(OpGT, out, left, right)
		case ">=":
			c.emit(OpGE, out, left, right)
		default:
			return 0, fmt.Errorf("bytecode: unsupported operator %s", ex.Op)
		}
		return out, nil
	case *ast.UnaryExpr:
		x, err := c.expr(ex.X)
		if err != nil {
			return 0, err
		}
		out := c.reg()
		switch ex.Op {
		case "-":
			c.emit(OpNeg, out, x, 0)
		case "not":
			c.emit(OpNot, out, x, 0)
		case "#":
			c.emit(OpLen, out, x, 0)
		default:
			return 0, fmt.Errorf("bytecode: unsupported unary operator %s", ex.Op)
		}
		return out, nil
	case *ast.FunctionExpr:
		return c.function(ex.Params, ex.Vararg, ex.Body)
	case *ast.CallExpr:
		if err := c.supportedCall(ex.Fn); err != nil {
			return 0, err
		}
		fn, err := c.expr(ex.Fn)
		if err != nil {
			return 0, err
		}
		argRegs := make([]int, 0, len(ex.Args))
		for _, arg := range ex.Args {
			argReg, err := c.expr(arg)
			if err != nil {
				return 0, err
			}
			argRegs = append(argRegs, argReg)
		}
		argStart := c.reg()
		for i, argReg := range argRegs {
			if i > 0 {
				c.reg()
			}
			c.emit(OpMove, argStart+i, argReg, 0)
		}
		out := c.reg()
		c.emitFull(OpCall, out, fn, argStart, len(argRegs))
		return out, nil
	case *ast.IndexExpr:
		tableReg, err := c.expr(ex.X)
		if err != nil {
			return 0, err
		}
		keyReg, err := c.expr(ex.Key)
		if err != nil {
			return 0, err
		}
		out := c.reg()
		c.emit(OpGetTable, out, tableReg, keyReg)
		return out, nil
	case *ast.TableExpr:
		out := c.reg()
		c.emit(OpNewTable, out, 0, 0)
		arrayIndex := 1
		for i, field := range ex.Fields {
			if field.Key == nil && i == len(ex.Fields)-1 {
				if call, ok := field.Value.(*ast.CallExpr); ok {
					callOut := c.reg()
					if err := c.callMulti(callOut, call); err != nil {
						return 0, err
					}
					c.emit(OpAppendTableMulti, out, callOut, 0)
					return out, nil
				}
			}
			keyReg := 0
			var err error
			if field.Key == nil {
				keyReg = c.constant(value.Number(arrayIndex))
				arrayIndex++
			} else {
				keyReg, err = c.expr(field.Key)
				if err != nil {
					return 0, err
				}
			}
			valueReg, err := c.expr(field.Value)
			if err != nil {
				return 0, err
			}
			c.emit(OpSetTable, out, keyReg, valueReg)
		}
		return out, nil
	case *ast.VarargExpr:
		if !c.proto.Vararg {
			return 0, fmt.Errorf("bytecode: cannot use ... outside a vararg function")
		}
		out := c.reg()
		c.emit(OpVararg, out, 0, 0)
		return out, nil
	default:
		return 0, fmt.Errorf("bytecode: unsupported expression %T", expr)
	}
}

func (c *compiler) assignmentValueRegs(targets int, values []ast.Expr) ([]int, error) {
	regs := make([]int, targets)
	for i := 0; i < targets; i++ {
		regs[i] = c.reg()
	}
	for i, valueExpr := range values {
		if i >= len(regs) {
			break
		}
		if i == len(values)-1 {
			if call, ok := valueExpr.(*ast.CallExpr); ok {
				if err := c.callMulti(regs[i], call); err != nil {
					return nil, err
				}
				for j := i + 1; j < len(regs); j++ {
					c.emit(OpMove, regs[j], regs[j-1]+1, 0)
				}
				return regs, nil
			}
		}
		valueReg, err := c.expr(valueExpr)
		if err != nil {
			return nil, err
		}
		c.emit(OpMove, regs[i], valueReg, 0)
	}
	for i := len(values); i < len(regs); i++ {
		nilReg := c.constant(value.Nil)
		c.emit(OpMove, regs[i], nilReg, 0)
	}
	return regs, nil
}

func (c *compiler) logical(ex *ast.BinaryExpr) (int, error) {
	left, err := c.expr(ex.Left)
	if err != nil {
		return 0, err
	}
	out := c.reg()
	c.emit(OpMove, out, left, 0)
	switch ex.Op {
	case "and":
		jumpToEnd := c.emit(OpJumpIfFalse, left, 0, 0)
		right, err := c.expr(ex.Right)
		if err != nil {
			return 0, err
		}
		c.emit(OpMove, out, right, 0)
		c.patch(jumpToEnd, len(c.proto.Instructions))
	case "or":
		jumpToRight := c.emit(OpJumpIfFalse, left, 0, 0)
		jumpToEnd := c.emit(OpJump, 0, 0, 0)
		c.patch(jumpToRight, len(c.proto.Instructions))
		right, err := c.expr(ex.Right)
		if err != nil {
			return 0, err
		}
		c.emit(OpMove, out, right, 0)
		c.patch(jumpToEnd, len(c.proto.Instructions))
	default:
		return 0, fmt.Errorf("bytecode: unsupported logical operator %s", ex.Op)
	}
	return out, nil
}

func (c *compiler) callMulti(out int, ex *ast.CallExpr) error {
	if err := c.supportedCall(ex.Fn); err != nil {
		return err
	}
	fn, err := c.expr(ex.Fn)
	if err != nil {
		return err
	}
	argRegs := make([]int, 0, len(ex.Args))
	for _, arg := range ex.Args {
		argReg, err := c.expr(arg)
		if err != nil {
			return err
		}
		argRegs = append(argRegs, argReg)
	}
	argStart := c.reg()
	for i, argReg := range argRegs {
		if i > 0 {
			c.reg()
		}
		c.emit(OpMove, argStart+i, argReg, 0)
	}
	c.emitFull(OpCallMulti, out, fn, argStart, len(argRegs))
	return nil
}

func (c *compiler) function(params []string, vararg bool, body []ast.Stmt) (int, error) {
	child := &compiler{proto: &Prototype{Params: append([]string(nil), params...), Vararg: vararg, ArgRegister: -1}, scopes: []map[string]int{{}}, tableScopes: []map[string]bool{{}}, stringScopes: []map[string]bool{{}}, globalFuncs: c.globalFuncs, globalTables: c.globalTables, hostCalls: c.hostCalls, parent: c, upvalueByName: map[string]int{}}
	for _, param := range params {
		child.defineLocal(param)
	}
	if vararg {
		child.proto.ArgRegister = child.defineLocal("arg")
	}
	if err := child.block(body); err != nil {
		return 0, err
	}
	child.emit(OpReturn, -1, 0, 0)
	child.proto.Registers = child.nextReg
	protoIndex := len(c.proto.Prototypes)
	c.proto.Prototypes = append(c.proto.Prototypes, child.proto)
	out := c.reg()
	c.emit(OpClosure, out, protoIndex, 0)
	return out, nil
}

func (c *compiler) supportedCall(fn ast.Expr) error {
	switch f := fn.(type) {
	case *ast.NameExpr:
		if _, ok := c.lookupLocal(f.Name); ok {
			return nil
		}
		if c.globalFuncs[f.Name] {
			return nil
		}
		if c.hostCalls[f.Name] {
			return nil
		}
		return fmt.Errorf("bytecode: dynamic or Go function calls are not supported")
	case *ast.FunctionExpr:
		return nil
	case *ast.IndexExpr:
		if c.isKnownTablePath(f.X) || c.isKnownString(f.X) {
			return nil
		}
		return fmt.Errorf("bytecode: dynamic or Go function calls are not supported")
	default:
		return fmt.Errorf("bytecode: dynamic or Go function calls are not supported")
	}
}

func (c *compiler) assignFunction(name string, valueReg int) error {
	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		c.globalFuncs[name] = true
		if reg, ok := c.lookupLocal(name); ok {
			c.emit(OpMove, reg, valueReg, 0)
			c.setLocalKinds(name, false, false)
		} else {
			c.emit(OpSetGlobal, c.name(name), valueReg, 0)
			c.globalTables[name] = false
		}
		return nil
	}
	tableReg, err := c.namePath(parts[:len(parts)-1])
	if err != nil {
		return err
	}
	keyReg := c.constant(value.String(parts[len(parts)-1]))
	c.emit(OpSetTable, tableReg, keyReg, valueReg)
	return nil
}

func (c *compiler) namePath(parts []string) (int, error) {
	if len(parts) == 0 {
		return 0, fmt.Errorf("bytecode: empty name path")
	}
	var out int
	if reg, ok := c.lookupLocal(parts[0]); ok {
		out = c.reg()
		c.emit(OpMove, out, reg, 0)
	} else {
		out = c.reg()
		c.emit(OpGetGlobal, out, c.name(parts[0]), 0)
	}
	for _, part := range parts[1:] {
		keyReg := c.constant(value.String(part))
		next := c.reg()
		c.emit(OpGetTable, next, out, keyReg)
		out = next
	}
	return out, nil
}

func (c *compiler) resolveUpvalue(name string) (int, bool) {
	if c.parent == nil {
		return 0, false
	}
	if existing, ok := c.upvalueByName[name]; ok {
		return existing, true
	}
	if reg, ok := c.parent.lookupLocal(name); ok {
		return c.addUpvalue(name, reg, true), true
	}
	if upvalue, ok := c.parent.resolveUpvalue(name); ok {
		return c.addUpvalue(name, upvalue, false), true
	}
	return 0, false
}

func (c *compiler) addUpvalue(name string, index int, inStack bool) int {
	idx := len(c.proto.Upvalues)
	c.proto.Upvalues = append(c.proto.Upvalues, Upvalue{Name: name, Index: index, InStack: inStack})
	c.upvalueByName[name] = idx
	return idx
}

func (c *compiler) constant(v value.Value) int {
	idx := c.literal(v)
	reg := c.reg()
	c.emit(OpLoadConst, reg, idx, 0)
	return reg
}

func (c *compiler) literal(v value.Value) int {
	idx := len(c.proto.Constants)
	c.proto.Constants = append(c.proto.Constants, v)
	return idx
}

func (c *compiler) reg() int {
	reg := c.nextReg
	c.nextReg++
	return reg
}

func (c *compiler) scopedBlock(stmts []ast.Stmt) error {
	c.scopes = append(c.scopes, map[string]int{})
	c.tableScopes = append(c.tableScopes, map[string]bool{})
	c.stringScopes = append(c.stringScopes, map[string]bool{})
	defer func() {
		c.scopes = c.scopes[:len(c.scopes)-1]
		c.tableScopes = c.tableScopes[:len(c.tableScopes)-1]
		c.stringScopes = c.stringScopes[:len(c.stringScopes)-1]
	}()
	return c.block(stmts)
}

func (c *compiler) defineLocal(name string) int {
	reg := c.reg()
	c.bindLocal(name, reg)
	return reg
}

func (c *compiler) bindLocal(name string, reg int) {
	c.scopes[len(c.scopes)-1][name] = reg
	c.tableScopes[len(c.tableScopes)-1][name] = false
	c.stringScopes[len(c.stringScopes)-1][name] = false
}

func (c *compiler) lookupLocal(name string) (int, bool) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if reg, ok := c.scopes[i][name]; ok {
			return reg, true
		}
	}
	return 0, false
}

func (c *compiler) setLocalKinds(name string, isTable bool, isString bool) {
	for i := len(c.tableScopes) - 1; i >= 0; i-- {
		if _, ok := c.scopes[i][name]; ok {
			c.tableScopes[i][name] = isTable
			c.stringScopes[i][name] = isString
			return
		}
	}
}

func (c *compiler) lookupLocalTable(name string) bool {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if _, ok := c.scopes[i][name]; ok {
			return c.tableScopes[i][name]
		}
	}
	return false
}

func (c *compiler) isKnownTablePath(expr ast.Expr) bool {
	switch ex := expr.(type) {
	case *ast.NameExpr:
		if _, ok := c.lookupLocal(ex.Name); ok {
			return c.lookupLocalTable(ex.Name)
		}
		return c.globalTables[ex.Name]
	case *ast.IndexExpr:
		if _, ok := ex.Key.(*ast.LiteralExpr); !ok {
			return false
		}
		return c.isKnownTablePath(ex.X)
	default:
		return false
	}
}

func (c *compiler) isKnownString(expr ast.Expr) bool {
	switch ex := expr.(type) {
	case *ast.LiteralExpr:
		return ex.Kind == "string"
	case *ast.NameExpr:
		for i := len(c.scopes) - 1; i >= 0; i-- {
			if _, ok := c.scopes[i][ex.Name]; ok {
				return c.stringScopes[i][ex.Name]
			}
		}
	}
	return false
}

func isTableLiteral(expr ast.Expr) bool {
	_, ok := expr.(*ast.TableExpr)
	return ok
}

func isStringLiteral(expr ast.Expr) bool {
	lit, ok := expr.(*ast.LiteralExpr)
	return ok && lit.Kind == "string"
}

func (c *compiler) pushLoop() {
	c.loops = append(c.loops, loopContext{})
}

func (c *compiler) popLoop(target int) {
	if len(c.loops) == 0 {
		return
	}
	loop := c.loops[len(c.loops)-1]
	c.loops = c.loops[:len(c.loops)-1]
	for _, jump := range loop.breaks {
		c.patch(jump, target)
	}
}

func (c *compiler) name(name string) int {
	return c.literal(value.String(name))
}

func (c *compiler) emit(op Opcode, a, b, d int) int {
	c.proto.Instructions = append(c.proto.Instructions, Instruction{Op: op, A: a, B: b, C: d})
	return len(c.proto.Instructions) - 1
}

func (c *compiler) emitFull(op Opcode, a, b, cc, d int) int {
	c.proto.Instructions = append(c.proto.Instructions, Instruction{Op: op, A: a, B: b, C: cc, D: d})
	return len(c.proto.Instructions) - 1
}

func (c *compiler) patch(index int, target int) {
	c.proto.Instructions[index].B = target
}

func (c *compiler) patchD(index int, target int) {
	c.proto.Instructions[index].D = target
}
