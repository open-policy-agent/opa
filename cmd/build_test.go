package cmd

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/loader"

	"github.com/open-policy-agent/opa/util/test"
)

func TestBuildProducesBundle(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			p = 1
		`,
	}

	test.WithTempFS(files, func(root string) {
		params := newBuildParams()
		params.outputFile = path.Join(root, "bundle.tar.gz")

		err := dobuild(params, []string{root})
		if err != nil {
			t.Fatal(err)
		}

		_, err = loader.NewFileLoader().AsBundle(params.outputFile)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestBuildFilesystemModeIgnoresTarGz(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			p = 1
		`,
	}

	test.WithTempFS(files, func(root string) {
		params := newBuildParams()
		params.outputFile = path.Join(root, "bundle.tar.gz")

		err := dobuild(params, []string{root})
		if err != nil {
			t.Fatal(err)
		}

		_, err = loader.NewFileLoader().AsBundle(params.outputFile)
		if err != nil {
			t.Fatal(err)
		}

		// Just run the build again to simulate the user doing back-to-back builds.
		err = dobuild(params, []string{root})
		if err != nil {
			t.Fatal(err)
		}

	})
}

func TestBuildErrorDoesNotWriteFile(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			p { p }
		`,
	}

	test.WithTempFS(files, func(root string) {
		params := newBuildParams()
		params.outputFile = path.Join(root, "bundle.tar.gz")

		err := dobuild(params, []string{root})
		if err == nil || !strings.Contains(err.Error(), "rule p is recursive") {
			t.Fatal("expected recursion error but got:", err)
		}

		if _, err := os.Stat(params.outputFile); err == nil {
			t.Fatal("expected stat error")
		}
	})
}

func TestBuildErrorVerifyNonBundle(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			p { p }
		`,
	}

	test.WithTempFS(files, func(root string) {
		params := newBuildParams()
		params.outputFile = path.Join(root, "bundle.tar.gz")
		params.pubKey = "secret"

		err := dobuild(params, []string{root})
		if err == nil {
			t.Fatal("expected error but got nil")
		}

		exp := "enable bundle mode (ie. --bundle) to verify or sign bundle files or directories"
		if err.Error() != exp {
			t.Fatalf("expected error message %v but got %v", exp, err.Error())
		}
	})
}
