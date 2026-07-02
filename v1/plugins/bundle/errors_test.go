package bundle

import (
	"errors"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/download"
)

func TestErrors(t *testing.T) {
	errs := Errors{
		NewBundleError("foo", errors.New("foo error")),
		NewBundleError("bar", errors.New("bar error")),
	}

	expected := "Bundle name: foo, Code: bundle_error, HTTPCode: -1, Message: foo error\nBundle name: bar, Code: bundle_error, HTTPCode: -1, Message: bar error"
	result := errs.Error()

	if result != expected {
		t.Errorf("Expected: %v \nbut got: %v", expected, result)
	}
}

func TestUnwrapSlice(t *testing.T) {
	fooErr := NewBundleError("foo", errors.New("foo error"))
	barErr := NewBundleError("bar", errors.New("bar error"))

	errs := Errors{fooErr, barErr}

	result := errs.Unwrap()

	if result[0].Error() != fooErr.Error() {
		t.Fatalf("expected %v \nbut got: %v", fooErr, result[0])
	}
	if result[1].Error() != barErr.Error() {
		t.Fatalf("expected %v \nbut got: %v", barErr, result[1])
	}
}

func TestUnwrap(t *testing.T) {
	serverHTTPError := NewBundleError("server", download.HTTPError{StatusCode: 500})
	clientHTTPError := NewBundleError("client", download.HTTPError{StatusCode: 400})
	astErrors := ast.Errors{ast.NewError(ast.ParseErr, ast.NewLocation(nil, "foo.rego", 100, 2), "blarg")}

	errs := Errors{serverHTTPError, clientHTTPError, NewBundleError("ast", astErrors)}

	// unwrap first bundle.Error
	var bundleError Error
	if !errors.As(errs, &bundleError) {
		t.Fatal("failed to unwrap Error")
	}
	if bundleError.Error() != serverHTTPError.Error() {
		t.Fatalf("expected: %v \ngot: %v", serverHTTPError, bundleError)
	}

	// unwrap first HTTPError
	var httpError download.HTTPError
	if !errors.As(errs, &httpError) {
		t.Fatal("failed to unwrap Error")
	}
	if httpError.Error() != serverHTTPError.Err.Error() {
		t.Fatalf("expected: %v \ngot: %v", serverHTTPError.Err, httpError)
	}

	// unwrap HTTPError from bundle.Error
	if !errors.As(bundleError, &httpError) {
		t.Fatal("failed to unwrap HTTPError")
	}
	if httpError.Error() != serverHTTPError.Err.Error() {
		t.Fatalf("expected: %v \nbgot: %v", serverHTTPError.Err, httpError)
	}

	var unwrappedAstErrors ast.Errors
	if !errors.As(errs, &unwrappedAstErrors) {
		t.Fatal("failed to unwrap ast.Errors")
	}
	if unwrappedAstErrors.Error() != astErrors.Error() {
		t.Fatalf("expected: %v \ngot: %v", astErrors, unwrappedAstErrors)
	}
}

func TestHTTPErrorWrapping(t *testing.T) {
	err := download.HTTPError{StatusCode: 500}
	bundleErr := NewBundleError("foo", err)

	if bundleErr.BundleName != "foo" {
		t.Fatalf("BundleName: expected: %v \ngot: %v", "foo", bundleErr.BundleName)
	}
	if bundleErr.HTTPCode != err.StatusCode {
		t.Fatalf("HTTPCode: expected: %v \ngot: %v", err.StatusCode, bundleErr.HTTPCode)
	}
	if bundleErr.Message != err.Error() {
		t.Fatalf("Message: expected: %v \ngot: %v", err.Error(), bundleErr.Message)
	}
	if bundleErr.Code != errCode {
		t.Fatalf("Code: expected: %v \ngot: %v", errCode, bundleErr.Code)
	}
	if bundleErr.Err != err {
		t.Fatalf("Err: expected: %v \ngot: %v", err, bundleErr.Err)
	}
}

func TestASTErrorsWrapping(t *testing.T) {
	err := ast.Errors{ast.NewError(ast.ParseErr, ast.NewLocation(nil, "foo.rego", 100, 2), "blarg")}
	bundleErr := NewBundleError("foo", err)

	if bundleErr.BundleName != "foo" {
		t.Fatalf("BundleName: expected: %v \ngot: %v", "foo", bundleErr.BundleName)
	}
	if bundleErr.HTTPCode != -1 {
		t.Fatalf("HTTPCode: expected: %v \ngot: %v", -1, bundleErr.HTTPCode)
	}
	if bundleErr.Message != err.Error() {
		t.Fatalf("Message: expected: %v \ngot: %v", err.Error(), bundleErr.Message)
	}
	if bundleErr.Code != errCode {
		t.Fatalf("Code: expected: %v \ngot: %v", errCode, bundleErr.Code)
	}
	if bundleErr.Err.Error() != err.Error() {
		t.Fatalf("Err: expected: %v \ngot: %v", err.Error(), bundleErr.Err.Error())
	}
}

func TestGenericErrorWrapping(t *testing.T) {
	err := errors.New("foo error")
	bundleErr := NewBundleError("foo", err)

	if bundleErr.BundleName != "foo" {
		t.Fatalf("BundleName: expected: %v \ngot: %v", "foo", bundleErr.BundleName)
	}
	if bundleErr.HTTPCode != -1 {
		t.Fatalf("HTTPCode: expected: %v \ngot: %v", -1, bundleErr.HTTPCode)
	}
	if bundleErr.Message != err.Error() {
		t.Fatalf("Message: expected: %v \ngot: %v", err.Error(), bundleErr.Message)
	}
	if bundleErr.Code != errCode {
		t.Fatalf("Code: expected: %v \ngot: %v", errCode, bundleErr.Code)
	}
	if bundleErr.Err.Error() != err.Error() {
		t.Fatalf("Err: expected: %v \ngot: %v", err.Error(), bundleErr.Err.Error())
	}
}
