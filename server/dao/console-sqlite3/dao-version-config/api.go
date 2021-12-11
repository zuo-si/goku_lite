package dao_version_config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/eolinker/goku-api-gateway/config"
)

//GetAPIContent 获取接口信息
func (d *VersionConfigDao) GetAPIContent() ([]*config.APIContent, error) {
	db := d.db
	sql := "SELECT apiID,apiName,IFNULL(protocol,'http'),IFNULL(balanceName,''),IFNULL(targetURL,''),CASE WHEN isFollow = 'true' THEN 'FOLLOW' ELSE targetMethod END targetMethod,responseDataType,requestURL,requestMethod,timeout,alertValve,retryCount,IFNULL(linkApis,''),IFNULL(staticResponse,'') FROM goku_gateway_api"
	rows, err := db.Query(sql)
	if err != nil {
		return nil, err
	}
	apiContents := make([]*config.APIContent, 0, 100)
	defer rows.Close()
	for rows.Next() {
		var apiContent config.APIContent
		var linkApisStr, protocol, balance, targetURL, targetMethod, requestMethod string
		var retryCount int
		linkApis := make([]config.APIStepUIConfig, 0)
		err = rows.Scan(&apiContent.ID, &apiContent.Name, &protocol, &balance, &targetURL, &targetMethod, &apiContent.OutPutEncoder, &apiContent.RequestURL, &requestMethod, &apiContent.TimeOutTotal, &apiContent.AlertThreshold, &retryCount, &linkApisStr, &apiContent.StaticResponse)
		if err != nil {
			return nil, err
		}
		if linkApisStr != "" {
			err = json.Unmarshal([]byte(linkApisStr), &linkApis)
			if err != nil {
				return nil, err
			}
		}

		apiContent.Methods = strings.Split(requestMethod, ",")
		if len(linkApis) < 1 {
			apiContent.Steps = append(apiContent.Steps, &config.APIStepConfig{
				Name:        "step1",
				ParentNames: []string{},
				Proto:       protocol,
				Balance:     balance,
				Path:        targetURL,
				Method:      targetMethod,
				Encode:      "origin",
				Decode:      apiContent.OutPutEncoder,
				TimeOut:     apiContent.TimeOutTotal,
				Retry:       retryCount,
			})
		} else {
			for _, api := range linkApis {
				actions := make([]*config.ActionConfig, 0, 20)
				for _, del := range api.Delete {
					actions = append(actions, &config.ActionConfig{
						ActionType: "delete",
						Original:   del.Origin,
					})
				}
				for _, move := range api.Move {
					actions = append(actions, &config.ActionConfig{
						ActionType: "move",
						Original:   move.Origin,
						Target:     move.Target,
					})
				}
				for _, rename := range api.Rename {
					actions = append(actions, &config.ActionConfig{
						ActionType: "rename",
						Original:   rename.Origin,
						Target:     rename.Target,
					})
				}

				for _, arrayFilter := range api.ArrayFilter {
					if arrayFilter.Origin == "" || arrayFilter.Field == "" || arrayFilter.Operator == "" || arrayFilter.Operand == "" {
						continue
					}
					actions = append(actions, &config.ActionConfig{
						ActionType: "arrayfilter",
						Original:   arrayFilter.Origin,
						Field:      arrayFilter.Field,
						Operator:   arrayFilter.Operator,
						Operand:    arrayFilter.Operand,
					})
				}

				for _, arraySort := range api.ArraySort {
					if arraySort.Origin == "" || arraySort.Field == "" {
						continue
					}
					actions = append(actions, &config.ActionConfig{
						ActionType: "arraysort",
						Original:   arraySort.Origin,
						Field:      arraySort.Field,
					})
				}

				//针对whiteliststr特殊处理
				if api.WhiteListStr != "" {
					api.WhiteList = strings.Split(api.WhiteListStr, ",")
				}
				//针对parentnames特殊处理
				if api.ParentNamesStr != "" {
					api.ParentNames = strings.Split(api.ParentNamesStr, ",")
				}

				apiContent.Steps = append(apiContent.Steps, &config.APIStepConfig{
					Name:          api.Name,
					ParentNames:   api.ParentNames,
					Proto:         api.Proto,
					Balance:       api.Balance,
					Path:          api.Path,
					Body:          api.Body,
					Method:        api.Method,
					Encode:        api.Encode,
					Decode:        api.Decode,
					TimeOut:       api.TimeOut,
					Retry:         api.Retry,
					FirstKeyUpper: api.FirstKeyUpper,
					JSON2Form:     api.JSON2Form,
					Group:         api.Group,
					Target:        api.Target,
					WhiteList:     api.WhiteList,
					BlackList:     api.BlackList,
					Actions:       actions,
				})
			}
		}

		apiContents = append(apiContents, &apiContent)

	}

	bytes, _ := json.Marshal(apiContents)
	fmt.Println("apicontents: ", string(bytes))

	return apiContents, nil
}
