package application

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unsafe"

	"github.com/eolinker/goku-api-gateway/config"
	log "github.com/eolinker/goku-api-gateway/goku-log"
	"github.com/eolinker/goku-api-gateway/goku-node/common"
	"github.com/eolinker/goku-api-gateway/node/gateway/application/backend"
	"github.com/eolinker/goku-api-gateway/node/gateway/application/interpreter"
	"github.com/eolinker/goku-api-gateway/node/gateway/response"
)

//LayerApplication layer application
type LayerApplication struct {
	output    response.Encoder
	backsides []*backend.Layer
	static    *staticeResponse

	timeOut time.Duration
}

//Execute execute
func (app *LayerApplication) Execute(ctx *common.Context) {
	orgBody, _ := ctx.ProxyRequest.RawBody()

	bodyObj, _ := ctx.ProxyRequest.BodyInterface()

	variables := interpreter.NewVariablesExt(orgBody, bodyObj, ctx.ProxyRequest.Headers(), ctx.ProxyRequest.Cookies(), ctx.RestfulParam, ctx.ProxyRequest.Querys(), len(app.backsides))

	deadline := context.Background()
	cancelFunc := context.CancelFunc(nil)
	app.timeOut = 0
	if app.timeOut > 0 {
		deadline, cancelFunc = context.WithDeadline(deadline, time.Now().Add(app.timeOut))
	} else {
		deadline, cancelFunc = context.WithCancel(deadline)
	}

	resC := make(chan int, 1)
	errC := make(chan error, 1)
	go app.doParallelly(deadline, variables, ctx, resC, errC)

	defer func() {
		close(resC)
		close(errC)
	}()

	select {
	case <-deadline.Done():
		ctx.SetStatus(503, "503")
		ctx.SetBody([]byte("[ERROR]timeout!"))
		// 超时
		return
	case e := <-errC:
		fmt.Println(e)
		cancelFunc()
		ctx.SetStatus(504, "504")
		ctx.SetBody([]byte("[ERROR]Fail to get response after proxy!"))
		//error
		return
	case <-resC:
		//response
		cancelFunc()
		break
	}

	mergeResponse, headers := variables.MergeResponse()

	//特殊处理，当最后一步JSON2Form=true，采用stringencoding编码
	var bs []byte
	if len(app.backsides) > 0 && app.backsides[len(app.backsides)-1].JSON2Form {
		app.output = response.GetEncoder(response.String)
		bs1, err := json2Form(mergeResponse)
		if err != nil {
			return
		}
		bs = bs1
	}

	body, e := app.output.Encode(mergeResponse, bs)
	if e != nil {
		log.Warn("encode response error:", e)
		return
	}
	//if headers.Get("Content-Encoding") == "gzip" {
	//	var b bytes.Buffer
	//	wb := gzip.NewWriter(&b)
	//	wb.Write(body)
	//	wb.Flush()
	//	body, _ = ioutil.ReadAll(&b)
	//}
	ctx.SetProxyResponseHandler(common.NewResponseReader(headers, 200, "200", body))

}

//json2Form ...
func json2Form(mergedResponse interface{}) ([]byte, error) {
	if _, ok := mergedResponse.(map[string]interface{}); !ok {
		bs, err := json.Marshal(mergedResponse)
		if err != nil {
			return nil, err
		}
		return bs, nil
	}
	var formDatas []string
	for key, val := range mergedResponse.(map[string]interface{}) {
		bs, err := json.Marshal(val)
		if err != nil {
			return nil, err
		}
		item := fmt.Sprintf("%s=%s", key, url.QueryEscape(*(*string)(unsafe.Pointer(&bs))))
		formDatas = append(formDatas, item)
	}
	return response.StringBytes(strings.Join(formDatas, "&")), nil
}

/* func (app *LayerApplication) do(ctxDeadline context.Context, variables *interpreter.Variables, ctx *common.Context, resC chan<- int, errC chan<- error) {

	l := len(app.backsides)
	for i, b := range app.backsides {

		if deadline, ok := ctxDeadline.Deadline(); ok {
			if time.Now().After(deadline) {
				// 超时
				log.Warn("time out before send step:", i, "/", l)
				return
			}
		}
		r, err := b.Send(ctxDeadline, ctx, variables)

		if deadline, ok := ctxDeadline.Deadline(); ok {
			if time.Now().After(deadline) {
				// 超时
				log.Warn("time out before send step:", i+1, "/", l)
				return
			}
		}
		if err != nil {
			errC <- err
			log.Warn("error by send step:", i+1, "/", l, "\t:", err)
			return
		}
		variables.AppendResponse(r.Header, r.Body)
	}
	if deadline, ok := ctxDeadline.Deadline(); ok {
		if time.Now().After(deadline) {
			// 超时
			log.Warn("time out before send step:", l, "/", l)
			return
		}
	}
	resC <- 1

} */

func (app *LayerApplication) doParallelly(ctxDeadline context.Context, variables *interpreter.Variables, ctx *common.Context, resC chan<- int, errC chan<- error) {
	l := len(app.backsides)
	if l > 0 {
		if ctx.MultiStepLocks == nil {
			ctx.MultiStepLocks = make(map[string]*common.ContextStepLock)
		}
		for _, bs := range app.backsides {
			ctx.MultiStepLocks[bs.Name] = new(common.ContextStepLock)
		}

		_, err := app.backsides[l-1].SendParallelly(ctxDeadline, ctx, variables)

		if err != nil {
			errC <- err
			log.Warn("error by send parallelly.", "\t:", err)
			return
		}
	}

	resC <- 1

}

func (app *LayerApplication) getParentLayers(pNames []string) []*backend.Layer {
	var result []*backend.Layer
	for _, pname := range pNames {
		for _, item := range app.backsides {
			if item.Name == pname {
				result = append(result, item)
			}
		}
	}
	return result
}

//index start from 1, following variables's body array
func (app *LayerApplication) fillLayerRelationship() {
	for idx, bs := range app.backsides {
		bs.Index = idx + 1
		bs.ParentLayers = app.getParentLayers(bs.ParentNames)
	}
}

//NewLayerApplication create new layer application
func NewLayerApplication(apiContent *config.APIContent) *LayerApplication {
	app := &LayerApplication{
		output:    response.GetEncoder(apiContent.OutPutEncoder),
		backsides: make([]*backend.Layer, 0, len(apiContent.Steps)),
		static:    nil,
		timeOut:   time.Duration(apiContent.TimeOutTotal) * time.Millisecond,
	}

	for _, step := range apiContent.Steps {
		app.backsides = append(app.backsides, backend.NewLayer(step))
	}

	//fill all layer releationship
	app.fillLayerRelationship()

	if apiContent.StaticResponse != "" {
		staticResponseStrategy := config.Parse(apiContent.StaticResponseStrategy)
		app.static = newStaticeResponse(apiContent.StaticResponse, staticResponseStrategy)
	}
	return app
}
