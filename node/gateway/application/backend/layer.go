package backend

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/eolinker/goku-api-gateway/config"
	log "github.com/eolinker/goku-api-gateway/goku-log"
	"github.com/eolinker/goku-api-gateway/goku-node/common"
	"github.com/eolinker/goku-api-gateway/goku-service/application"
	"github.com/eolinker/goku-api-gateway/goku-service/balance"
	"github.com/eolinker/goku-api-gateway/node/gateway/application/action"
	"github.com/eolinker/goku-api-gateway/node/gateway/application/interpreter"
	"github.com/eolinker/goku-api-gateway/node/gateway/response"
)

//Layer layer
type Layer struct {
	Name         string     //support parallel
	ParentNames  []string   //support parallel
	ParentLayers []*Layer   //support parallel
	Index        int        //support parallel
	Lock         sync.Mutex //support parallel
	BalanceName  string
	Balance      application.IHttpApplication
	HasBalance   bool
	Protocol     string

	Filter action.Filter
	Method string
	Path   interpreter.Interpreter
	Decode response.DecodeHandle

	Body          interpreter.Interpreter
	Encode        string
	Target        string
	FirstKeyUpper bool
	JSON2Form     bool
	Group         []string
	Retry         int
	TimeOut       time.Duration
}

//Send send
/* func (b *Layer) Send(deadline context.Context, ctx *common.Context, variables *interpreter.Variables) (*BackendResponse, error) {
	path := b.Path.Execution(variables)
	body := b.Body.Execution(variables)
	method := b.Method
	if method == "FOLLOW" {
		method = ctx.ProxyRequest.Method
	}
	header := ctx.ProxyRequest.Headers()
	if b.Encode == "json" {
		header.Set("content-type", "application/json; charset=utf-8")
	} else if b.Encode == "xml" {
		header.Set("content-type", "application/xml; charset=utf-8")
	}

	r, finalTargetServer, retryTargetServers, err := b.Balance.Send(ctx, b.Protocol, method, path, nil, header, []byte(body), b.TimeOut, b.Retry)

	if err != nil {
		return nil, err
	}
	backendResponse := &BackendResponse{
		Method:   strings.ToUpper(method),
		Protocol: b.Protocol,
		//Response:           r,
		TargetURL:          path,
		FinalTargetServer:  finalTargetServer,
		RetryTargetServers: retryTargetServers,
		Header:             r.Header,
	}

	defer r.Body.Close()
	bd := r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		bd, _ = gzip.NewReader(r.Body)
		r.Header.Del("Content-Encoding")
	}

	backendResponse.BodyOrg, err = ioutil.ReadAll(bd)
	if err != nil {
		return backendResponse, nil
	}

	rp, e := response.Decode(backendResponse.BodyOrg, b.Decode)
	if e != nil {
		backendResponse.Body = nil
		return nil, e
	}

	b.Filter.Do(rp)

	if b.Target != "" {
		rp.ReTarget(b.Target)
	}
	if len(b.Group) > 0 {
		rp.Group(b.Group)
	}

	backendResponse.Body = rp.Data
	return backendResponse, nil
} */

//NewLayer newLayer
func NewLayer(step *config.APIStepConfig) *Layer {
	var b = &Layer{
		Name:          step.Name,
		ParentNames:   step.ParentNames,
		BalanceName:   step.Balance,
		Balance:       nil,
		HasBalance:    false,
		Protocol:      step.Proto,
		Filter:        genFilter(step.BlackList, step.WhiteList, step.Actions),
		Method:        strings.ToUpper(step.Method),
		Path:          interpreter.GenPath(step.Path),
		Decode:        response.GetDecoder(step.Decode),
		Encode:        step.Encode,
		Target:        step.Target,
		Group:         nil,
		FirstKeyUpper: step.FirstKeyUpper,
		JSON2Form:     step.JSON2Form,
		TimeOut:       time.Duration(step.TimeOut) * time.Millisecond,
		Body:          interpreter.Gen(step.Body, step.Encode),
		Retry:         step.Retry,
	}
	if step.Group != "" {
		b.Group = strings.Split(step.Group, ".")
	}

	b.Balance, b.HasBalance = balance.GetByName(b.BalanceName)

	return b
}

type httpResponseCacheObject struct {
	Header             http.Header
	StatusCode         int
	Status             string
	FinalTargetServer  string
	RetryTargetServers []string
	BodyOrg            []byte
}

//Send send parallelly
func (b *Layer) SendParallelly(deadline context.Context, ctx *common.Context, variables *interpreter.Variables) (*BackendResponse, error) {
	parentBacksides := b.ParentLayers
	length := len(parentBacksides)

	if length > 0 {
		var wg sync.WaitGroup
		errChan := make(chan error, length)
		wg.Add(length)
		for i := 0; i < length; i++ {
			go func(errChan chan<- error, sb *Layer) {
				defer wg.Done()
				_, err := sb.SendParallelly(deadline, ctx, variables)
				if err != nil {
					errChan <- err
				}
			}(errChan, parentBacksides[i])
		}
		wg.Wait()
		select {
		case err := <-errChan:
			return nil, err
		default:
		}
		close(errChan)
	}

	defer ctx.MultiStepLocks[b.Name].Unlock()
	ctx.MultiStepLocks[b.Name].Lock()

	if variables.Bodes[b.Index] != nil {
		return &BackendResponse{
			StepName:  b.Name,
			StepIndex: b.Index,
			Header:    variables.Headers[b.Index],
			Body:      variables.Bodes[b.Index],
		}, nil
	}

	if deadline, ok := deadline.Deadline(); ok {
		if time.Now().After(deadline) {
			// 超时
			log.Warn("time out before send step:", b.Name)
			return nil, errors.New("timeout")
		}
	}

	path := b.Path.Execution(variables)
	body := b.Body.Execution(variables)
	method := b.Method
	if method == "FOLLOW" {
		method = ctx.ProxyRequest.Method
	}
	header := ctx.ProxyRequest.Headers()
	if b.Encode == "json" {
		header.Set("content-type", "application/json; charset=utf-8")
	} else if b.Encode == "xml" {
		header.Set("content-type", "application/xml; charset=utf-8")
	}

	backendResponse := &BackendResponse{
		StepName:  b.Name,
		StepIndex: b.Index,
		Method:    strings.ToUpper(method),
		Protocol:  b.Protocol,
		//Response:           r,
		TargetURL: path,
	}

	//针对GET请求，增加Cache处理
	var cacheObject httpResponseCacheObject
	var cacheFound bool
	if strings.ToLower(method) == "get" && len(body) == 0 && header.Get("x-cache-disabled") != "true" {
		cachedKey := fmt.Sprintf("%s://%s", b.Protocol, path)
		cachedVal, found := getCache(cachedKey)
		if found {
			cacheFound = true
			cacheObject = cachedVal.(httpResponseCacheObject)

			backendResponse.FinalTargetServer = cacheObject.FinalTargetServer
			backendResponse.RetryTargetServers = make([]string, len(cacheObject.RetryTargetServers))
			copy(backendResponse.RetryTargetServers, cacheObject.RetryTargetServers)
			backendResponse.Header = cacheObject.Header.Clone()
			backendResponse.Status = cacheObject.Status
			backendResponse.StatusCode = cacheObject.StatusCode
			backendResponse.BodyOrg = make([]byte, len(cacheObject.BodyOrg))
			copy(backendResponse.BodyOrg, cacheObject.BodyOrg)
		}
	}
	if !cacheFound {
		r, finalTargetServer, retryTargetServers, err := b.Balance.Send(ctx, b.Protocol, method, path, nil, header, response.StringBytes(body), b.TimeOut, b.Retry)

		if err != nil {
			return nil, err
		}

		if deadline, ok := deadline.Deadline(); ok {
			if time.Now().After(deadline) {
				// 超时
				log.Warn("time out after send step:", b.Name)
				return nil, errors.New("timeout")
			}
		}

		if r.StatusCode >= 300 || r.StatusCode < 200 {
			return nil, errors.New(r.Status)
		}

		backendResponse.FinalTargetServer = finalTargetServer
		backendResponse.RetryTargetServers = retryTargetServers
		backendResponse.Header = r.Header
		backendResponse.Status = r.Status
		backendResponse.StatusCode = r.StatusCode

		defer r.Body.Close()
		bd := r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			bd, _ = gzip.NewReader(r.Body)
			r.Header.Del("Content-Encoding")
		}

		backendResponse.BodyOrg, err = ioutil.ReadAll(bd)
		if err != nil {
			return backendResponse, nil
		}

		if strings.ToLower(method) == "get" && len(body) == 0 && header.Get("x-cache-disabled") != "true" {
			cachedKey := fmt.Sprintf("%s://%s", b.Protocol, path)
			cacheObject = httpResponseCacheObject{
				FinalTargetServer: finalTargetServer,
				Header:            r.Header.Clone(),
				Status:            r.Status,
				StatusCode:        r.StatusCode,
			}
			cacheObject.RetryTargetServers = make([]string, len(retryTargetServers))
			copy(cacheObject.RetryTargetServers, retryTargetServers)
			cacheObject.BodyOrg = make([]byte, len(backendResponse.BodyOrg))
			copy(cacheObject.BodyOrg, backendResponse.BodyOrg)
			setCache(cachedKey, cacheObject)
		}
	}

	rp, e := response.Decode(backendResponse.BodyOrg, b.Decode)
	if e != nil {
		backendResponse.Body = nil
		return nil, e
	}

	b.Filter.Do(rp)

	if b.Target != "" {
		rp.ReTarget(b.Target)
	}
	if len(b.Group) > 0 {
		rp.Group(b.Group)
	}

	if b.FirstKeyUpper {
		rp.FirstKeyUpper()
	}

	backendResponse.Body = rp.Data

	variables.AppendResponseExt(backendResponse.StepIndex, backendResponse.Header, backendResponse.Body)

	return backendResponse, nil
}
