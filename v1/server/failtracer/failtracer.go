package failtracer

import (
	"fmt"
	"slices"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"

	"github.com/open-policy-agent/opa/internal/levenshtein"
)

const (
	// maxDistanceForHint is the levenshtein distance below which we'll emit a hint
	maxDistanceForHint = 3
)

type Hint struct {
	Message  string        `json:"message"`
	Location *ast.Location `json:"location,omitempty"`
}

type failTracer struct {
	exprs []*ast.Expr
}

type FailTracer interface {
	Enabled() bool
	TraceEvent(topdown.Event)
	Config() topdown.TraceConfig
	Hints([]ast.Ref) []Hint
}

func New() FailTracer {
	return &failTracer{}
}

// Enabled always returns true if the failTracer is instantiated.
func (b *failTracer) Enabled() bool {
	return b != nil
}

func (b *failTracer) TraceEvent(evt topdown.Event) {
	if evt.Op == topdown.FailOp {
		expr, ok := evt.Node.(*ast.Expr)
		if ok {
			b.exprs = append(b.exprs, expr)
		}
	}
}

// Config returns the Tracers standard configuration
func (*failTracer) Config() topdown.TraceConfig {
	return topdown.TraceConfig{PlugLocalVars: true}
}

func (b *failTracer) Hints(unknowns []ast.Ref) []Hint {
	var hints []Hint //nolint:prealloc
	seenRefs := map[string]struct{}{}
	candidates := make([]string, 0, len(unknowns))
	for _, ref := range unknowns {
		if len(ref) < 2 {
			continue
		}
		candidates = append(candidates, string(ref[1].Value.(ast.String)))
	}

	for _, expr := range b.exprs {
		var ref ast.Ref // when this is processed, only one input.X.Y ref is in the expression (SSA)
		switch {
		case expr.IsCall():
			for i := range 2 {
				op := expr.Operand(i)
				if r, ok := op.Value.(ast.Ref); ok && r.HasPrefix(ast.InputRootRef) {
					ref = r
				}
			}
		}
		// NOTE(sr): if we allow naked ast.Term for filter policies, they need to be handled in switch ^

		if len(ref) < 2 {
			continue
		}
		tblPart, ok := ref[1].Value.(ast.String)
		if !ok {
			continue
		}
		miss := string(tblPart)
		rs := ref[1:].String()
		if _, ok := seenRefs[rs]; ok {
			continue
		}

		closestStrings := levenshtein.ClosestStrings(maxDistanceForHint, miss, slices.Values(candidates))
		proposals := make([]ast.Ref, len(closestStrings))
		for i := range closestStrings {
			prop := make([]*ast.Term, 2, len(ref))
			prop[0] = ast.InputRootDocument
			prop[1] = ast.StringTerm(closestStrings[i])
			prop = append(prop, ref[2:]...)
			proposals[i] = prop
		}
		var msg string
		switch len(proposals) {
		case 0:
			continue
		case 1:
			msg = fmt.Sprintf("%v undefined, did you mean %s?", ref, proposals[0])
		default:
			msg = fmt.Sprintf("%v undefined, did you mean any of %v?", ref, proposals)
		}
		hints = append(hints, Hint{
			Location: expr.Loc(),
			Message:  msg,
		})
		seenRefs[rs] = struct{}{}
	}
	return hints
}
