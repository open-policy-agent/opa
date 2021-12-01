package cmd

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/cmd/internal/exec"
	sdk_test "github.com/open-policy-agent/opa/sdk/test"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func TestExecBasic(t *testing.T) {

	files := map[string]string{
		"test.json":  `{"foo": 7}`,
		"test2.yaml": `bar: 8`,
		"test3.yml":  `baz: 9`,
		"ignore":     `garbage`, // do not recognize this filetype
	}

	test.WithTempFS(files, func(dir string) {

		s := sdk_test.MustNewServer(sdk_test.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"test.rego": `
				package system
				main["hello"]
			`,
		}))

		defer s.Stop()

		var buf bytes.Buffer
		params := exec.NewParams(&buf)
		_ = params.OutputFormat.Set("json")
		params.ConfigOverrides = []string{
			"services.test.url=" + s.URL(),
			"bundles.test.resource=/bundles/bundle.tar.gz",
		}

		params.Paths = append(params.Paths, dir)
		err := runExec(params)
		if err != nil {
			t.Fatal(err)
		}

		output := util.MustUnmarshalJSON(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil))

		exp := util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/test.json",
			"result": ["hello"]
		}, {
			"path": "/test2.yaml",
			"result": ["hello"]
		}, {
			"path": "/test3.yml",
			"result": ["hello"]
		}]}`))

		if !reflect.DeepEqual(output, exp) {
			t.Fatal("Expected:", exp, "Got:", output)
		}
	})

}

func TestExecDecisionOption(t *testing.T) {

	files := map[string]string{
		"test.json": `{"foo": 7}`,
	}

	test.WithTempFS(files, func(dir string) {

		s := sdk_test.MustNewServer(sdk_test.MockBundle("/bundles/bundle.tar.gz", map[string]string{
			"test.rego": `
				package foo
				main["hello"]
			`,
		}))

		defer s.Stop()

		var buf bytes.Buffer
		params := exec.NewParams(&buf)
		_ = params.OutputFormat.Set("json")
		params.Decision = "foo/main"
		params.ConfigOverrides = []string{
			"services.test.url=" + s.URL(),
			"bundles.test.resource=/bundles/bundle.tar.gz",
		}

		params.Paths = append(params.Paths, dir)
		err := runExec(params)
		if err != nil {
			t.Fatal(err)
		}

		output := util.MustUnmarshalJSON(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil))

		exp := util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/test.json",
			"result": ["hello"]
		}]}`))

		if !reflect.DeepEqual(output, exp) {
			t.Fatal("Expected:", exp, "Got:", output)
		}

	})

}

func TestExecBundleFlag(t *testing.T) {

	files := map[string]string{
		"files/test.json": `{"foo": 7}`,
		"bundle/x.rego": `package system

		main["hello"]`,
	}

	test.WithTempFS(files, func(dir string) {

		var buf bytes.Buffer
		params := exec.NewParams(&buf)
		_ = params.OutputFormat.Set("json")
		params.BundlePaths = []string{dir + "/bundle/"}
		params.Paths = append(params.Paths, dir+"/files/")

		err := runExec(params)
		if err != nil {
			t.Fatal(err)
		}

		output := util.MustUnmarshalJSON(bytes.ReplaceAll(buf.Bytes(), []byte(dir), nil))

		exp := util.MustUnmarshalJSON([]byte(`{"result": [{
			"path": "/files/test.json",
			"result": ["hello"]
		}]}`))

		if !reflect.DeepEqual(output, exp) {
			t.Fatal("Expected:", exp, "Got:", output)
		}

	})
}
