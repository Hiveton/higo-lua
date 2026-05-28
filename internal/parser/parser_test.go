package parser

import "testing"

func TestParserParsesFunctionAndForLoop(t *testing.T) {
	_, err := Parse("test.lua", `
function add(a, b)
  return a + b
end
local sum = 0
for i = 1, 3 do
  sum = sum + i
end
`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
}
