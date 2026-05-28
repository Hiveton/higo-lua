package lexer

import "testing"

func TestLexerSkipsCommentsAndReadsLongString(t *testing.T) {
	lx := New("test.lua", `-- comment
local value = [[hello
lua]]`)

	tokens, err := lx.All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	if len(tokens) < 5 {
		t.Fatalf("got %d tokens, want at least 5", len(tokens))
	}
	if tokens[0].Type != TokenLocal || tokens[1].Literal != "value" || tokens[3].Literal != "hello\nlua" {
		t.Fatalf("unexpected tokens: %#v", tokens[:4])
	}
}

func TestLexerReadsLua51LongBracketLevels(t *testing.T) {
	lx := New("test.lua", `--[=[ skip [[ nested marker ]] ]=]
local value = [==[hello [=[ inner ]=] lua]==]`)

	tokens, err := lx.All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	if len(tokens) < 5 {
		t.Fatalf("got %d tokens, want at least 5", len(tokens))
	}
	if tokens[0].Type != TokenLocal || tokens[1].Literal != "value" || tokens[3].Literal != "hello [=[ inner ]=] lua" {
		t.Fatalf("unexpected tokens: %#v", tokens[:4])
	}
}

func TestLexerDropsInitialNewlineInLongString(t *testing.T) {
	lx := New("test.lua", "local value = [[\nhello]]")

	tokens, err := lx.All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	if tokens[3].Type != TokenString || tokens[3].Literal != "hello" {
		t.Fatalf("long string token = %#v, want initial newline dropped", tokens[3])
	}
}

func TestLexerReadsLua51NumberExponentAndEscapes(t *testing.T) {
	lx := New("test.lua", `local a = 1e-3
local b = "A\10\097\t\z"`)

	tokens, err := lx.All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	if tokens[3].Type != TokenNumber || tokens[3].Literal != "1e-3" {
		t.Fatalf("number token = %#v, want 1e-3", tokens[3])
	}
	if tokens[7].Type != TokenString || tokens[7].Literal != "A\na\tz" {
		t.Fatalf("string token = %#v, want escaped string", tokens[7])
	}
}

func TestLexerReadsLeadingDotNumber(t *testing.T) {
	lx := New("test.lua", `local a = .5`)

	tokens, err := lx.All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	if tokens[3].Type != TokenNumber || tokens[3].Literal != ".5" {
		t.Fatalf("number token = %#v, want .5", tokens[3])
	}
}
