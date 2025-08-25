/*
Copyright (c) the purl authors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

// Package packageurl implements the package-url spec
package packageurl

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
)

var (
	// QualifierKeyPattern describes a valid qualifier key:
	//
	// - The key must be composed only of ASCII letters and numbers, '.',
	//   '-' and '_' (period, dash and underscore).
	// - A key cannot start with a number.
	QualifierKeyPattern = regexp.MustCompile(`^[A-Za-z\.\-_][0-9A-Za-z\.\-_]*$`)
	// TypePattern describes a valid type:
	//
	// - The type must be composed only of ASCII letters and numbers, '.',
	// '+' and '-' (period, plus and dash).
	// - A type cannot start with a number.
	TypePattern = regexp.MustCompile(`^[A-Za-z\.\-\+][0-9A-Za-z\.\-\+]*$`)
)

// These are the known purl types as defined in the spec. Some of these require
// special treatment during parsing.
// https://github.com/package-url/purl-spec#known-purl-types
var (
	// TypeAlpm is a pkg:alpm purl.
	TypeAlpm = "alpm"
	// TypeApk is a pkg:apk purl.
	TypeApk = "apk"
	// TypeBitbucket is a pkg:bitbucket purl.
	TypeBitbucket = "bitbucket"
	// TypeBitnami is a pkg:bitnami purl.
	TypeBitnami = "bitnami"
	// TypeCargo is a pkg:cargo purl.
	TypeCargo = "cargo"
	// TypeCocoapods is a pkg:cocoapods purl.
	TypeCocoapods = "cocoapods"
	// TypeComposer is a pkg:composer purl.
	TypeComposer = "composer"
	// TypeConan is a pkg:conan purl.
	TypeConan = "conan"
	// TypeConda is a pkg:conda purl.
	TypeConda = "conda"
	// TypeCran is a pkg:cran purl.
	TypeCran = "cran"
	// TypeDebian is a pkg:deb purl.
	TypeDebian = "deb"
	// TypeDocker is a pkg:docker purl.
	TypeDocker = "docker"
	// TypeGem is a pkg:gem purl.
	TypeGem = "gem"
	// TypeGeneric is a pkg:generic purl.
	TypeGeneric = "generic"
	// TypeGithub is a pkg:github purl.
	TypeGithub = "github"
	// TypeGolang is a pkg:golang purl.
	TypeGolang = "golang"
	// TypeHackage is a pkg:hackage purl.
	TypeHackage = "hackage"
	// TypeHex is a pkg:hex purl.
	TypeHex = "hex"
	// TypeHuggingface is pkg:huggingface purl.
	TypeHuggingface = "huggingface"
	// TypeMLflow is pkg:mlflow purl.
	TypeMLFlow = "mlflow"
	// TypeMaven is a pkg:maven purl.
	TypeMaven = "maven"
	// TypeNPM is a pkg:npm purl.
	TypeNPM = "npm"
	// TypeNuget is a pkg:nuget purl.
	TypeNuget = "nuget"
	// TypeOCI is a pkg:oci purl
	TypeOCI = "oci"
	// TypePub is a pkg:pub purl.
	TypePub = "pub"
	// TypePyPi is a pkg:pypi purl.
	TypePyPi = "pypi"
	// TypeQPKG is a pkg:qpkg purl.
	TypeQpkg = "qpkg"
	// TypeRPM is a pkg:rpm purl.
	TypeRPM = "rpm"
	// TypeSWID is pkg:swid purl
	TypeSWID = "swid"
	// TypeSwift is pkg:swift purl
	TypeSwift = "swift"

	// KnownTypes is a map of types that are officially supported by the spec.
	// See https://github.com/package-url/purl-spec/blob/master/PURL-TYPES.rst#known-purl-types
	KnownTypes = map[string]struct{}{
		TypeAlpm:        {},
		TypeApk:         {},
		TypeBitbucket:   {},
		TypeBitnami:     {},
		TypeCargo:       {},
		TypeCocoapods:   {},
		TypeComposer:    {},
		TypeConan:       {},
		TypeConda:       {},
		TypeCran:        {},
		TypeDebian:      {},
		TypeDocker:      {},
		TypeGem:         {},
		TypeGeneric:     {},
		TypeGithub:      {},
		TypeGolang:      {},
		TypeHackage:     {},
		TypeHex:         {},
		TypeHuggingface: {},
		TypeMaven:       {},
		TypeMLFlow:      {},
		TypeNPM:         {},
		TypeNuget:       {},
		TypeOCI:         {},
		TypePub:         {},
		TypePyPi:        {},
		TypeQpkg:        {},
		TypeRPM:         {},
		TypeSWID:        {},
		TypeSwift:       {},
	}

	TypeApache      = "apache"
	TypeAndroid     = "android"
	TypeAtom        = "atom"
	TypeBower       = "bower"
	TypeBrew        = "brew"
	TypeBuildroot   = "buildroot"
	TypeCarthage    = "carthage"
	TypeChef        = "chef"
	TypeChocolatey  = "chocolatey"
	TypeClojars     = "clojars"
	TypeCoreos      = "coreos"
	TypeCpan        = "cpan"
	TypeCtan        = "ctan"
	TypeCrystal     = "crystal"
	TypeDrupal      = "drupal"
	TypeDtype       = "dtype"
	TypeDub         = "dub"
	TypeElm         = "elm"
	TypeEclipse     = "eclipse"
	TypeGitea       = "gitea"
	TypeGitlab      = "gitlab"
	TypeGradle      = "gradle"
	TypeGuix        = "guix"
	TypeHaxe        = "haxe"
	TypeHelm        = "helm"
	TypeJulia       = "julia"
	TypeLua         = "lua"
	TypeMelpa       = "melpa"
	TypeMeteor      = "meteor"
	TypeNim         = "nim"
	TypeNix         = "nix"
	TypeOpam        = "opam"
	TypeOpenwrt     = "openwrt"
	TypeOsgi        = "osgi"
	TypeP2          = "p2"
	TypePear        = "pear"
	TypePecl        = "pecl"
	TypePERL6       = "perl6"
	TypePlatformio  = "platformio"
	TypeEbuild      = "ebuild"
	TypePuppet      = "puppet"
	TypeSourceforge = "sourceforge"
	TypeSublime     = "sublime"
	TypeTerraform   = "terraform"
	TypeVagrant     = "vagrant"
	TypeVim         = "vim"
	TypeWORDPRESS   = "wordpress"
	TypeYocto       = "yocto"

	// CandidateTypes is a map of types that are not yet officially supported by the spec,
	// but are being considered for inclusion.
	// See https://github.com/package-url/purl-spec/blob/master/PURL-TYPES.rst#other-candidate-types-to-define
	CandidateTypes = map[string]struct{}{
		TypeApache:      {},
		TypeAndroid:     {},
		TypeAtom:        {},
		TypeBower:       {},
		TypeBrew:        {},
		TypeBuildroot:   {},
		TypeCarthage:    {},
		TypeChef:        {},
		TypeChocolatey:  {},
		TypeClojars:     {},
		TypeCoreos:      {},
		TypeCpan:        {},
		TypeCtan:        {},
		TypeCrystal:     {},
		TypeDrupal:      {},
		TypeDtype:       {},
		TypeDub:         {},
		TypeElm:         {},
		TypeEclipse:     {},
		TypeGitea:       {},
		TypeGitlab:      {},
		TypeGradle:      {},
		TypeGuix:        {},
		TypeHaxe:        {},
		TypeHelm:        {},
		TypeJulia:       {},
		TypeLua:         {},
		TypeMelpa:       {},
		TypeMeteor:      {},
		TypeNim:         {},
		TypeNix:         {},
		TypeOpam:        {},
		TypeOpenwrt:     {},
		TypeOsgi:        {},
		TypeP2:          {},
		TypePear:        {},
		TypePecl:        {},
		TypePERL6:       {},
		TypePlatformio:  {},
		TypeEbuild:      {},
		TypePuppet:      {},
		TypeSourceforge: {},
		TypeSublime:     {},
		TypeTerraform:   {},
		TypeVagrant:     {},
		TypeVim:         {},
		TypeWORDPRESS:   {},
		TypeYocto:       {},
	}
)

// Qualifier represents a single key=value qualifier in the package url
type Qualifier struct {
	Key   string
	Value string
}

func (q Qualifier) String() string {
	// A value must be a percent-encoded string
	return fmt.Sprintf("%s=%s", q.Key, url.PathEscape(q.Value))
}

// Qualifiers is a slice of key=value pairs, with order preserved as it appears
// in the package URL.
type Qualifiers []Qualifier

// urlQuery returns a raw URL query with all the qualifiers as keys + values.
func (q Qualifiers) urlQuery() (rawQuery string) {
	v := make(url.Values)
	for _, qq := range q {
		v.Add(qq.Key, qq.Value)
	}
	return v.Encode()
}

// QualifiersFromMap constructs a Qualifiers slice from a string map. To get a
// deterministic qualifier order (despite maps not providing any iteration order
// guarantees) the returned Qualifiers are sorted in increasing order of key.
func QualifiersFromMap(mm map[string]string) Qualifiers {
	q := Qualifiers{}

	for k, v := range mm {
		q = append(q, Qualifier{Key: k, Value: v})
	}

	// sort for deterministic qualifier order
	sort.Slice(q, func(i int, j int) bool { return q[i].Key < q[j].Key })

	return q
}

// Map converts a Qualifiers struct to a string map.
func (qq Qualifiers) Map() map[string]string {
	m := make(map[string]string)

	for i := 0; i < len(qq); i++ {
		k := qq[i].Key
		v := qq[i].Value
		m[k] = v
	}

	return m
}

func (qq Qualifiers) String() string {
	var kvPairs []string
	for _, q := range qq {
		kvPairs = append(kvPairs, q.String())
	}
	return strings.Join(kvPairs, "&")
}

func (qq *Qualifiers) Normalize() error {
	qs := *qq
	normedQQ := make(Qualifiers, 0, len(qs))
	for _, q := range qs {
		if q.Key == "" {
			return fmt.Errorf("key is missing from qualifier: %v", q)
		}
		if q.Value == "" {
			// Empty values are equivalent to the key being omitted from the PackageURL.
			continue
		}
		key := strings.ToLower(q.Key)
		if !validQualifierKey(key) {
			return fmt.Errorf("invalid qualifier key: %q", key)
		}
		normedQQ = append(normedQQ, Qualifier{key, q.Value})
	}
	sort.Slice(normedQQ, func(i, j int) bool { return normedQQ[i].Key < normedQQ[j].Key })
	for i := 1; i < len(normedQQ); i++ {
		if normedQQ[i-1].Key == normedQQ[i].Key {
			return fmt.Errorf("duplicate qualifier key: %q", normedQQ[i].Key)
		}
	}
	*qq = normedQQ
	return nil
}

// PackageURL is the struct representation of the parts that make a package url
type PackageURL struct {
	Type       string
	Namespace  string
	Name       string
	Version    string
	Qualifiers Qualifiers
	Subpath    string
}

// NewPackageURL creates a new PackageURL struct instance based on input
func NewPackageURL(purlType, namespace, name, version string,
	qualifiers Qualifiers, subpath string) *PackageURL {

	return &PackageURL{
		Type:       purlType,
		Namespace:  namespace,
		Name:       name,
		Version:    version,
		Qualifiers: qualifiers,
		Subpath:    subpath,
	}
}

// ToString returns the human-readable instance of the PackageURL structure.
// This is the literal purl as defined by the spec.
func (p *PackageURL) ToString() string {
	u := &url.URL{
		Scheme:   "pkg",
		RawQuery: p.Qualifiers.urlQuery(),
		Fragment: p.Subpath,
	}

	paths := []string{p.Type}
	// we need to escape each segment by itself, so that we don't escape "/" in the namespace.
	for _, segment := range strings.Split(p.Namespace, "/") {
		if segment == "" {
			continue
		}
		paths = append(paths, escape(segment))
	}

	nameWithVersion := escape(p.Name)
	if p.Version != "" {
		nameWithVersion += "@" + escape(p.Version)
	}

	paths = append(paths, nameWithVersion)

	u.Opaque = strings.Join(paths, "/")
	return u.String()
}

func (p PackageURL) String() string {
	return p.ToString()
}

// FromString parses a valid package url string into a PackageURL structure
func FromString(purl string) (PackageURL, error) {
	u, err := url.Parse(purl)
	if err != nil {
		return PackageURL{}, fmt.Errorf("failed to parse as URL: %w", err)
	}

	if u.Scheme != "pkg" {
		return PackageURL{}, fmt.Errorf("purl scheme is not \"pkg\": %q", u.Scheme)
	}

	p := u.Opaque
	// if a purl starts with pkg:/ or even pkg://, we need to fall back to host + path.
	if p == "" {
		p = strings.TrimPrefix(path.Join(u.Host, u.Path), "/")
	}

	typ, p, ok := strings.Cut(p, "/")
	if !ok {
		return PackageURL{}, fmt.Errorf("purl is missing type or name")
	}
	typ = strings.ToLower(typ)

	qualifiers, err := parseQualifiers(u.RawQuery)
	if err != nil {
		return PackageURL{}, fmt.Errorf("invalid qualifiers: %w", err)
	}
	namespace, name, version, err := separateNamespaceNameVersion(p)
	if err != nil {
		return PackageURL{}, err
	}

	pURL := PackageURL{
		Qualifiers: qualifiers,
		Type:       typ,
		Namespace:  namespace,
		Name:       name,
		Version:    version,
		Subpath:    u.Fragment,
	}

	err = pURL.Normalize()
	return pURL, err
}

// Normalize converts p to its canonical form, returning an error if p is invalid.
func (p *PackageURL) Normalize() error {
	typ := strings.ToLower(p.Type)
	if !validType(typ) {
		return fmt.Errorf("invalid type %q", typ)
	}
	namespace := strings.Trim(p.Namespace, "/")
	if err := p.Qualifiers.Normalize(); err != nil {
		return fmt.Errorf("invalid qualifiers: %v", err)
	}
	if p.Name == "" {
		return errors.New("purl is missing name")
	}
	subpath := strings.Trim(p.Subpath, "/")
	segs := strings.Split(p.Subpath, "/")
	for i, s := range segs {
		if (s == "." || s == "..") && i != 0 {
			return fmt.Errorf("invalid Package URL subpath: %q", p.Subpath)
		}
	}
	*p = PackageURL{
		Type:       typ,
		Namespace:  typeAdjustNamespace(typ, namespace),
		Name:       typeAdjustName(typ, p.Name, p.Qualifiers),
		Version:    typeAdjustVersion(typ, p.Version),
		Qualifiers: p.Qualifiers,
		Subpath:    subpath,
	}
	return validCustomRules(*p)
}

// escape the given string in a purl-compatible way.
func escape(s string) string {
	// for compatibility with other implementations and the purl-spec, we want to escape all
	// characters, which is what "QueryEscape" does. The issue with QueryEscape is that it encodes
	// " " (space) as "+", which is valid in a query, but invalid in a path (see
	// https://stackoverflow.com/questions/2678551/when-should-space-be-encoded-to-plus-or-20) for
	// context).
	// To work around that, we replace the "+" signs with the path-compatible "%20".
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func separateNamespaceNameVersion(path string) (ns, name, version string, err error) {
	name = path

	if namespaceSep := strings.LastIndex(name, "/"); namespaceSep != -1 {
		ns, name = name[:namespaceSep], name[namespaceSep+1:]

		ns, err = url.PathUnescape(ns)
		if err != nil {
			return "", "", "", fmt.Errorf("error unescaping namespace: %w", err)
		}
	}

	if versionSep := strings.LastIndex(name, "@"); versionSep != -1 {
		name, version = name[:versionSep], name[versionSep+1:]

		version, err = url.PathUnescape(version)
		if err != nil {
			return "", "", "", fmt.Errorf("error unescaping version: %w", err)
		}
	}

	name, err = url.PathUnescape(name)
	if err != nil {
		return "", "", "", fmt.Errorf("error unescaping name: %w", err)
	}

	if name == "" {
		return "", "", "", fmt.Errorf("purl is missing name")
	}

	return ns, name, version, nil
}

func parseQualifiers(rawQuery string) (Qualifiers, error) {
	// we need to parse the qualifiers ourselves and cannot rely on the `url.Query` type because
	// that uses a map, meaning it's unordered. We want to keep the order of the qualifiers, so this
	// function re-implements the `url.parseQuery` function based on our `Qualifier` type. Most of
	// the code here is taken from `url.parseQuery`.
	q := Qualifiers{}
	for rawQuery != "" {
		var key string
		key, rawQuery, _ = strings.Cut(rawQuery, "&")
		if strings.Contains(key, ";") {
			return nil, fmt.Errorf("invalid semicolon separator in query")
		}
		if key == "" {
			continue
		}
		key, value, _ := strings.Cut(key, "=")
		key, err := url.QueryUnescape(key)
		if err != nil {
			return nil, fmt.Errorf("error unescaping qualifier key %q", key)
		}

		if !validQualifierKey(key) {
			return nil, fmt.Errorf("invalid qualifier key: '%s'", key)
		}

		value, err = url.QueryUnescape(value)
		if err != nil {
			return nil, fmt.Errorf("error unescaping qualifier value %q", value)
		}

		q = append(q, Qualifier{
			Key:   strings.ToLower(key),
			Value: value,
		})
	}
	return q, nil
}

// Make any purl type-specific adjustments to the parsed namespace.
// See https://github.com/package-url/purl-spec#known-purl-types
func typeAdjustNamespace(purlType, ns string) string {
	switch purlType {
	case TypeAlpm,
		TypeApk,
		TypeBitbucket,
		TypeComposer,
		TypeDebian,
		TypeGithub,
		TypeGolang,
		TypeNPM,
		TypeRPM,
		TypeQpkg:
		return strings.ToLower(ns)
	}
	return ns
}

// Make any purl type-specific adjustments to the parsed name.
// See https://github.com/package-url/purl-spec#known-purl-types
func typeAdjustName(purlType, name string, qualifiers Qualifiers) string {
	quals := qualifiers.Map()
	switch purlType {
	case TypeAlpm,
		TypeApk,
		TypeBitbucket,
		TypeBitnami,
		TypeComposer,
		TypeDebian,
		TypeGithub,
		TypeGolang,
		TypeNPM:
		return strings.ToLower(name)
	case TypePyPi:
		return strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	case TypeMLFlow:
		return adjustMlflowName(name, quals)
	}
	return name
}

// Make any purl type-specific adjustments to the parsed version.
// See https://github.com/package-url/purl-spec#known-purl-types
func typeAdjustVersion(purlType, version string) string {
	switch purlType {
	case TypeHuggingface:
		return strings.ToLower(version)
	}
	return version
}

// https://github.com/package-url/purl-spec/blob/master/PURL-TYPES.rst#mlflow
func adjustMlflowName(name string, qualifiers map[string]string) string {
	if repo, ok := qualifiers["repository_url"]; ok {
		if strings.Contains(repo, "azureml") {
			// Azure ML is case-sensitive and must be kept as-is
			return name
		} else if strings.Contains(repo, "databricks") {
			// Databricks is case-insensitive and must be lowercased
			return strings.ToLower(name)
		} else {
			// Unknown repository type, keep as-is
			return name
		}
	} else {
		// No repository qualifier given, keep as-is
		return name
	}
}

// validQualifierKey validates a qualifierKey against our QualifierKeyPattern.
func validQualifierKey(key string) bool {
	return QualifierKeyPattern.MatchString(key)
}

// validType validates a type against our TypePattern.
func validType(typ string) bool {
	return TypePattern.MatchString(typ)
}

// validCustomRules evaluates additional rules for each package url type, as specified in the package-url specification.
// On success, it returns nil. On failure, a descriptive error will be returned.
func validCustomRules(p PackageURL) error {
	q := p.Qualifiers.Map()
	switch p.Type {
	case TypeConan:
		if p.Namespace != "" {
			if val, ok := q["channel"]; ok {
				if val == "" {
					return errors.New("the qualifier channel must be not empty if namespace is present")
				}
			} else {
				return errors.New("channel qualifier does not exist")
			}
		} else {
			if val, ok := q["channel"]; ok {
				if val != "" {
					return errors.New("namespace is required if channel is non empty")
				}
			}
		}
	case TypeSwift:
		if p.Namespace == "" {
			return errors.New("namespace is required")
		}
		if p.Version == "" {
			return errors.New("version is required")
		}
	case TypeCran:
		if p.Version == "" {
			return errors.New("version is required")
		}
	}
	return nil
}
