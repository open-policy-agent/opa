package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
)

func main() {
	in := os.Stdin
	if len(os.Args) > 1 {
		f, err := os.Open(os.Args[1])
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		in = f
	}
	pn, err := ParseReader("", in)
	if err != nil {
		log.Fatal(err)
	}
	ret, err := pn.(ProgramNode).exec()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ret)
}

var lvalues = make(map[string]int)

// Statement is the smallest standalone element
type Statement interface {
	exec() error
}

// ProgramNode is a root node
type ProgramNode struct {
	statements StatementsNode
	ret        ReturnNode
}

func newProgramNode(stmts StatementsNode, ret ReturnNode) (ProgramNode, error) {
	return ProgramNode{stmts, ret}, nil
}
func (n ProgramNode) exec() (int, error) {
	err := n.statements.exec()
	if err != nil {
		return 0, err
	}
	return n.ret.exec()
}

// StatementsNode is a list of statement
type StatementsNode struct {
	statements []Statement
}

func newStatementsNode(stmts interface{}) (StatementsNode, error) {

	st := toIfaceSlice(stmts)
	ex := make([]Statement, len(st))
	for i, v := range st {
		ex[i] = v.(Statement)
	}
	return StatementsNode{ex}, nil
}
func (n StatementsNode) exec() error {
	for _, v := range n.statements {
		err := v.exec()
		if err != nil {
			return err
		}
	}
	return nil
}

// ReturnNode return value to the caller.
type ReturnNode struct {
	arg IdentifierNode
}

func newReturnNode(arg IdentifierNode) (ReturnNode, error) {
	return ReturnNode{arg}, nil
}
func (n ReturnNode) exec() (int, error) {
	v, err := n.arg.exec()
	return v, err
}

// IfNode controls conditional branching.
type IfNode struct {
	arg        LogicalExpressionNode
	statements StatementsNode
}

func newIfNode(arg LogicalExpressionNode, stmts StatementsNode) (IfNode, error) {
	return IfNode{arg, stmts}, nil
}
func (n IfNode) exec() error {
	cond, err := n.arg.exec()
	if err != nil {
		return err
	}
	if cond {
		err := n.statements.exec()
		return err
	}
	return nil
}

// AssignmentNode gives a value to a variable
type AssignmentNode struct {
	lvalue string
	rvalue AdditiveExpressionNode
}

func newAssignmentNode(lvalue IdentifierNode, rvalue AdditiveExpressionNode) (AssignmentNode, error) {
	return AssignmentNode{lvalue.val, rvalue}, nil
}
func (n AssignmentNode) exec() error {
	v, err := n.rvalue.exec()
	if err != nil {
		return err
	}
	lvalues[n.lvalue] = v
	return nil
}

// LogicalExpressionNode is a logical expression
type LogicalExpressionNode struct {
	expr PrimaryExpressionNode
}

func newLogicalExpressionNode(expr PrimaryExpressionNode) (LogicalExpressionNode, error) {
	return LogicalExpressionNode{expr}, nil
}
func (n LogicalExpressionNode) exec() (bool, error) {
	ret, err := n.expr.exec()
	b := ret != 0
	return b, err
}

// AdditiveExpressionNode is a additive expression
type AdditiveExpressionNode struct {
	arg1 interface{}
	arg2 PrimaryExpressionNode
	op   string
}

func newAdditiveExpressionNode(arg PrimaryExpressionNode, rest interface{}) (AdditiveExpressionNode, error) {
	var a AdditiveExpressionNode
	var arg1 interface{} = arg

	restSl := toIfaceSlice(rest)
	if len(restSl) == 0 {
		zero, _ := newIntegerNode("0")
		arg2, _ := newPrimaryExpressionNode(zero)
		a = AdditiveExpressionNode{arg1, arg2, "+"}
	}
	for _, v := range restSl {
		restExpr := toIfaceSlice(v)
		arg2 := restExpr[3].(PrimaryExpressionNode)
		op := restExpr[1].(string)
		a = AdditiveExpressionNode{arg1, arg2, op}
		arg1 = a
	}
	return a, nil
}
func (n AdditiveExpressionNode) exec() (int, error) {
	var v, varg1, varg2 int
	var err error
	switch n.arg1.(type) {
	case PrimaryExpressionNode:
		varg1, err = n.arg1.(PrimaryExpressionNode).exec()
	case AdditiveExpressionNode:
		varg1, err = n.arg1.(AdditiveExpressionNode).exec()
	default:
		return 0, errors.New("arg1 has invalid node type while exec AdditiveExpression")
	}
	if err != nil {
		return varg1, err
	}
	varg2, err = n.arg2.exec()
	switch n.op {
	case "+":
		v = varg1 + varg2
	case "-":
		v = varg1 - varg2
	default:
		return 0, errors.New("invalid operation while exec AdditiveExpression")
	}
	return v, err
}

// PrimaryExpressionNode is a basic element
type PrimaryExpressionNode struct {
	arg interface{}
}

func newPrimaryExpressionNode(arg interface{}) (PrimaryExpressionNode, error) {
	return PrimaryExpressionNode{arg}, nil
}
func (n PrimaryExpressionNode) exec() (int, error) {
	var v int
	var err error
	switch n.arg.(type) {
	case IntegerNode:
		v, err = n.arg.(IntegerNode).exec()
	case IdentifierNode:
		v, err = n.arg.(IdentifierNode).exec()
	default:
		return 0, errors.New("invalid operation while exec AdditiveExpression")
	}
	return v, err
}

// IntegerNode is a integer number
type IntegerNode struct {
	val int
}

func newIntegerNode(val string) (IntegerNode, error) {
	v, err := strconv.ParseInt(val, 0, 64)
	return IntegerNode{int(v)}, err
}
func (n IntegerNode) exec() (int, error) {
	return n.val, nil
}

// IdentifierNode is a reference to variable
type IdentifierNode struct {
	val string
}

func newIdentifierNode(val string) (IdentifierNode, error) {
	return IdentifierNode{val}, nil
}
func (n IdentifierNode) exec() (int, error) {
	v, ok := lvalues[n.val]
	if !ok {
		return 0, errors.New("Identifier " + n.val + " not defined")
	}
	return v, nil
}
