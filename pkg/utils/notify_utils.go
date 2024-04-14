/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package utils

import (
	"context"
	"reflect"
	"runtime/debug"

	"huawei-csi-driver/utils/log"
)

const (
	// BackendStatus backend status topic
	BackendStatus = "BACKEND_STATUS"
)

var notifyMap map[string]interface{}

func init() {
	notifyMap = map[string]interface{}{}
}

// Subscribe subscribe topic
func Subscribe(topic string, fn interface{}) {
	if notifyMap == nil {
		notifyMap = map[string]interface{}{}
	}
	notifyMap[topic] = fn
}

// Publish event to subscriber
func Publish(ctx context.Context, key string, args ...interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.AddContext(ctx).Errorf("Runtime error caught in Publish: %v", r)
			log.AddContext(ctx).Errorf("%s", debug.Stack())
		}
	}()
	if callBack, ok := notifyMap[key]; ok {
		in := make([]reflect.Value, len(args))
		for i, arg := range args {
			in[i] = reflect.ValueOf(arg)
		}
		reflect.ValueOf(callBack).Call(in)
	} else {
		log.AddContext(ctx).Infof("%s does not have any subscribers")
	}
}
