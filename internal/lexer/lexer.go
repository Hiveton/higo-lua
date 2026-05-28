package lexer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Lexer struct {
	source string
	input  string
	pos    int
	line   int
	col    int
}

func New(source, input string) *Lexer {
	return &Lexer{source: source, input: input, line: 1, col: 1}
}

func (l *Lexer) All() ([]Token, error) {
	var tokens []Token
	for {
		tok, err := l.Next()
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF {
			return tokens, nil
		}
	}
}

func (l *Lexer) Next() (Token, error) {
	if err := l.skipSpaceAndComments(); err != nil {
		return Token{}, err
	}
	pos := l.position()
	r := l.peek()
	if r == 0 {
		return Token{Type: TokenEOF, Pos: pos}, nil
	}
	if isIdentStart(r) {
		lit := l.readWhile(isIdentPart)
		if typ, ok := keywords[lit]; ok {
			return Token{Type: typ, Literal: lit, Pos: pos}, nil
		}
		return Token{Type: TokenIdent, Literal: lit, Pos: pos}, nil
	}
	if unicode.IsDigit(r) || (r == '.' && unicode.IsDigit(l.peekN(1))) {
		return Token{Type: TokenNumber, Literal: l.readNumber(), Pos: pos}, nil
	}
	if r == '"' || r == '\'' {
		s, err := l.readQuoted(r)
		return Token{Type: TokenString, Literal: s, Pos: pos}, err
	}
	if r == '[' {
		if s, ok, err := l.readLongBracket(); ok || err != nil {
			return Token{Type: TokenString, Literal: s, Pos: pos}, err
		}
	}

	for _, op := range []struct {
		text string
		typ  TokenType
	}{
		{"...", TokenDots}, {"..", TokenDotDot}, {"==", TokenEq}, {"~=", TokenNE}, {"<=", TokenLE}, {">=", TokenGE},
	} {
		if strings.HasPrefix(l.input[l.pos:], op.text) {
			for range op.text {
				l.advance()
			}
			return Token{Type: op.typ, Literal: op.text, Pos: pos}, nil
		}
	}
	l.advance()
	return Token{Type: TokenType(string(r)), Literal: string(r), Pos: pos}, nil
}

func (l *Lexer) skipSpaceAndComments() error {
	for {
		for unicode.IsSpace(l.peek()) {
			l.advance()
		}
		if l.peek() != '-' || l.peekN(1) != '-' {
			return nil
		}
		l.advance()
		l.advance()
		if l.peek() == '[' {
			if _, ok, err := l.readLongBracket(); ok || err != nil {
				if err != nil {
					return err
				}
				continue
			}
		}
		for l.peek() != 0 && l.peek() != '\n' {
			l.advance()
		}
	}
}

func (l *Lexer) readNumber() string {
	start := l.pos
	if l.peek() == '0' && (l.peekN(1) == 'x' || l.peekN(1) == 'X') {
		l.advance()
		l.advance()
		for isHexDigit(l.peek()) {
			l.advance()
		}
		return l.input[start:l.pos]
	}
	for unicode.IsDigit(l.peek()) {
		l.advance()
	}
	if l.peek() == '.' && l.peekN(1) != '.' {
		l.advance()
		for unicode.IsDigit(l.peek()) {
			l.advance()
		}
	}
	if l.peek() == 'e' || l.peek() == 'E' {
		mark := l.pos
		l.advance()
		if l.peek() == '+' || l.peek() == '-' {
			l.advance()
		}
		if !unicode.IsDigit(l.peek()) {
			l.pos = mark
			return l.input[start:l.pos]
		}
		for unicode.IsDigit(l.peek()) {
			l.advance()
		}
	}
	return l.input[start:l.pos]
}

func (l *Lexer) readQuoted(quote rune) (string, error) {
	l.advance()
	var b strings.Builder
	for {
		r := l.peek()
		if r == 0 {
			return "", l.err("unterminated string")
		}
		l.advance()
		if r == quote {
			return b.String(), nil
		}
		if r != '\\' {
			b.WriteRune(r)
			continue
		}
		esc := l.peek()
		if esc == 0 {
			return "", l.err("unterminated escape")
		}
		l.advance()
		switch esc {
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte('\v')
		case '\\', '"', '\'':
			b.WriteRune(esc)
		default:
			if unicode.IsDigit(esc) {
				digits := []rune{esc}
				for len(digits) < 3 && unicode.IsDigit(l.peek()) {
					digits = append(digits, l.peek())
					l.advance()
				}
				n, err := strconv.Atoi(string(digits))
				if err != nil || n > 255 {
					return "", l.err("invalid decimal escape")
				}
				b.WriteByte(byte(n))
				continue
			}
			b.WriteRune(esc)
		}
	}
}

func (l *Lexer) readUntil(end string) (string, error) {
	start := l.pos
	for {
		if l.pos >= len(l.input) {
			return "", l.err("unterminated long string")
		}
		if strings.HasPrefix(l.input[l.pos:], end) {
			s := l.input[start:l.pos]
			for range end {
				l.advance()
			}
			return s, nil
		}
		l.advance()
	}
}

func (l *Lexer) readLongBracket() (string, bool, error) {
	end, ok := l.longBracketEnd()
	if !ok {
		return "", false, nil
	}
	for i := 0; i < len(end); i++ {
		l.advance()
	}
	if l.peek() == '\n' {
		l.advance()
	} else if l.peek() == '\r' {
		l.advance()
		if l.peek() == '\n' {
			l.advance()
		}
	}
	s, err := l.readUntil(end)
	return s, true, err
}

func (l *Lexer) longBracketEnd() (string, bool) {
	if l.peek() != '[' {
		return "", false
	}
	i := l.pos + 1
	for i < len(l.input) && l.input[i] == '=' {
		i++
	}
	if i >= len(l.input) || l.input[i] != '[' {
		return "", false
	}
	return "]" + l.input[l.pos+1:i] + "]", true
}

func (l *Lexer) readWhile(fn func(rune) bool) string {
	start := l.pos
	for fn(l.peek()) {
		l.advance()
	}
	return l.input[start:l.pos]
}

func (l *Lexer) peek() rune { return l.peekN(0) }

func (l *Lexer) peekN(n int) rune {
	i := l.pos
	for ; n > 0 && i < len(l.input); n-- {
		_, size := utf8.DecodeRuneInString(l.input[i:])
		i += size
	}
	if i >= len(l.input) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.input[i:])
	return r
}

func (l *Lexer) advance() rune {
	r, size := utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += size
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

func (l *Lexer) position() Position {
	return Position{Source: l.source, Line: l.line, Column: l.col}
}

func (l *Lexer) err(msg string) error {
	p := l.position()
	return fmt.Errorf("%s:%d:%d: %s", p.Source, p.Line, p.Column, msg)
}

func isIdentStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }
func isIdentPart(r rune) bool  { return isIdentStart(r) || unicode.IsDigit(r) }
func isHexDigit(r rune) bool {
	return unicode.IsDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

func ParseNumber(lit string) (float64, error) {
	if strings.HasPrefix(lit, "0x") || strings.HasPrefix(lit, "0X") {
		n, err := strconv.ParseInt(lit[2:], 16, 64)
		return float64(n), err
	}
	return strconv.ParseFloat(lit, 64)
}
