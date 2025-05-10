package sdk

import (
	"github.com/open-policy-agent/opa/v1/rego"
)

type RawMapper struct {
}

func (*RawMapper) MapResults(pq *rego.PartialQueries) (any, error) {

	return pq, nil
}

func (*RawMapper) ResultToJSON(results any) (any, error) {
	return results, nil
}
