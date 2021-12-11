package interpreter

import (
	"net/http"
	"net/url"
	"strings"
)

type _Cookies []*http.Cookie

//Variables variables
type Variables struct {
	Org     []byte
	Bodes   []interface{}
	Headers []http.Header
	Cookies []_Cookies
	Restful map[string]string
	Query   url.Values
}

//MergeResponse mergeResponse
func (v *Variables) MergeResponse() (interface{}, http.Header) {

	body := MergeBodys(v.Bodes[1:])

	header := MergeHeaders(v.Headers[1:])

	cookies := MergeCookies(v.Cookies[1:])

	// 把cookie加回header中
	rt := &http.Request{Header: header}
	for _, c := range cookies {
		rt.AddCookie(c)
	}

	return body, header
}

//MergeResponseExt mergeResponse
func (v *Variables) MergeResponseExt() (interface{}, http.Header) {

	body := v.Bodes[len(v.Bodes)-1]

	header := MergeHeaders(v.Headers[1:])

	cookies := MergeCookies(v.Cookies[1:])

	// 把cookie加回header中
	rt := &http.Request{Header: header}
	for _, c := range cookies {
		rt.AddCookie(c)
	}

	return body, header
}

//NewVariables newVariables
func NewVariables(org []byte, body interface{}, header http.Header, cookie []*http.Cookie, restful map[string]string, query url.Values, size int) *Variables {
	max := size + 1
	bodes := make([]interface{}, 0, max)
	headers := make([]http.Header, 0, max)
	cookies := make([]_Cookies, 0, max)

	v := &Variables{
		Org:     org,
		Bodes:   append(bodes, body),
		Headers: append(headers, header),
		Restful: restful,
		Query:   query,
	}
	v.Cookies = append(cookies, cookie)
	// 暂时先删除掉cookie
	header.Del("Cookie")

	return v
}

//AppendResponse appendResponse
/* func (v *Variables) AppendResponse(header http.Header, body interface{}) {
	v.Headers = append(v.Headers, header)
	v.Bodes = append(v.Bodes, body)
	req := http.Request{Header: header}
	v.Cookies = append(v.Cookies, _Cookies(req.Cookies()))
	// 暂时先删除掉cookie
	header.Del("Cookie")
} */

func NewVariablesExt(org []byte, body interface{}, header http.Header, cookie []*http.Cookie, restful map[string]string, query url.Values, size int) *Variables {
	max := size + 1
	bodes := make([]interface{}, max, max)
	headers := make([]http.Header, max, max)
	cookies := make([]_Cookies, max, max)

	v := &Variables{
		Org:     org,
		Bodes:   bodes,
		Headers: headers,
		Restful: restful,
		Query:   query,
		Cookies: cookies,
	}
	v.Bodes[0] = body
	v.Headers[0] = header
	v.Cookies[0] = cookie

	// 暂时先删除掉cookie
	header.Del("Cookie")

	return v
}

func (v *Variables) AppendResponseExt(index int, header http.Header, body interface{}) {
	v.Headers[index] = header
	v.Bodes[index] = body
	req := http.Request{Header: header}
	v.Cookies[index] = _Cookies(req.Cookies())
	// 暂时先删除掉cookie
	header.Del("Cookie")
}

//Interpreter interpreter
type Interpreter interface {
	Execution(value *Variables) string
}

type _Executor []Reader

//Execution execution
func (exe _Executor) Execution(value *Variables) string {

	switch len(exe) {
	case 0:
		return ""
	case 1:
		return exe[0].Read(value)
	}
	builder := strings.Builder{}

	for _, r := range exe {
		builder.WriteString(r.Read(value))
	}
	return builder.String()

}
