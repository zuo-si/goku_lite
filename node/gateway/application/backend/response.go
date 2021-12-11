package backend

import "net/http"

//BackendResponse 后端响应
type BackendResponse struct {
	StepName           string
	StepIndex          int
	Method             string
	Protocol           string
	TargetURL          string
	FinalTargetServer  string
	RetryTargetServers []string
	BodyOrg            []byte
	Header             http.Header
	Body               interface{}
	StatusCode         int
	Status             string
}
