
/*
 Copyright (c) Huawei Technologies Co., Ltd. 2021-2021. All rights reserved.
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/
// Package client to restful client to access enterprise storage
package client
import (
	"context"
	"strconv"

	"huawei-csi-driver/utils"
)
func (cli *Client) getResponseDataMap(ctx context.Context, data interface{}) (map[string]interface{}, error) {
	respData, ok := data.(map[string]interface{})
	if !ok {
		return nil, utils.Errorf(ctx,"the response data is not a map[string]interface{}")
	}
	return respData, nil
}

func (cli *Client) getResponseDataList(ctx context.Context, data interface{}) ([]interface{}, error) {
	respData, ok := data.([]interface{})
	if !ok {
		return nil, utils.Errorf(ctx,"the response data is not a []interface{}")
	}
	return respData, nil
}

func (cli *Client) getCountFromResponse(ctx context.Context, data interface{}) (int64, error) {
	respData, err := cli.getResponseDataMap(ctx, data)
	if err != nil {
		return 0, err
	}
	countStr, ok := respData["COUNT"].(string)
	if !ok  {
		return 0, utils.Errorf(ctx,"The COUNT is not in respData %v", respData)
	}
	count, err := strconv.ParseInt(countStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (cli *Client) getSystemUTCTime(ctx context.Context) (int64, error) {
	resp, err := cli.get(ctx,"/system_utc_time", nil)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return 0, utils.Errorf(ctx,"get system UTC time error: %d", code)
	}

	if resp.Data == nil {
		return 0, utils.Errorf(ctx,"can not get the system UTC time")
	}

	respData, err := cli.getResponseDataMap(ctx, resp.Data)
	if err != nil {
		return 0, err
	}

	utcTime, ok := respData["CMO_SYS_UTC_TIME"].(string)
	if !ok {
		return 0, utils.Errorf(ctx, "The CMO_SYS_UTC_TIME is not in respData %v", respData)
	}

	time, err := strconv.ParseInt(utcTime, 10, 64)
	if err != nil {
		return 0, err
	}
	return time, nil
}
