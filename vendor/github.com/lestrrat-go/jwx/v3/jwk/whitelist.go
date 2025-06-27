package jwk

import "github.com/lestrrat-go/httprc/v3"

type Whitelist = httprc.Whitelist
type WhitelistFunc = httprc.WhitelistFunc

// InsecureWhitelist is an alias to httprc.InsecureWhitelist. Use
// functions in the `httprc` package to interact with this type.
type InsecureWhitelist = httprc.InsecureWhitelist

func NewInsecureWhitelist() InsecureWhitelist {
	return httprc.NewInsecureWhitelist()
}

// BlockAllWhitelist is an alias to httprc.BlockAllWhitelist. Use
// functions in the `httprc` package to interact with this type.
type BlockAllWhitelist = httprc.BlockAllWhitelist

func NewBlockAllWhitelist() BlockAllWhitelist {
	return httprc.NewBlockAllWhitelist()
}

// RegexpWhitelist is an alias to httprc.RegexpWhitelist. Use
// functions in the `httprc` package to interact with this type.
type RegexpWhitelist = httprc.RegexpWhitelist

func NewRegexpWhitelist() *RegexpWhitelist {
	return httprc.NewRegexpWhitelist()
}

// MapWhitelist is an alias to httprc.MapWhitelist. Use
// functions in the `httprc` package to interact with this type.
type MapWhitelist = httprc.MapWhitelist

func NewMapWhitelist() MapWhitelist {
	return httprc.NewMapWhitelist()
}
