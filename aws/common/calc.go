// Inspired from https://github.com/alecthomas/participle/blob/master/_examples/expr/main.go
//
// This file contains the particle base expression parser and evaluator to compute the values
// of sequences.
package common

import (
	"github.com/alecthomas/participle"
	"github.com/pkg/errors"
	"strings"
)

type Factor struct {
	Number        *int64      `  @Int`
	Variable      *string     `| @"x"`
	Subexpression *Expression `| "(" @@ ")"`
}

type OpValue struct {
	Op     string  `@("*" | "/")`
	Factor *Factor `@@`
}

type Term struct {
	Factor   *Factor    `@@`
	OpFactor []*OpValue `(@@)*`
}

type OpTerm struct {
	Op   string `@("+" | "-")`
	Term *Term  `@@`
}

type Expression struct {
	Term    *Term     `@@`
	OpTerms []*OpTerm `(@@)*`
}

var parser = participle.MustBuild(&Expression{})

func EvalExpression(e Expression, x int64) int64 {
	value := EvalTerm(*e.Term, x)
	for _, t := range e.OpTerms {
		tmp := EvalTerm(*t.Term, x)
		value = EvalOp(value, t.Op, tmp)
	}
	return value
}

func EvalTerm(t Term, x int64) int64 {
	value := EvalFactor(*t.Factor, x)
	for _, f := range t.OpFactor {
		tmp := EvalFactor(*f.Factor, x)
		value = EvalOp(value, f.Op, tmp)
	}
	return value
}

func EvalFactor(t Factor, x int64) int64 {
	if t.Number != nil {
		return *t.Number
	} else if t.Variable != nil {
		return x
	} else {
		return EvalExpression(*t.Subexpression, x)
	}
}

func EvalOp(v1 int64, op string, v2 int64) int64 {
	switch op {
	case "+":
		return v1 + v2
	case "-":
		return v1 - v2
	case "*":
		return v1 * v2
	case "/":
		return v1 / v2
	}
	panic("unsupported operator : " + op)
}

func Eval(expr string, x int64) (int64, error) {
	ast := &Expression{}
	if err := parser.Parse(strings.NewReader(expr), ast); err != nil {
		return 0, errors.Wrapf(err, "could not parse the expression `%s`", expr)
	}
	return EvalExpression(*ast, x), nil
}
