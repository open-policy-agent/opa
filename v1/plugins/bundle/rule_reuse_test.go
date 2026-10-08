package bundle

import (
	"maps"
	"slices"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/download"
	"github.com/open-policy-agent/opa/v1/metrics"
)

func TestPluginRuleReuse(t *testing.T) {
	ctx := t.Context()
	manager := getTestManager()
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop(ctx)
	name := "sp"
	plugin := New(&Config{Bundles: map[string]*Source{name: {RuleReuse: true}}}, manager)
	plugin.status[name] = &Status{Name: name, Metrics: metrics.New()}
	plugin.downloaders[name] = download.New(download.Config{}, plugin.manager.Client(""), name)

	rule := func(id, value string) string {
		return "# METADATA\n# labels:\n#   id: \"" + id + "\"\nallow if input.x == \"" + value + "\"\n\n"
	}
	activate := func(rev string, files [][2]string) map[string]any {
		t.Helper()
		files = append(files, [2]string{"/.manifest", `{"revision": "` + rev + `", "roots": ["sp", "main"]}`})
		b, err := bundle.NewCustomReader(bundle.NewTarballLoaderWithBaseURL(archive.MustWriteTarGz(files), "")).
			WithProcessAnnotations(true).
			WithRuleReuse(manager.GetCompiler).
			Read()
		if err != nil {
			t.Fatal(err)
		}
		m := metrics.New()
		if err := plugin.oneShot(ctx, name, download.Update{Bundle: &b, Metrics: m}); err != nil {
			t.Fatal(err)
		}
		return m.All()
	}
	main := [2]string{"/main/main.rego", "package main\n\nallowed if data.sp.allow\n"}

	got := activate("1", [][2]string{main, {"/sp/a.rego", "package sp\n\n" + rule("r1", "a") + rule("r2", "b")}})
	if got["counter_rego_module_compile_full"] != uint64(1) {
		t.Fatalf("first activation: %v", got)
	}

	got = activate("2", [][2]string{main, {"/sp/a.rego", "package sp\n\n" + rule("r0", "z") + rule("r1", "a") + rule("r2", "b")}})
	if _, full := got["counter_rego_module_compile_full"]; full ||
		got["counter_rego_module_compile_reused_rules"] != uint64(3) || got["counter_rego_module_parse_skipped_rules"] != uint64(3) {
		t.Fatalf("second activation: %v", got)
	}

	// Another bundle's activation reads sp's policies back from the store.
	other := "other"
	plugin.config.Bundles[other] = &Source{RuleReuse: true}
	plugin.status[other] = &Status{Name: other, Metrics: metrics.New()}
	plugin.downloaders[other] = download.New(download.Config{}, plugin.manager.Client(""), other)
	activateOther := func(rev, src string) map[string]any {
		t.Helper()
		ob, err := bundle.NewCustomReader(bundle.NewTarballLoaderWithBaseURL(archive.MustWriteTarGz([][2]string{
			{"/.manifest", `{"revision": "` + rev + `", "roots": ["o"]}`},
			{"/o/o.rego", src},
		}), "")).WithProcessAnnotations(true).Read()
		if err != nil {
			t.Fatal(err)
		}
		m := metrics.New()
		if err := plugin.oneShot(ctx, other, download.Update{Bundle: &ob, Metrics: m}); err != nil {
			t.Fatal(err)
		}
		return m.All()
	}
	activateOther("1", "package o\n\nx if input.o\n")
	got = activateOther("2", "package o\n\nx if input.o\n\nx if input.p\n")
	if _, full := got["counter_rego_module_compile_full"]; full || got["counter_rego_module_parse_skipped_rules"] != uint64(4) {
		t.Fatalf("other bundle's switch: %v", got)
	}

	c := manager.GetCompiler()
	if names := slices.Sorted(maps.Keys(c.Modules)); !slices.Equal(names, []string{"other/o/o.rego", "sp/main/main.rego", "sp/sp/a.rego"}) {
		t.Fatalf("modules: %v", names)
	}
	var values []string
	for _, r := range c.GetRulesExact(ast.MustParseRef("data.sp.allow")) {
		values = append(values, r.Body[0].Operand(1).String())
	}
	if !slices.Equal(values, []string{`"z"`, `"a"`, `"b"`}) {
		t.Fatalf("rules: %v", values)
	}
}
