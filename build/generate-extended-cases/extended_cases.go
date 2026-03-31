package cases

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ir"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/test/cases"
	"github.com/open-policy-agent/opa/v1/test/cases/testdata"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/types"
	"github.com/open-policy-agent/opa/v1/util"
)

var exceptionsFile = flag.String("exceptions", "./exceptions.yaml", "set file to load a list of test names to exclude")

var exceptions map[string]string

func setup() {
	exceptions = map[string]string{}

	bs, err := os.ReadFile(*exceptionsFile)
	if err != nil {
		fmt.Println("Unable to load exceptions file: " + err.Error())
		os.Exit(1)
	}
	err = util.Unmarshal(bs, &exceptions)
	if err != nil {
		fmt.Println("Unable to parse exceptions file: " + err.Error())
		os.Exit(1)
	}
}

func shouldSkip(tc cases.TestCase) bool {
	_, ok := exceptions[tc.Note]
	return ok
}

type ExtendedTestCase struct {
	cases.TestCase
	EntryPoints    []string   `json:"entrypoints"`
	Plan           *ir.Policy `json:"plan"`
	WantPlanResult any        `json:"want_plan_result"`
	Ignore         bool       `json:"ignore"`
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
	return LoadIrExtendedTestCasesFiltered()
}

// Filters are functions that will return true if a test case should be filtered out
type Filters func(*ExtendedTestCase) bool

// CapabilitiesFilter will filter out any test cases not using a defined capability
func CapabilitiesFilter(c *ast.Capabilities) Filters {
	builtins := map[*ir.BuiltinFunc]struct{}{}
	for _, b := range c.Builtins {
		builtins[&ir.BuiltinFunc{Name: b.Name, Decl: b.Decl}] = struct{}{}
	}

	return func(r *ExtendedTestCase) bool {
		if len(builtins) == 0 {
			return true
		}
		if len(builtins) > len(r.Plan.Static.BuiltinFuncs) {
			return true
		}

		for _, b := range r.Plan.Static.BuiltinFuncs {
			// if the test case contains a builtin not in the capabilities file, reject it
			if _, ok := builtins[b]; !ok {
				return true
			}
		}

		return false
	}
}

func LoadIrExtendedTestCasesFiltered(filters ...Filters) ([]ExtendedSet, error) {
	setup()

	// Used by the 'time/time caching' test
	sleep := &ast.Builtin{
		Name: "test.sleep",
		Decl: types.NewFunction(
			types.Args(types.S),
			types.NewNull(),
		),
	}
	if !slices.Contains(ast.Builtins, sleep) {
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
	}

	rego.ResetTargetPlugins()
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
		useNumber := yaml.JSONOpt(func(d *json.Decoder) *json.Decoder {
			d.UseNumber()
			return d
		})
		if err := yaml.Unmarshal(f, &x, useNumber); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		for _, tc := range x.Cases {

			if shouldSkip(tc.TestCase) {
				continue
			}

			// TODO: drop once future.keywords.not is enabled by default
			caps := ast.CapabilitiesForThisVersion()
			caps.FutureKeywords = append(caps.FutureKeywords, "not")

			opts := []func(*rego.Rego){
				rego.Target(pluginName),
				rego.Query(tc.Query),
				rego.SetRegoVersion(ast.RegoV1),
				rego.Capabilities(caps),
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

			for _, filter := range filters {
				if filter(tc) {
					tc.Ignore = true
					break
				}
			}

			results = append(results, x)
		}

		return nil
	})

	return results, err
}
