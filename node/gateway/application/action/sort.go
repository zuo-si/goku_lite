package action

import (
	"github.com/eolinker/goku-api-gateway/node/gateway/response"
)

//ArraySortFilter array sort filter
type ArraySortFilter struct {
	source string
	field  string
}

//Do do
func (f *ArraySortFilter) Do(value *response.Response) {
	if _, ok := value.Data.(map[string]interface{}); !ok {
		return
	}
	value.Sort(f.source, f.field)
}
