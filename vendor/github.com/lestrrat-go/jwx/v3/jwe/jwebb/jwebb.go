// Package jwebb provides the building blocks (hence the name "bb") for JWE operations.
// It should be thought of as a low-level API, almost akin to internal packages
// that should not be used directly by users of the jwx package. However, these exist
// to provide a more efficient way to perform JWE operations without the overhead of
// the higher-level jwe package to power-users who know what they are doing.
//
// This package is currently considered EXPERIMENTAL, and the API may change
// without notice. It is not recommended to use this package unless you are
// fully aware of the implications of using it.
//
// All bb packages in jwx follow the same design principles:
// 1. Does minimal checking of input parameters (for performance); callers need to ensure that the parameters are valid.
// 2. All exported functions are stringly typed (i.e. they do not take any parameters unless they absolutely have to).
// 3. Does not rely on other public jwx packages (they are standalone, except for internal packages).
package jwebb
