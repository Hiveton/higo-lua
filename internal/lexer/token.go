package lexer

type TokenType string

const (
	TokenEOF      TokenType = "eof"
	TokenIdent    TokenType = "ident"
	TokenNumber   TokenType = "number"
	TokenString   TokenType = "string"
	TokenLocal    TokenType = "local"
	TokenFunction TokenType = "function"
	TokenReturn   TokenType = "return"
	TokenEnd      TokenType = "end"
	TokenIf       TokenType = "if"
	TokenThen     TokenType = "then"
	TokenElse     TokenType = "else"
	TokenElseIf   TokenType = "elseif"
	TokenWhile    TokenType = "while"
	TokenDo       TokenType = "do"
	TokenRepeat   TokenType = "repeat"
	TokenUntil    TokenType = "until"
	TokenFor      TokenType = "for"
	TokenIn       TokenType = "in"
	TokenBreak    TokenType = "break"
	TokenTrue     TokenType = "true"
	TokenFalse    TokenType = "false"
	TokenNil      TokenType = "nil"
	TokenAnd      TokenType = "and"
	TokenOr       TokenType = "or"
	TokenNot      TokenType = "not"
	TokenDotDot   TokenType = ".."
	TokenEq       TokenType = "=="
	TokenNE       TokenType = "~="
	TokenLE       TokenType = "<="
	TokenGE       TokenType = ">="
	TokenDots     TokenType = "..."
)

type Position struct {
	Source string
	Line   int
	Column int
}

type Token struct {
	Type    TokenType
	Literal string
	Pos     Position
}

var keywords = map[string]TokenType{
	"local":    TokenLocal,
	"function": TokenFunction,
	"return":   TokenReturn,
	"end":      TokenEnd,
	"if":       TokenIf,
	"then":     TokenThen,
	"else":     TokenElse,
	"elseif":   TokenElseIf,
	"while":    TokenWhile,
	"do":       TokenDo,
	"repeat":   TokenRepeat,
	"until":    TokenUntil,
	"for":      TokenFor,
	"in":       TokenIn,
	"break":    TokenBreak,
	"true":     TokenTrue,
	"false":    TokenFalse,
	"nil":      TokenNil,
	"and":      TokenAnd,
	"or":       TokenOr,
	"not":      TokenNot,
}
