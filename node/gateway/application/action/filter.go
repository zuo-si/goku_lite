package action

import (
	"github.com/eolinker/goku-api-gateway/node/gateway/response"
)

//ArrayFilter array filter
type ArrayFilter struct {
	source      string
	field       string
	operator    string
	targetValue string
}

//Do do
func (f *ArrayFilter) Do(value *response.Response) {
	if _, ok := value.Data.(map[string]interface{}); !ok {
		return
	}
	value.Filter(f.source, f.field, f.operator, f.targetValue)
}
