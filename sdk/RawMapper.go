package sdk

import (
	"github.com/open-policy-agent/opa/rego"
)

type RawMapper struct {
}

func (e *RawMapper) MapResults(pq *rego.PartialQueries) (interface{}, error) {

	return pq, nil
}

func (e *RawMapper) ResultToJSON(results interface{}) (interface{}, error) {
	return results, nil
}
