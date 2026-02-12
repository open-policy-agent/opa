package cases

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ir"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/test/cases"
	"github.com/open-policy-agent/opa/v1/test/cases/testdata"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/types"
)

type ExtendedTestCase struct {
	cases.TestCase
	EntryPoints    []string   `json:"entrypoints"`
	Plan           *ir.Policy `json:"plan"`
	WantPlanResult any        `json:"want_plan_result"`
}

type ExtendedSet struct {
	Cases []*ExtendedTestCase `json:"cases"`
}

const pluginName = "compliance-tester"

type noopEvalPlugin struct {
	prepared *ir.Policy
}

func (t *noopEvalPlugin) reset() {
	t.prepared = nil
}

func (*noopEvalPlugin) Eval(_ context.Context, _ *rego.EvalContext, _ ast.Value) (ast.Value, error) {
	return nil, errors.New("not intended for evaluations")
}

func (*noopEvalPlugin) IsTarget(s string) bool {
	return s == "compliance-tester"
}

func (t *noopEvalPlugin) PrepareForEval(_ context.Context, policy *ir.Policy, _ ...rego.PrepareOption) (rego.TargetPluginEval, error) {
	t.prepared = policy
	return t, nil
}

// LoadIrExtendedTestCases adds the IR plan to existing testdata for external IR languages to use for testing (e.g. opa-swift)
// accepts a path to an existing testdata folder (e.g. testdata/v1), reading each test and its cases
func LoadIrExtendedTestCases() ([]ExtendedSet, error) {
	// Used by the 'time/time caching' test
	ast.RegisterBuiltin(&ast.Builtin{
		Name: "test.sleep",
		Decl: types.NewFunction(
			types.Args(types.S),
			types.NewNull(),
		),
	})

	topdown.RegisterBuiltinFunc("test.sleep", func(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
		d, _ := time.ParseDuration(string(operands[0].Value.(ast.String)))
		time.Sleep(d)
		return iter(ast.NullTerm())
	})

	evalPlugin := &noopEvalPlugin{}
	rego.RegisterPlugin(pluginName, evalPlugin)

	var results []ExtendedSet

	err := fs.WalkDir(testdata.V1, ".", func(path string, dir fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if dir.IsDir() {
			return nil
		}

		f, err := testdata.V1.ReadFile(path)
		if err != nil {
			return err
		}

		var x ExtendedSet
		if err := yaml.Unmarshal(f, &x); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		for _, tc := range x.Cases {

			opts := []func(*rego.Rego){
				rego.Target(pluginName),
				rego.Query(tc.Query),
				rego.SetRegoVersion(ast.RegoV1),
			}
			for i := range tc.Modules {
				opts = append(opts, rego.Module(fmt.Sprintf("module-%d.rego", i), tc.Modules[i]))
			}
			evalPlugin.reset()
			_, err := rego.New(opts...).PrepareForEval(context.Background())
			if err != nil {
				return fmt.Errorf("query planning failure/Skipping %s: %v", tc.Note, err)
			}

			if evalPlugin.prepared == nil {
				return fmt.Errorf("query planning/Skipping %s: no prepared plan, unsure what to do", tc.Note)
			}

			if len(evalPlugin.prepared.Plans.Plans) != 1 {
				return fmt.Errorf(
					"query planning/Skipping %s: >1 plan (%d), unsure what to do",
					tc.Note,
					len(evalPlugin.prepared.Plans.Plans),
				)
			}
			tc.Filename = path
			tc.Plan = evalPlugin.prepared
			tc.EntryPoints = []string{evalPlugin.prepared.Plans.Plans[0].Name}
			tc.WantPlanResult = tc.WantResult

			results = append(results, x)
		}

		return nil
	})

	return results, err
}
