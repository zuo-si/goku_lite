package response

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unsafe"
)

//Response response
type Response struct {
	// root 数据
	Data interface{}
}

//Delete delete
func (r *Response) Delete(pattern string) *Response {
	if pattern == "" {
		return r
	}
	root := _Node{
		data: r.Data,
	}

	root.Pattern(pattern, func(node *_Node) bool {
		if node.parent == nil {
			return false
		}
		parent := node.parent
		switch parent.data.(type) {
		case []interface{}:
			index := node.index
			sl := parent.data.([]interface{})

			nl := sl[:index]
			sl = append(nl, sl[index+1])
			parent.data = sl

		case map[string]interface{}:
			mp := parent.data.(map[string]interface{})
			delete(mp, node.key)
		}
		return false
	})
	return r
}

//SetValue 设置目标值，如果目标不存在，会对路径进行创建
func (r *Response) SetValue(pattern string, value interface{}) {
	if pattern == "" {
		r.Data = value
		return
	}
	root := _Node{
		data: r.Data,
	}
	root.Make(strings.Split(pattern, "."))

	root.Pattern(pattern, func(node *_Node) bool {

		if node.parent == nil {
			return false
		}
		parent := node.parent
		switch parent.data.(type) {
		case []interface{}:
			sl := parent.data.([]interface{})
			index := node.index
			sl[index] = value
			parent.data = sl

		case map[string]interface{}:
			mp := parent.data.(map[string]interface{})
			mp[node.key] = value
		}
		return false
	})

}

//ReTarget 选择目标重新设置为root
func (r *Response) ReTarget(pattern string) {
	if pattern == "" {
		return
	}
	root := _Node{
		data: r.Data,
	}

	match, _ := root.Pattern(pattern, func(node *_Node) bool {
		r.Data = node.data

		return true
	})
	if !match {
		r.Data = make(map[string]interface{})
	}
	return
}

//Group group
func (r *Response) Group(path []string) {
	l := len(path)
	if l == 0 {
		return
	}
	root := make(map[string]interface{})
	node := root

	lastKey := path[l-1]
	if l > 1 {
		for _, key := range path[:l-1] {
			v := make(map[string]interface{})
			node[key] = v
			node = v
		}
	}

	node[lastKey] = r.Data
	r.Data = root
}

//ReName 重命名
func (r *Response) ReName(pattern string, newName string) {
	if pattern == "" {
		return
	}
	root := _Node{
		data: r.Data,
	}

	root.Pattern(pattern, func(node *_Node) bool {
		if node.parent == nil {
			return false
		}
		parent := node.parent
		switch parent.data.(type) {
		case []interface{}:
			return false

		case map[string]interface{}:
			mp := parent.data.(map[string]interface{})
			delete(mp, node.key)
			mp[newName] = node.data
			return false
		}
		return false
	})
}

//FirstKeyUpper 第一层key首字母大写
func (r *Response) FirstKeyUpper() {
	if _, ok := r.Data.(map[string]interface{}); !ok {
		return
	}
	newObj := map[string]interface{}{}
	for key, val := range r.Data.(map[string]interface{}) {
		if len(key) == 1 {
			newObj[key] = val
		}
		if len(key) > 1 {
			newObj[strings.ToUpper(key[0:1])+key[1:]] = val
		}
	}
	r.Data = newObj
}

//Move move
func (r *Response) Move(source, target string) {

	if strings.Index(source, "*") != -1 {
		return
	}
	if strings.Index(target, "*") != -1 {
		return
	}
	root := _Node{
		data: r.Data,
	}
	var oldValues *_Node
	match, _ := root.Pattern(source, func(node *_Node) bool {
		oldValues = node

		if node.parent == nil {
			return false
		}
		parent := node.parent
		switch parent.data.(type) {
		case []interface{}:
			index := node.index
			sl := parent.data.([]interface{})

			nl := sl[:index]
			sl = append(nl, sl[index+1])
			parent.data = sl

		case map[string]interface{}:
			mp := parent.data.(map[string]interface{})
			delete(mp, node.key)
		}
		return false
	})
	if match {
		r.SetValue(target, oldValues.data)
	} else {
		r.SetValue(target, nil)
	}

}

//Array filter
func (r *Response) Filter(source, field, operator, target string) {

	if strings.Index(source, "*") != -1 {
		return
	}

	root := _Node{
		data: r.Data,
	}
	root.Pattern(source, func(node *_Node) bool {
		if node.parent == nil {
			return false
		}
		var selectedData []interface{}
		if _, ok := node.data.([]interface{}); !ok {
			return false
		}
		for _, item := range node.data.([]interface{}) {
			if _, ok := item.(map[string]interface{}); !ok {
				return false
			}
			v := item.(map[string]interface{})[field]
			if compare(v, operator, target) {
				selectedData = append(selectedData, item)
			}
		}
		node.data = selectedData
		parent := node.parent
		switch parent.data.(type) {
		case map[string]interface{}:
			mp := parent.data.(map[string]interface{})
			delete(mp, node.key)
			mp[node.key] = selectedData
		}
		return false
	})
}

func compare(source interface{}, operator, target string) bool {
	switch source.(type) {
	case int, int16, int32, int64:
		targetVal, err := strconv.ParseInt(target, 10, 64)
		if err != nil {
			return false
		}
		sourceVal := reflect.ValueOf(source).Int()
		switch operator {
		case "=":
			return sourceVal == targetVal
		case "<":
			return sourceVal < targetVal
		case ">":
			return sourceVal > targetVal
		}
	case uint, uint16, uint32, uint64:
		targetVal, err := strconv.ParseUint(target, 10, 64)
		if err != nil {
			return false
		}
		sourceVal := reflect.ValueOf(source).Uint()
		switch operator {
		case "=":
			return sourceVal == targetVal
		case "<":
			return sourceVal < targetVal
		case ">":
			return sourceVal > targetVal
		}
	case float32, float64:
		targetVal, err := strconv.ParseFloat(target, 64)
		if err != nil {
			return false
		}
		sourceVal := reflect.ValueOf(source).Float()
		switch operator {
		case "=":
			return sourceVal == targetVal
		case "<":
			return sourceVal < targetVal
		case ">":
			return sourceVal > targetVal
		}
	case string:
		targetVal := target
		sourceVal := reflect.ValueOf(source).String()
		switch operator {
		case "=":
			return sourceVal == targetVal
		case "<":
			return sourceVal < targetVal
		case ">":
			return sourceVal > targetVal
		}
	}
	return false
}

//Array filter
func (r *Response) Sort(source, field string) {

	if strings.Index(source, "*") != -1 {
		return
	}

	root := _Node{
		data: r.Data,
	}
	root.Pattern(source, func(node *_Node) bool {
		if node.parent == nil {
			return false
		}
		if _, ok := node.data.([]interface{}); !ok {
			return false
		}
		sortedArr := &sortedArray{
			Items: node.data.([]interface{}),
			Field: field,
		}
		sort.Sort(sortedArr)

		parent := node.parent
		switch parent.data.(type) {
		case map[string]interface{}:
			mp := parent.data.(map[string]interface{})
			delete(mp, node.key)
			mp[node.key] = sortedArr.Items
		}
		return false
	})
}

type sortedArray struct {
	Items []interface{}
	Field string
}

func (arr *sortedArray) Len() int {
	return len(arr.Items)
}
func (arr *sortedArray) Less(i, j int) bool {
	left := arr.Items[i]
	right := arr.Items[j]

	leftV := left.(map[string]interface{})[arr.Field]
	rightV := right.(map[string]interface{})[arr.Field]

	if reflect.TypeOf(leftV) != reflect.TypeOf(rightV) {
		return false
	}
	switch leftV.(type) {
	case int, int16, int32, int64:
		return reflect.ValueOf(leftV).Int() <= reflect.ValueOf(rightV).Int()
	case uint, uint16, uint32, uint64:
		return reflect.ValueOf(leftV).Uint() <= reflect.ValueOf(rightV).Uint()
	case float32, float64:
		return reflect.ValueOf(leftV).Float() <= reflect.ValueOf(rightV).Float()
	case string:
		return reflect.ValueOf(leftV).String() <= reflect.ValueOf(rightV).String()
	}

	return false
}
func (arr *sortedArray) Swap(i, j int) {
	arr.Items[i], arr.Items[j] = arr.Items[j], arr.Items[i]
}

func StringBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&sliceHeader{
		data: (*stringHeader)(unsafe.Pointer(&s)).data,
		len:  len(s),
		cap:  len(s),
	}))
}

// sliceHeader instead of reflect.SliceHeader
type sliceHeader struct {
	data unsafe.Pointer
	len  int
	cap  int
}

// stringHeader instead of reflect.StringHeader
type stringHeader struct {
	data unsafe.Pointer
	len  int
}
