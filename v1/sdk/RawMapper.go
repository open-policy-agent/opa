package sdk

import (
	"github.com/open-policy-agent/opa/v1/rego"
)

type RawMapper struct {
}

func (*RawMapper) MapResults(pq *rego.PartialQueries) (interface{}, error) {

	return pq, nil
}

func (*RawMapper) ResultToJSON(results interface{}) (interface{}, error) {
	return results, nil
}
