// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2020-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/http/utils"

	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
	sdkCommon "github.com/edgexfoundry/south-sdk-go/v4/internal/common"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/container"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/transformer"
	sdkModels "github.com/edgexfoundry/south-sdk-go/v4/pkg/models"

	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
)

func GetCommand(ctx context.Context, nodeName string, commandName string, queryParams string, regexCmd bool, dic *di.Container) (res *dtos.Event, err errors.EdgeX) {
	if nodeName == "" {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, "node name is empty", nil)
	}
	if commandName == "" {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, "command is empty", nil)
	}
	var node models.Node
	defer func() {
		if err != nil {
			NodeRequestFailed(nodeName, dic)
		} else {
			NodeRequestSucceeded(node, dic)
		}
	}()

	node, err = validateServiceAndNodeState(nodeName, dic)
	if err != nil {
		return nil, errors.NewCommonEdgeXWrapper(err)
	}

	_, cmdExist := cache.Profiles().Command(node.ProfileName, commandName)
	if cmdExist {
		res, err = readCommand(node, commandName, queryParams, dic)
	} else if regexCmd {
		res, err = readResourcesRegex(node, commandName, queryParams, dic)
	} else {
		res, err = readResource(node, commandName, queryParams, dic)
	}

	if err != nil {
		return nil, errors.NewCommonEdgeXWrapper(err)
	}

	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	lc.Debugf("GET Node Command successfully. Node: %s, Source: %s, %s: %s", nodeName, commandName, common.CorrelationHeader, utils.FromContext(ctx, common.CorrelationHeader))

	cache.Nodes().SetLastConnectedByName(nodeName)
	return res, nil
}

func SetCommand(ctx context.Context, nodeName string, commandName string, queryParams string, requests map[string]any, dic *di.Container) (event *dtos.Event, err errors.EdgeX) {
	if nodeName == "" {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, "node name is empty", nil)
	}
	if commandName == "" {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, "command is empty", nil)
	}
	var node models.Node
	defer func() {
		if err != nil {
			NodeRequestFailed(nodeName, dic)
		} else {
			NodeRequestSucceeded(node, dic)
		}
	}()

	node, err = validateServiceAndNodeState(nodeName, dic)
	if err != nil {
		return nil, errors.NewCommonEdgeXWrapper(err)
	}

	_, cmdExist := cache.Profiles().Command(node.ProfileName, commandName)
	if cmdExist {
		event, err = writeCommand(node, commandName, queryParams, requests, dic)
	} else {
		event, err = writeResource(node, commandName, queryParams, requests, dic)
	}

	if err != nil {
		return nil, errors.NewCommonEdgeXWrapper(err)
	}

	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	lc.Debugf("SET Node Command successfully. Node: %s, Source: %s, %s: %s", nodeName, commandName, common.CorrelationHeader, utils.FromContext(ctx, common.CorrelationHeader))

	cache.Nodes().SetLastConnectedByName(nodeName)
	return event, nil
}

func readResource(node models.Node, resourceName string, attributes string, dic *di.Container) (*dtos.Event, errors.EdgeX) {
	dr, ok := cache.Profiles().Resource(node.ProfileName, resourceName)
	if !ok {
		errMsg := fmt.Sprintf("Resource %s not found", resourceName)
		return nil, errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
	}
	// check resource is not write-only
	if dr.Properties.ReadWrite == common.ReadWrite_W {
		errMsg := fmt.Sprintf("Resource %s is marked as write-only", dr.Name)
		return nil, errors.NewCommonEdgeX(errors.KindNotAllowed, errMsg, nil)
	}

	var req sdkModels.CommandRequest
	var reqs []sdkModels.CommandRequest

	// prepare CommandRequest
	req.ResourceName = dr.Name
	req.Attributes = dr.Attributes
	if attributes != "" {
		if len(req.Attributes) <= 0 {
			req.Attributes = make(map[string]interface{})
		}
		req.Attributes[sdkCommon.URLRawQuery] = attributes
	}
	req.Type = dr.Properties.ValueType
	reqs = append(reqs, req)

	// execute protocol-specific read operation
	driver := container.ProtocolDriverFrom(dic.Get)
	results, err := driver.HandleReadCommands(node.Name, node.Protocols, reqs)
	if err != nil {
		errMsg := fmt.Sprintf("error reading Resource %s for %s", dr.Name, node.Name)
		return nil, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
	}

	// convert CommandValue to Event
	configuration := container.ConfigurationFrom(dic.Get)
	event, err := transformer.CommandValuesToEventDTO(results, node.Name, dr.Name, configuration.Node.DataTransform, dic)
	if err != nil {
		return nil, errors.NewCommonEdgeX(errors.KindServerError, "failed to convert CommandValue to Event", err)
	}

	return event, nil
}

func readResourcesRegex(node models.Node, regexResourceName string, attributes string, dic *di.Container) (*dtos.Event, errors.EdgeX) {
	regex, err := regexp.CompilePOSIX(regexResourceName)
	if err != nil {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, "failed to CompilePOSIX resource name", err)
	}

	resources, ok := cache.Profiles().ResourcesByRegex(node.ProfileName, regex)
	if !ok || len(resources) == 0 {
		errMsg := fmt.Sprintf("Regex Resource %s not found", regexResourceName)
		return nil, errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
	}

	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	reqs := make([]sdkModels.CommandRequest, 0)
	for _, dr := range resources {
		// check resource is not write-only
		if dr.Properties.ReadWrite == common.ReadWrite_W {
			lc.Debugf("Resource %s is marked as write-only, skipping adding to RegEx Read list", dr.Name)
			continue
		}

		// prepare CommandRequest
		var req sdkModels.CommandRequest
		req.ResourceName = dr.Name
		req.Attributes = dr.Attributes
		if attributes != "" {
			if len(req.Attributes) <= 0 {
				req.Attributes = make(map[string]any)
			}
			req.Attributes[sdkCommon.URLRawQuery] = attributes
		}
		req.Type = dr.Properties.ValueType

		reqs = append(reqs, req)
	}

	if len(reqs) == 0 {
		errMsg := fmt.Sprintf("no readable resources matched with %s", regexResourceName)
		return nil, errors.NewCommonEdgeX(errors.KindNotAllowed, errMsg, nil)
	}

	// execute protocol-specific read operation
	driver := container.ProtocolDriverFrom(dic.Get)
	results, err := driver.HandleReadCommands(node.Name, node.Protocols, reqs)
	if err != nil {
		errMsg := fmt.Sprintf("error reading Regex Resource(s) %s for %s", regexResourceName, node.Name)
		return nil, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
	}

	// convert CommandValue to Event
	configuration := container.ConfigurationFrom(dic.Get)
	event, err := transformer.CommandValuesToEventDTO(results, node.Name, regexResourceName, configuration.Node.DataTransform, dic)
	if err != nil {
		return nil, errors.NewCommonEdgeX(errors.KindServerError, "failed to convert CommandValue to Event", err)
	}

	return event, nil
}

func readCommand(node models.Node, commandName string, attributes string, dic *di.Container) (*dtos.Event, errors.EdgeX) {
	dc, ok := cache.Profiles().Command(node.ProfileName, commandName)
	if !ok {
		errMsg := fmt.Sprintf("Command %s not found", commandName)
		return nil, errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
	}
	// check command is not write-only
	if dc.ReadWrite == common.ReadWrite_W {
		errMsg := fmt.Sprintf("Command %s is marked as write-only", dc.Name)
		return nil, errors.NewCommonEdgeX(errors.KindNotAllowed, errMsg, nil)
	}
	// check ResourceOperation count does not exceed MaxCmdOps defined in configuration
	configuration := container.ConfigurationFrom(dic.Get)
	if len(dc.ResourceOperations) > configuration.Node.MaxCmdOps {
		errMsg := fmt.Sprintf("GET command %s exceed node %s MaxCmdOps (%d)", dc.Name, node.Name, configuration.Node.MaxCmdOps)
		return nil, errors.NewCommonEdgeX(errors.KindServerError, errMsg, nil)
	}

	// prepare CommandRequests
	reqs := make([]sdkModels.CommandRequest, len(dc.ResourceOperations))
	for i, op := range dc.ResourceOperations {
		drName := op.Resource
		// check the resource in ResourceOperation actually exist
		dr, ok := cache.Profiles().Resource(node.ProfileName, drName)
		if !ok {
			errMsg := fmt.Sprintf("Resource %s in GET commnd %s for %s not defined", drName, dc.Name, node.Name)
			return nil, errors.NewCommonEdgeX(errors.KindServerError, errMsg, nil)
		}

		reqs[i].ResourceName = dr.Name
		reqs[i].Attributes = dr.Attributes
		if attributes != "" {
			if len(reqs[i].Attributes) <= 0 {
				reqs[i].Attributes = make(map[string]interface{})
			}
			reqs[i].Attributes[sdkCommon.URLRawQuery] = attributes
		}
		reqs[i].Type = dr.Properties.ValueType
	}

	// execute protocol-specific read operation
	driver := container.ProtocolDriverFrom(dic.Get)
	results, err := driver.HandleReadCommands(node.Name, node.Protocols, reqs)
	if err != nil {
		errMsg := fmt.Sprintf("error reading Command %s for %s", dc.Name, node.Name)
		return nil, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
	}

	// convert CommandValue to Event
	event, err := transformer.CommandValuesToEventDTO(results, node.Name, dc.Name, configuration.Node.DataTransform, dic)
	if err != nil {
		return nil, errors.NewCommonEdgeX(errors.KindServerError, "failed to transform CommandValue to Event", err)
	}

	return event, nil
}

func writeResource(node models.Node, resourceName string, attributes string, requests map[string]any, dic *di.Container) (*dtos.Event, errors.EdgeX) {
	dr, ok := cache.Profiles().Resource(node.ProfileName, resourceName)
	if !ok {
		errMsg := fmt.Sprintf("Resource %s not found", resourceName)
		return nil, errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
	}
	// check resource is not read-only
	if dr.Properties.ReadWrite == common.ReadWrite_R {
		errMsg := fmt.Sprintf("Resource %s is marked as read-only", dr.Name)
		return nil, errors.NewCommonEdgeX(errors.KindNotAllowed, errMsg, nil)
	}

	// check set parameters contains provided resource
	v, ok := requests[dr.Name]
	if !ok {
		if dr.Properties.DefaultValue != "" {
			v = dr.Properties.DefaultValue
		} else {
			errMsg := fmt.Sprintf("Resource %s not found in request body and no default value defined", dr.Name)
			return nil, errors.NewCommonEdgeX(errors.KindServerError, errMsg, nil)
		}
	}

	// create CommandValue
	cv, edgexErr := createCommandValueFromResource(dr, v)
	if edgexErr != nil {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, "failed to create CommandValue", edgexErr)
	}

	// prepare CommandRequest
	reqs := make([]sdkModels.CommandRequest, 1)
	reqs[0].ResourceName = cv.ResourceName
	reqs[0].Attributes = dr.Attributes
	if attributes != "" {
		if len(reqs[0].Attributes) <= 0 {
			reqs[0].Attributes = make(map[string]any)
		}
		reqs[0].Attributes[sdkCommon.URLRawQuery] = attributes
	}
	reqs[0].Type = cv.Type

	// transform write value
	configuration := container.ConfigurationFrom(dic.Get)
	if configuration.Node.DataTransform {
		edgexErr = transformer.TransformWriteParameter(cv, dr.Properties)
		if edgexErr != nil {
			return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, "failed to transform set parameter", edgexErr)
		}
	}

	// execute protocol-specific write operation
	driver := container.ProtocolDriverFrom(dic.Get)
	err := driver.HandleWriteCommands(node.Name, node.Protocols, reqs, []*sdkModels.CommandValue{cv})
	if err != nil {
		errMsg := fmt.Sprintf("error writing Resource %s for %s", dr.Name, node.Name)
		return nil, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
	}

	// Updated resource value will be published to MessageBus as long as it's not write-only
	if dr.Properties.ReadWrite != common.ReadWrite_W {
		return transformer.CommandValuesToEventDTO([]*sdkModels.CommandValue{cv}, node.Name, resourceName, configuration.Node.DataTransform, dic)
	}

	return nil, nil
}

func writeCommand(node models.Node, commandName string, attributes string, requests map[string]any, dic *di.Container) (*dtos.Event, errors.EdgeX) {
	dc, ok := cache.Profiles().Command(node.ProfileName, commandName)
	if !ok {
		errMsg := fmt.Sprintf("Command %s not found", commandName)
		return nil, errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
	}
	// check command is not read-only
	if dc.ReadWrite == common.ReadWrite_R {
		errMsg := fmt.Sprintf("Command %s is marked as read-only", dc.Name)
		return nil, errors.NewCommonEdgeX(errors.KindNotAllowed, errMsg, nil)
	}
	// check ResourceOperation count does not exceed MaxCmdOps defined in configuration
	configuration := container.ConfigurationFrom(dic.Get)
	if len(dc.ResourceOperations) > configuration.Node.MaxCmdOps {
		errMsg := fmt.Sprintf("SET command %s exceed node %s MaxCmdOps (%d)", dc.Name, node.Name, configuration.Node.MaxCmdOps)
		return nil, errors.NewCommonEdgeX(errors.KindServerError, errMsg, nil)
	}

	// create CommandValues
	cvs := make([]*sdkModels.CommandValue, 0, len(requests))
	for _, ro := range dc.ResourceOperations {
		drName := ro.Resource
		// check the resource in ResourceOperation actually exist
		dr, ok := cache.Profiles().Resource(node.ProfileName, drName)
		if !ok {
			errMsg := fmt.Sprintf("Resource %s in SET commnd %s for %s not defined", drName, dc.Name, node.Name)
			return nil, errors.NewCommonEdgeX(errors.KindServerError, errMsg, nil)
		}

		// check request body contains the resource
		value, ok := requests[ro.Resource]
		if !ok {
			if ro.DefaultValue != "" {
				value = ro.DefaultValue
			} else if dr.Properties.DefaultValue != "" {
				value = dr.Properties.DefaultValue
			} else {
				errMsg := fmt.Sprintf("Resource %s not found in request body and no default value defined", dr.Name)
				return nil, errors.NewCommonEdgeX(errors.KindServerError, errMsg, nil)
			}
		}

		// ResourceOperation mapping, notice that the order is opposite to get command mapping
		// i.e. the mapping value is actually the key for set command.
		if len(ro.Mappings) > 0 {
			for k, v := range ro.Mappings {
				if v == value {
					value = k
					break
				}
			}
		}

		// create CommandValue
		cv, err := createCommandValueFromResource(dr, value)
		if err == nil {
			cvs = append(cvs, cv)
		} else {
			return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, "failed to create CommandValue", err)
		}
	}

	// prepare CommandRequests
	reqs := make([]sdkModels.CommandRequest, len(cvs))
	for i, cv := range cvs {
		dr, _ := cache.Profiles().Resource(node.ProfileName, cv.ResourceName)

		reqs[i].ResourceName = cv.ResourceName
		reqs[i].Attributes = dr.Attributes
		if attributes != "" {
			if len(reqs[i].Attributes) <= 0 {
				reqs[i].Attributes = make(map[string]interface{})
			}
			reqs[i].Attributes[sdkCommon.URLRawQuery] = attributes
		}
		reqs[i].Type = cv.Type

		// transform write value
		if configuration.Node.DataTransform {
			err := transformer.TransformWriteParameter(cv, dr.Properties)
			if err != nil {
				return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, "failed to transform set parameter", err)
			}
		}
	}

	// execute protocol-specific write operation
	driver := container.ProtocolDriverFrom(dic.Get)
	err := driver.HandleWriteCommands(node.Name, node.Protocols, reqs, cvs)
	if err != nil {
		errMsg := fmt.Sprintf("error writing Command %s for %s", dc.Name, node.Name)
		return nil, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
	}

	// Updated resource(s) value will be published to MessageBus as long as they're not write-only
	if dc.ReadWrite != common.ReadWrite_W {
		return transformer.CommandValuesToEventDTO(cvs, node.Name, commandName, configuration.Node.DataTransform, dic)
	}

	return nil, nil
}

func validateServiceAndNodeState(nodeName string, dic *di.Container) (models.Node, errors.EdgeX) {
	// check node service AdminState
	ds := container.NodeServiceFrom(dic.Get)
	if ds.AdminState == models.Locked {
		return models.Node{}, errors.NewCommonEdgeX(errors.KindServiceLocked, "service locked", nil)
	}

	// check requested node exists
	node, ok := cache.Nodes().ForName(nodeName)
	if !ok {
		return models.Node{}, errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, fmt.Sprintf("node %s not found", nodeName), nil)
	}

	// check node's AdminState
	if node.AdminState == models.Locked {
		return models.Node{}, errors.NewCommonEdgeX(errors.KindServiceLocked, fmt.Sprintf("node %s locked", node.Name), nil)
	}
	// check node's OperatingState
	// if it's a node return attempt, operating state is allowed to be DOWN
	if node.OperatingState == models.Down {
		err := errors.NewCommonEdgeX(errors.KindServiceLocked, fmt.Sprintf("node %s OperatingState is DOWN", node.Name), nil)
		config := container.ConfigurationFrom(dic.Get)
		if config.Node.AllowedFails == 0 || config.Node.NodeDownTimeout == 0 {
			return models.Node{}, err
		}
		reqFailsTracker := container.AllowedRequestFailuresTrackerFrom(dic.Get)
		if reqFailsTracker.Value(nodeName) > 0 {
			return models.Node{}, err
		}
	}

	// check node's ProfileName
	if node.ProfileName == "" {
		return models.Node{}, errors.NewCommonEdgeX(errors.KindServiceLocked, "no associated node profile", nil)
	}

	return node, nil
}

func createCommandValueFromResource(dr models.Resource, value interface{}) (*sdkModels.CommandValue, errors.EdgeX) {
	if value == nil {
		return &sdkModels.CommandValue{
			ResourceName: dr.Name,
			Type:         dr.Properties.ValueType,
			Value:        value,
			Tags:         make(map[string]string)}, nil
	}

	var err error
	var result *sdkModels.CommandValue

	v := fmt.Sprint(value)

	if dr.Properties.ValueType != common.ValueTypeString && strings.TrimSpace(v) == "" {
		return nil, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("empty string is invalid for %v value type", dr.Properties.ValueType), nil)
	}

	switch dr.Properties.ValueType {
	case common.ValueTypeString:
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeString, v)
	case common.ValueTypeStringArray:
		var arr []string
		err = json.Unmarshal([]byte(v), &arr)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeStringArray, arr)
	case common.ValueTypeBool:
		var value bool
		value, err = strconv.ParseBool(v)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeBool, value)
	case common.ValueTypeBoolArray:
		var arr []bool
		err = json.Unmarshal([]byte(v), &arr)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeBoolArray, arr)
	case common.ValueTypeUint8:
		var n uint64
		n, err = strconv.ParseUint(v, 10, 8)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeUint8, uint8(n))
	case common.ValueTypeUint8Array:
		var arr []uint8
		strArr := strings.Split(strings.Trim(v, "[]"), ",")
		for _, u := range strArr {
			n, err := strconv.ParseUint(strings.Trim(u, " "), 10, 8)
			if err != nil {
				errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
				return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
			}
			arr = append(arr, uint8(n))
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeUint8Array, arr)
	case common.ValueTypeUint16:
		var n uint64
		n, err = strconv.ParseUint(v, 10, 16)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeUint16, uint16(n))
	case common.ValueTypeUint16Array:
		var arr []uint16
		strArr := strings.Split(strings.Trim(v, "[]"), ",")
		for _, u := range strArr {
			n, err := strconv.ParseUint(strings.Trim(u, " "), 10, 16)
			if err != nil {
				errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
				return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
			}
			arr = append(arr, uint16(n))
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeUint16Array, arr)
	case common.ValueTypeUint32:
		var n uint64
		n, err = strconv.ParseUint(v, 10, 32)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeUint32, uint32(n))
	case common.ValueTypeUint32Array:
		var arr []uint32
		strArr := strings.Split(strings.Trim(v, "[]"), ",")
		for _, u := range strArr {
			n, err := strconv.ParseUint(strings.Trim(u, " "), 10, 32)
			if err != nil {
				errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
				return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
			}
			arr = append(arr, uint32(n))
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeUint32Array, arr)
	case common.ValueTypeUint64:
		var n uint64
		n, err = strconv.ParseUint(v, 10, 64)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeUint64, n)
	case common.ValueTypeUint64Array:
		var arr []uint64
		strArr := strings.Split(strings.Trim(v, "[]"), ",")
		for _, u := range strArr {
			n, err := strconv.ParseUint(strings.Trim(u, " "), 10, 64)
			if err != nil {
				errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
				return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
			}
			arr = append(arr, n)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeUint64Array, arr)
	case common.ValueTypeInt8:
		var n int64
		n, err = strconv.ParseInt(v, 10, 8)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeInt8, int8(n))
	case common.ValueTypeInt8Array:
		var arr []int8
		err = json.Unmarshal([]byte(v), &arr)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeInt8Array, arr)
	case common.ValueTypeInt16:
		var n int64
		n, err = strconv.ParseInt(v, 10, 16)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeInt16, int16(n))
	case common.ValueTypeInt16Array:
		var arr []int16
		err = json.Unmarshal([]byte(v), &arr)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeInt16Array, arr)
	case common.ValueTypeInt32:
		var n int64
		n, err = strconv.ParseInt(v, 10, 32)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeInt32, int32(n))
	case common.ValueTypeInt32Array:
		var arr []int32
		err = json.Unmarshal([]byte(v), &arr)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeInt32Array, arr)
	case common.ValueTypeInt64:
		var n int64
		n, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeInt64, n)
	case common.ValueTypeInt64Array:
		var arr []int64
		err = json.Unmarshal([]byte(v), &arr)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeInt64Array, arr)
	case common.ValueTypeFloat32:
		var val float64
		val, err = strconv.ParseFloat(v, 32)
		if err == nil {
			result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeFloat32, float32(val))
			break
		}
		if numError, ok := err.(*strconv.NumError); ok {
			if numError.Err == strconv.ErrRange {
				err = errors.NewCommonEdgeX(errors.KindServerError, "NumError", err)
				break
			}
		}
		var decodedToBytes []byte
		decodedToBytes, err = base64.StdEncoding.DecodeString(v)
		if err == nil {
			var val float32
			val, err = float32FromBytes(decodedToBytes)
			if err != nil {
				break
			} else if math.IsNaN(float64(val)) {
				err = fmt.Errorf("fail to parse %v to float32, unexpected result %v", v, val)
			} else {
				result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeFloat32, val)
			}
		}
	case common.ValueTypeFloat32Array:
		var arr []float32
		err = json.Unmarshal([]byte(v), &arr)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeFloat32Array, arr)
	case common.ValueTypeFloat64:
		var val float64
		val, err = strconv.ParseFloat(v, 64)
		if err == nil {
			result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeFloat64, val)
			break
		}
		if numError, ok := err.(*strconv.NumError); ok {
			if numError.Err == strconv.ErrRange {
				err = errors.NewCommonEdgeX(errors.KindServerError, "NumError", err)
				break
			}
		}
		var decodedToBytes []byte
		decodedToBytes, err = base64.StdEncoding.DecodeString(v)
		if err == nil {
			val, err = float64FromBytes(decodedToBytes)
			if err != nil {
				break
			} else if math.IsNaN(val) {
				err = fmt.Errorf("fail to parse %v to float64, unexpected result %v", v, val)
			} else {
				result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeFloat64, val)
			}
		}
	case common.ValueTypeFloat64Array:
		var arr []float64
		err = json.Unmarshal([]byte(v), &arr)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, dr.Properties.ValueType)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeFloat64Array, arr)
	case common.ValueTypeObject:
		var obj map[string]any
		obj, err = normalizeToObject(value)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, common.ValueTypeObject)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeObject, obj)
	case common.ValueTypeObjectArray:
		var objArr []map[string]any
		objArr, err = normalizeToObjectArray(value)
		if err != nil {
			errMsg := fmt.Sprintf("failed to convert set parameter %s to ValueType %s", v, common.ValueTypeObjectArray)
			return result, errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		result, err = sdkModels.NewCommandValue(dr.Name, common.ValueTypeObjectArray, objArr)
	default:
		err = errors.NewCommonEdgeX(errors.KindServerError, "unrecognized value type", nil)
	}

	if err != nil {
		return nil, errors.NewCommonEdgeXWrapper(err)
	}
	return result, nil
}

func float32FromBytes(numericValue []byte) (res float32, err error) {
	reader := bytes.NewReader(numericValue)
	err = binary.Read(reader, binary.BigEndian, &res)
	return
}

func float64FromBytes(numericValue []byte) (res float64, err error) {
	reader := bytes.NewReader(numericValue)
	err = binary.Read(reader, binary.BigEndian, &res)
	return
}

func normalizeToObject(value any) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}

	var m map[string]any
	switch v := value.(type) {
	case string: // Support string defaultValue in node profiles
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return nil, fmt.Errorf("failed to convert set parameter %s to ValueType %s", v, common.ValueTypeObject)
		}
	case map[string]any:
		m = v
	default:
		return nil, fmt.Errorf("unsupported type for Object: %T", value)
	}

	return m, nil
}

func normalizeToObjectArray(value any) ([]map[string]any, error) {
	if value == nil {
		return nil, nil
	}

	var arr []map[string]any
	switch v := value.(type) {
	case string: // Support string defaultValue in node profiles
		if err := json.Unmarshal([]byte(v), &arr); err != nil {
			return nil, fmt.Errorf("failed to convert set parameter %s to ValueType %s", v, common.ValueTypeObjectArray)
		}
	case []any:
		for _, elem := range v {
			if m, ok := elem.(map[string]any); ok {
				arr = append(arr, m)
			} else {
				return nil, fmt.Errorf("element is not map[string]any: %v", elem)
			}
		}
	case []map[string]any:
		arr = v
	default:
		return nil, fmt.Errorf("unsupported type for ObjectArray: %T", value)
	}

	return arr, nil
}
