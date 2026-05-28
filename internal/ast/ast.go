package ast

import "github.com/hiveton/higolua/internal/lexer"

type Chunk struct {
	Block []Stmt
}

type Node interface {
	Position() lexer.Position
}

type Stmt interface {
	Node
	stmt()
}

type Expr interface {
	Node
	expr()
}

type Base struct {
	Pos lexer.Position
}

func (b Base) Position() lexer.Position { return b.Pos }

type ReturnStmt struct {
	Base
	Values []Expr
}

func (*ReturnStmt) stmt() {}

type LocalAssignStmt struct {
	Base
	Names  []string
	Values []Expr
}

func (*LocalAssignStmt) stmt() {}

type AssignStmt struct {
	Base
	Targets []Expr
	Values  []Expr
}

func (*AssignStmt) stmt() {}

type CallStmt struct {
	Base
	Call Expr
}

func (*CallStmt) stmt() {}

type DoStmt struct {
	Base
	Body []Stmt
}

func (*DoStmt) stmt() {}

type FunctionStmt struct {
	Base
	Name   string
	Params []string
	Vararg bool
	Body   []Stmt
	Method bool
}

func (*FunctionStmt) stmt() {}

type IfStmt struct {
	Base
	Cond Expr
	Then []Stmt
	Else []Stmt
}

func (*IfStmt) stmt() {}

type WhileStmt struct {
	Base
	Cond Expr
	Body []Stmt
}

func (*WhileStmt) stmt() {}

type RepeatStmt struct {
	Base
	Body []Stmt
	Cond Expr
}

func (*RepeatStmt) stmt() {}

type ForStmt struct {
	Base
	Name  string
	Start Expr
	End   Expr
	Step  Expr
	Body  []Stmt
}

func (*ForStmt) stmt() {}

type GenericForStmt struct {
	Base
	Names []string
	Exprs []Expr
	Body  []Stmt
}

func (*GenericForStmt) stmt() {}

type BreakStmt struct{ Base }

func (*BreakStmt) stmt() {}

type LiteralExpr struct {
	Base
	Kind  string
	Value string
}

func (*LiteralExpr) expr() {}

type NameExpr struct {
	Base
	Name string
}

func (*NameExpr) expr() {}

type VarargExpr struct {
	Base
}

func (*VarargExpr) expr() {}

type UnaryExpr struct {
	Base
	Op string
	X  Expr
}

func (*UnaryExpr) expr() {}

type BinaryExpr struct {
	Base
	Op          string
	Left, Right Expr
}

func (*BinaryExpr) expr() {}

type IndexExpr struct {
	Base
	X   Expr
	Key Expr
}

func (*IndexExpr) expr() {}

type CallExpr struct {
	Base
	Fn   Expr
	Args []Expr
}

func (*CallExpr) expr() {}

type FunctionExpr struct {
	Base
	Params []string
	Vararg bool
	Body   []Stmt
}

func (*FunctionExpr) expr() {}

type TableField struct {
	Key   Expr
	Value Expr
}

type TableExpr struct {
	Base
	Fields []TableField
}

func (*TableExpr) expr() {}
