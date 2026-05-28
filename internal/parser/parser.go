package parser

import (
	"fmt"

	"github.com/hiveton/higolua/internal/ast"
	"github.com/hiveton/higolua/internal/lexer"
)

type Parser struct {
	tokens []lexer.Token
	pos    int
}

func Parse(source, input string) (chunk *ast.Chunk, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("%v", r)
			}
		}
	}()
	tokens, err := lexer.New(source, input).All()
	if err != nil {
		return nil, err
	}
	p := &Parser{tokens: tokens}
	block, err := p.blockUntil(lexer.TokenEOF)
	if err != nil {
		return nil, err
	}
	return &ast.Chunk{Block: block}, nil
}

func (p *Parser) blockUntil(end ...lexer.TokenType) ([]ast.Stmt, error) {
	var out []ast.Stmt
	for !p.atAny(end...) {
		for p.match(";") {
		}
		if p.atAny(end...) {
			break
		}
		if p.at(lexer.TokenEOF) {
			if len(end) == 1 && end[0] == lexer.TokenEOF {
				break
			}
			return nil, p.err("unexpected end of file")
		}
		stmt, err := p.stmt()
		if err != nil {
			return nil, err
		}
		out = append(out, stmt)
		for p.match(";") {
		}
	}
	return out, nil
}

func (p *Parser) stmt() (ast.Stmt, error) {
	t := p.peek()
	switch t.Type {
	case lexer.TokenReturn:
		p.next()
		values, err := p.exprList()
		return &ast.ReturnStmt{Base: ast.Base{Pos: t.Pos}, Values: values}, err
	case lexer.TokenLocal:
		return p.localStmt()
	case lexer.TokenFunction:
		return p.functionStmt()
	case lexer.TokenDo:
		return p.doStmt()
	case lexer.TokenIf:
		return p.ifStmt()
	case lexer.TokenWhile:
		return p.whileStmt()
	case lexer.TokenRepeat:
		return p.repeatStmt()
	case lexer.TokenFor:
		return p.forStmt()
	case lexer.TokenBreak:
		p.next()
		return &ast.BreakStmt{Base: ast.Base{Pos: t.Pos}}, nil
	default:
		return p.assignOrCall()
	}
}

func (p *Parser) doStmt() (ast.Stmt, error) {
	t := p.expect(lexer.TokenDo)
	body, err := p.blockUntil(lexer.TokenEnd)
	if err != nil {
		return nil, err
	}
	p.expect(lexer.TokenEnd)
	return &ast.DoStmt{Base: ast.Base{Pos: t.Pos}, Body: body}, nil
}

func (p *Parser) localStmt() (ast.Stmt, error) {
	t := p.expect(lexer.TokenLocal)
	if p.match(lexer.TokenFunction) {
		name := p.expect(lexer.TokenIdent).Literal
		params, vararg, body, err := p.funcBody()
		if err != nil {
			return nil, err
		}
		return &ast.LocalAssignStmt{Base: ast.Base{Pos: t.Pos}, Names: []string{name}, Values: []ast.Expr{&ast.FunctionExpr{Base: ast.Base{Pos: t.Pos}, Params: params, Vararg: vararg, Body: body}}}, nil
	}
	var names []string
	names = append(names, p.expect(lexer.TokenIdent).Literal)
	for p.match(",") {
		names = append(names, p.expect(lexer.TokenIdent).Literal)
	}
	var values []ast.Expr
	var err error
	if p.match("=") {
		values, err = p.exprList()
	}
	return &ast.LocalAssignStmt{Base: ast.Base{Pos: t.Pos}, Names: names, Values: values}, err
}

func (p *Parser) functionStmt() (ast.Stmt, error) {
	t := p.expect(lexer.TokenFunction)
	name := p.expect(lexer.TokenIdent).Literal
	for p.match(".") {
		name += "." + p.expect(lexer.TokenIdent).Literal
	}
	method := false
	if p.match(":") {
		method = true
		name += "." + p.expect(lexer.TokenIdent).Literal
	}
	params, vararg, body, err := p.funcBody()
	if err != nil {
		return nil, err
	}
	if method {
		params = append([]string{"self"}, params...)
	}
	return &ast.FunctionStmt{Base: ast.Base{Pos: t.Pos}, Name: name, Params: params, Vararg: vararg, Body: body, Method: method}, nil
}

func (p *Parser) funcBody() ([]string, bool, []ast.Stmt, error) {
	p.expect("(")
	var params []string
	var vararg bool
	if !p.at(")") {
		if p.match(lexer.TokenDots) {
			vararg = true
		} else {
			params = append(params, p.expect(lexer.TokenIdent).Literal)
			for p.match(",") {
				if p.match(lexer.TokenDots) {
					vararg = true
					break
				}
				params = append(params, p.expect(lexer.TokenIdent).Literal)
			}
		}
	}
	p.expect(")")
	body, err := p.blockUntil(lexer.TokenEnd)
	if err != nil {
		return nil, false, nil, err
	}
	p.expect(lexer.TokenEnd)
	return params, vararg, body, nil
}

func (p *Parser) ifStmt() (ast.Stmt, error) {
	t := p.expect(lexer.TokenIf)
	cond, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	p.expect(lexer.TokenThen)
	thenBlock, err := p.blockUntil(lexer.TokenElse, lexer.TokenElseIf, lexer.TokenEnd)
	if err != nil {
		return nil, err
	}
	var elseBlock []ast.Stmt
	if p.match(lexer.TokenElseIf) {
		stmt, err := p.elseifStmt()
		if err != nil {
			return nil, err
		}
		elseBlock = []ast.Stmt{stmt}
	} else if p.match(lexer.TokenElse) {
		elseBlock, err = p.blockUntil(lexer.TokenEnd)
		if err != nil {
			return nil, err
		}
	}
	p.expect(lexer.TokenEnd)
	return &ast.IfStmt{Base: ast.Base{Pos: t.Pos}, Cond: cond, Then: thenBlock, Else: elseBlock}, nil
}

func (p *Parser) elseifStmt() (ast.Stmt, error) {
	t := p.tokens[p.pos-1]
	cond, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	p.expect(lexer.TokenThen)
	thenBlock, err := p.blockUntil(lexer.TokenElse, lexer.TokenElseIf, lexer.TokenEnd)
	if err != nil {
		return nil, err
	}
	var elseBlock []ast.Stmt
	if p.match(lexer.TokenElseIf) {
		stmt, err := p.elseifStmt()
		if err != nil {
			return nil, err
		}
		elseBlock = []ast.Stmt{stmt}
	} else if p.match(lexer.TokenElse) {
		elseBlock, err = p.blockUntil(lexer.TokenEnd)
		if err != nil {
			return nil, err
		}
	}
	return &ast.IfStmt{Base: ast.Base{Pos: t.Pos}, Cond: cond, Then: thenBlock, Else: elseBlock}, nil
}

func (p *Parser) whileStmt() (ast.Stmt, error) {
	t := p.expect(lexer.TokenWhile)
	cond, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	p.expect(lexer.TokenDo)
	body, err := p.blockUntil(lexer.TokenEnd)
	if err != nil {
		return nil, err
	}
	p.expect(lexer.TokenEnd)
	return &ast.WhileStmt{Base: ast.Base{Pos: t.Pos}, Cond: cond, Body: body}, nil
}

func (p *Parser) repeatStmt() (ast.Stmt, error) {
	t := p.expect(lexer.TokenRepeat)
	body, err := p.blockUntil(lexer.TokenUntil)
	if err != nil {
		return nil, err
	}
	p.expect(lexer.TokenUntil)
	cond, err := p.expr(0)
	return &ast.RepeatStmt{Base: ast.Base{Pos: t.Pos}, Body: body, Cond: cond}, err
}

func (p *Parser) forStmt() (ast.Stmt, error) {
	t := p.expect(lexer.TokenFor)
	name := p.expect(lexer.TokenIdent).Literal
	if p.at(",", lexer.TokenIn) {
		names := []string{name}
		if p.match(",") {
			for {
				names = append(names, p.expect(lexer.TokenIdent).Literal)
				if !p.match(",") {
					break
				}
			}
		}
		p.expect(lexer.TokenIn)
		exprs, err := p.exprList()
		if err != nil {
			return nil, err
		}
		p.expect(lexer.TokenDo)
		body, err := p.blockUntil(lexer.TokenEnd)
		if err != nil {
			return nil, err
		}
		p.expect(lexer.TokenEnd)
		return &ast.GenericForStmt{Base: ast.Base{Pos: t.Pos}, Names: names, Exprs: exprs, Body: body}, nil
	}
	p.expect("=")
	start, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	p.expect(",")
	end, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	var step ast.Expr = &ast.LiteralExpr{Base: ast.Base{Pos: t.Pos}, Kind: "number", Value: "1"}
	if p.match(",") {
		step, err = p.expr(0)
		if err != nil {
			return nil, err
		}
	}
	p.expect(lexer.TokenDo)
	body, err := p.blockUntil(lexer.TokenEnd)
	if err != nil {
		return nil, err
	}
	p.expect(lexer.TokenEnd)
	return &ast.ForStmt{Base: ast.Base{Pos: t.Pos}, Name: name, Start: start, End: end, Step: step, Body: body}, nil
}

func (p *Parser) assignOrCall() (ast.Stmt, error) {
	first, err := p.prefix()
	if err != nil {
		return nil, err
	}
	if _, ok := first.(*ast.CallExpr); ok && !p.at("=") && !p.at(",") {
		return &ast.CallStmt{Base: ast.Base{Pos: first.Position()}, Call: first}, nil
	}
	targets := []ast.Expr{first}
	for p.match(",") {
		x, err := p.prefix()
		if err != nil {
			return nil, err
		}
		targets = append(targets, x)
	}
	p.expect("=")
	values, err := p.exprList()
	return &ast.AssignStmt{Base: ast.Base{Pos: first.Position()}, Targets: targets, Values: values}, err
}

func (p *Parser) exprList() ([]ast.Expr, error) {
	if p.at(lexer.TokenEOF, lexer.TokenEnd, lexer.TokenElse, lexer.TokenUntil) {
		return nil, nil
	}
	x, err := p.expr(0)
	if err != nil {
		return nil, err
	}
	out := []ast.Expr{x}
	for p.match(",") {
		x, err := p.expr(0)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, nil
}

func (p *Parser) expr(min int) (ast.Expr, error) {
	left, err := p.unary()
	if err != nil {
		return nil, err
	}
	for {
		op := p.peek()
		prec, rightAssoc := precedence(op.Type)
		if prec < min {
			return left, nil
		}
		p.next()
		nextMin := prec + 1
		if rightAssoc {
			nextMin = prec
		}
		right, err := p.expr(nextMin)
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryExpr{Base: ast.Base{Pos: op.Pos}, Op: tokenText(op), Left: left, Right: right}
	}
}

func (p *Parser) unary() (ast.Expr, error) {
	if p.at("-", "#", lexer.TokenNot) {
		t := p.next()
		x, err := p.unary()
		return &ast.UnaryExpr{Base: ast.Base{Pos: t.Pos}, Op: tokenText(t), X: x}, err
	}
	return p.simple()
}

func (p *Parser) simple() (ast.Expr, error) {
	t := p.peek()
	switch t.Type {
	case lexer.TokenNumber:
		p.next()
		return &ast.LiteralExpr{Base: ast.Base{Pos: t.Pos}, Kind: "number", Value: t.Literal}, nil
	case lexer.TokenString:
		p.next()
		return &ast.LiteralExpr{Base: ast.Base{Pos: t.Pos}, Kind: "string", Value: t.Literal}, nil
	case lexer.TokenTrue, lexer.TokenFalse, lexer.TokenNil:
		p.next()
		return &ast.LiteralExpr{Base: ast.Base{Pos: t.Pos}, Kind: string(t.Type), Value: t.Literal}, nil
	case lexer.TokenDots:
		p.next()
		return &ast.VarargExpr{Base: ast.Base{Pos: t.Pos}}, nil
	case lexer.TokenFunction:
		p.next()
		params, vararg, body, err := p.funcBody()
		return &ast.FunctionExpr{Base: ast.Base{Pos: t.Pos}, Params: params, Vararg: vararg, Body: body}, err
	case "{":
		return p.table()
	default:
		return p.prefix()
	}
}

func (p *Parser) table() (ast.Expr, error) {
	t := p.expect("{")
	var fields []ast.TableField
	for !p.at("}") {
		if p.match("[") {
			key, err := p.expr(0)
			if err != nil {
				return nil, err
			}
			p.expect("]")
			p.expect("=")
			val, err := p.expr(0)
			if err != nil {
				return nil, err
			}
			fields = append(fields, ast.TableField{Key: key, Value: val})
		} else if p.at(lexer.TokenIdent) && p.peekN(1).Type == "=" {
			name := p.next()
			p.expect("=")
			val, err := p.expr(0)
			if err != nil {
				return nil, err
			}
			fields = append(fields, ast.TableField{Key: &ast.LiteralExpr{Base: ast.Base{Pos: name.Pos}, Kind: "string", Value: name.Literal}, Value: val})
		} else {
			val, err := p.expr(0)
			if err != nil {
				return nil, err
			}
			fields = append(fields, ast.TableField{Value: val})
		}
		if !p.match(",", ";") {
			break
		}
	}
	p.expect("}")
	return &ast.TableExpr{Base: ast.Base{Pos: t.Pos}, Fields: fields}, nil
}

func (p *Parser) prefix() (ast.Expr, error) {
	var x ast.Expr
	if p.match("(") {
		var err error
		x, err = p.expr(0)
		if err != nil {
			return nil, err
		}
		p.expect(")")
	} else {
		t := p.expect(lexer.TokenIdent)
		x = &ast.NameExpr{Base: ast.Base{Pos: t.Pos}, Name: t.Literal}
	}
	for {
		switch {
		case p.match("."):
			name := p.expect(lexer.TokenIdent)
			x = &ast.IndexExpr{Base: ast.Base{Pos: name.Pos}, X: x, Key: &ast.LiteralExpr{Base: ast.Base{Pos: name.Pos}, Kind: "string", Value: name.Literal}}
		case p.match("["):
			key, err := p.expr(0)
			if err != nil {
				return nil, err
			}
			p.expect("]")
			x = &ast.IndexExpr{Base: ast.Base{Pos: key.Position()}, X: x, Key: key}
		case p.match(":"):
			name := p.expect(lexer.TokenIdent)
			p.expect("(")
			args, err := p.args()
			if err != nil {
				return nil, err
			}
			fn := &ast.IndexExpr{Base: ast.Base{Pos: name.Pos}, X: x, Key: &ast.LiteralExpr{Base: ast.Base{Pos: name.Pos}, Kind: "string", Value: name.Literal}}
			args = append([]ast.Expr{x}, args...)
			x = &ast.CallExpr{Base: ast.Base{Pos: x.Position()}, Fn: fn, Args: args}
		case p.match("("):
			args, err := p.args()
			if err != nil {
				return nil, err
			}
			x = &ast.CallExpr{Base: ast.Base{Pos: x.Position()}, Fn: x, Args: args}
		case p.peek().Type == lexer.TokenString:
			t := p.next()
			arg := &ast.LiteralExpr{Base: ast.Base{Pos: t.Pos}, Kind: "string", Value: t.Literal}
			x = &ast.CallExpr{Base: ast.Base{Pos: x.Position()}, Fn: x, Args: []ast.Expr{arg}}
		case p.at("{"):
			arg, err := p.table()
			if err != nil {
				return nil, err
			}
			x = &ast.CallExpr{Base: ast.Base{Pos: x.Position()}, Fn: x, Args: []ast.Expr{arg}}
		default:
			return x, nil
		}
	}
}

func (p *Parser) args() ([]ast.Expr, error) {
	if p.match(")") {
		return nil, nil
	}
	args, err := p.exprList()
	if err != nil {
		return nil, err
	}
	p.expect(")")
	return args, nil
}

func precedence(t lexer.TokenType) (int, bool) {
	switch t {
	case lexer.TokenOr:
		return 1, false
	case lexer.TokenAnd:
		return 2, false
	case "<", ">", lexer.TokenLE, lexer.TokenGE, lexer.TokenEq, lexer.TokenNE:
		return 3, false
	case lexer.TokenDotDot:
		return 4, true
	case "+", "-":
		return 5, false
	case "*", "/", "%":
		return 6, false
	case "^":
		return 7, true
	}
	return -1, false
}

func (p *Parser) at(tt ...lexer.TokenType) bool {
	return p.atAny(tt...)
}

func (p *Parser) atAny(tt ...lexer.TokenType) bool {
	for _, typ := range tt {
		if p.peek().Type == typ {
			return true
		}
	}
	return false
}

func (p *Parser) match(tt ...lexer.TokenType) bool {
	if p.at(tt...) {
		p.next()
		return true
	}
	return false
}

func (p *Parser) expect(tt lexer.TokenType) lexer.Token {
	if !p.at(tt) {
		panic(p.err(fmt.Sprintf("expected %s, got %s", tt, p.peek().Type)))
	}
	return p.next()
}

func (p *Parser) next() lexer.Token {
	t := p.peek()
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
	return t
}

func (p *Parser) peek() lexer.Token { return p.peekN(0) }
func (p *Parser) peekN(n int) lexer.Token {
	i := p.pos + n
	if i >= len(p.tokens) {
		return p.tokens[len(p.tokens)-1]
	}
	return p.tokens[i]
}

func (p *Parser) err(msg string) error {
	pos := p.peek().Pos
	return fmt.Errorf("%s:%d:%d: %s", pos.Source, pos.Line, pos.Column, msg)
}

func tokenText(t lexer.Token) string {
	if t.Literal != "" {
		return t.Literal
	}
	return string(t.Type)
}
