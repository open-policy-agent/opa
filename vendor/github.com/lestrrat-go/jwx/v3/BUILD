load("@rules_go//go:def.bzl", "go_library", "go_test")
load("@gazelle//:def.bzl", "gazelle")

# gazelle:prefix github.com/lestrrat-go/jwx/v3
# gazelle:go_naming_convention import_alias

gazelle(name = "gazelle")

go_library(
    name = "jwx",
    srcs = [
        "format.go",
        "formatkind_string_gen.go",
        "jwx.go",
        "options.go",
    ],
    importpath = "github.com/lestrrat-go/jwx/v3",
    visibility = ["//visibility:public"],
    deps = [
        "//internal/json",
        "//internal/tokens",
        "@com_github_lestrrat_go_option_v2//:option",
    ],
)

go_test(
    name = "jwx_test",
    srcs = ["jwx_test.go"],
    deps = [
        ":jwx",
        "//internal/jose",
        "//internal/json",
        "//internal/jwxtest",
        "//jwa",
        "//jwe",
        "//jwk",
        "//jwk/ecdsa",
        "//jws",
        "@com_github_stretchr_testify//require",
    ],
)

alias(
    name = "go_default_library",
    actual = ":jwx",
    visibility = ["//visibility:public"],
)
