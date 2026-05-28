// Package interp is reserved for the public State-backed AST interpreter.
//
// The first implementation keeps interpreter execution methods close to the
// public state package so Go embedding APIs remain simple. Future revisions can
// move that code here without changing github.com/Hiveton/higo-lua/state.
package interp
