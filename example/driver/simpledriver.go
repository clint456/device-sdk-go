// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018-2024 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// Package driver provides a simple example implementation of
// ProtocolDriver interface.
package driver

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	gometrics "github.com/rcrowley/go-metrics"

	"github.com/edgexfoundry/south-sdk-go/v4/example/config"
	"github.com/edgexfoundry/south-sdk-go/v4/pkg/interfaces"
	sdkModels "github.com/edgexfoundry/south-sdk-go/v4/pkg/models"
)

const readCommandsExecutedName = "ReadCommandsExecuted"

type SimpleDriver struct {
	sdk                  interfaces.NodeServiceSDK
	lc                   logger.LoggingClient
	stopDiscovery        stopDiscoveryState
	stopProfileScan      stopProfileScanState
	asyncCh              chan<- *sdkModels.AsyncValues
	nodeCh               chan<- []sdkModels.DiscoveredNode
	switchButton         bool
	xRotation            int32
	yRotation            int32
	zRotation            int32
	counter              interface{}
	stringArray          []string
	readCommandsExecuted gometrics.Counter
	serviceConfig        *config.ServiceConfig
}

type stopDiscoveryState struct {
	stop   bool
	locker sync.RWMutex
}

type stopProfileScanState struct {
	stop   map[string]bool
	locker sync.RWMutex
}

func getImageBytes(imgFile string, buf *bytes.Buffer) error {
	// Read existing image from file
	img, err := os.Open(imgFile)
	if err != nil {
		return err
	}
	defer img.Close()

	// TODO: Attach MediaType property, determine if decoding
	//  early is required (to optimize edge processing)

	// Expect "png" or "jpeg" image type
	imageData, imageType, err := image.Decode(img)
	if err != nil {
		return err
	}
	// Finished with file. Reset file pointer
	_, err = img.Seek(0, 0)
	if err != nil {
		return err
	}
	switch imageType {
	case "jpeg":
		err = jpeg.Encode(buf, imageData, nil)
		if err != nil {
			return err
		}
	case "png":
		err = png.Encode(buf, imageData)
		if err != nil {
			return err
		}
	}
	return nil
}

// Initialize performs protocol-specific initialization for the node
// service.
func (s *SimpleDriver) Initialize(sdk interfaces.NodeServiceSDK) error {
	s.sdk = sdk
	s.lc = sdk.LoggingClient()
	s.asyncCh = sdk.AsyncValuesChannel()
	s.nodeCh = sdk.DiscoveredNodeChannel()
	s.serviceConfig = &config.ServiceConfig{}
	s.counter = map[string]interface{}{
		"f1": "ABC",
		"f2": 123,
	}
	s.stringArray = []string{"foo", "bar"}
	s.stopProfileScan = stopProfileScanState{stop: make(map[string]bool)}

	if err := sdk.LoadCustomConfig(s.serviceConfig, "SimpleCustom"); err != nil {
		return fmt.Errorf("unable to load 'SimpleCustom' custom configuration: %s", err.Error())
	}

	s.lc.Infof("Custom config is: %v", s.serviceConfig.SimpleCustom)

	if err := s.serviceConfig.SimpleCustom.Validate(); err != nil {
		return fmt.Errorf("'SimpleCustom' custom configuration validation failed: %s", err.Error())
	}

	if err := sdk.ListenForCustomConfigChanges(
		&s.serviceConfig.SimpleCustom.Writable,
		"SimpleCustom/Writable", s.ProcessCustomConfigChanges); err != nil {
		return fmt.Errorf("unable to listen for changes for 'SimpleCustom.Writable' custom configuration: %s", err.Error())
	}

	s.readCommandsExecuted = gometrics.NewCounter()

	var err error
	metricsManger := sdk.MetricsManager()
	if metricsManger != nil {
		err = metricsManger.Register(readCommandsExecutedName, s.readCommandsExecuted, nil)
	} else {
		err = errors.New("metrics manager not available")
	}

	if err != nil {
		return fmt.Errorf("unable to register metric %s: %s", readCommandsExecutedName, err.Error())
	}

	s.lc.Infof("Registered %s metric for collection when enabled", readCommandsExecutedName)

	return nil
}

// ProcessCustomConfigChanges ...
func (s *SimpleDriver) ProcessCustomConfigChanges(rawWritableConfig interface{}) {
	updated, ok := rawWritableConfig.(*config.SimpleWritable)
	if !ok {
		s.lc.Error("unable to process custom config updates: Can not cast raw config to type 'SimpleWritable'")
		return
	}

	s.lc.Info("Received configuration updates for 'SimpleCustom.Writable' section")

	previous := s.serviceConfig.SimpleCustom.Writable
	s.serviceConfig.SimpleCustom.Writable = *updated

	if reflect.DeepEqual(previous, *updated) {
		s.lc.Info("No changes detected")
		return
	}

	// Now check to determine what changed.
	// In this example we only have the one writable setting,
	// so the check is not really need but left here as an example.
	// Since this setting is pulled from configuration each time it is need, no extra processing is required.
	// This may not be true for all settings, such as external host connection info, which
	// may require re-establishing the connection to the external host for example.
	if previous.DiscoverSleepDurationSecs != updated.DiscoverSleepDurationSecs {
		s.lc.Infof("DiscoverSleepDurationSecs changed to: %d", updated.DiscoverSleepDurationSecs)
	}
}

// HandleReadCommands triggers a protocol Read operation for the specified node.
func (s *SimpleDriver) HandleReadCommands(nodeName string, protocols map[string]models.ProtocolProperties, reqs []sdkModels.CommandRequest) (res []*sdkModels.CommandValue, err error) {
	s.lc.Debugf("SimpleDriver.HandleReadCommands: protocols: %v resource: %v attributes: %v", protocols, reqs[0].ResourceName, reqs[0].Attributes)

	res = make([]*sdkModels.CommandValue, 0)
	for _, req := range reqs {
		var cv *sdkModels.CommandValue
		switch req.ResourceName {
		case "SwitchButton":
			cv, _ = sdkModels.NewCommandValue(req.ResourceName, common.ValueTypeBool, s.switchButton)
		case "Xrotation":
			cv, _ = sdkModels.NewCommandValue(req.ResourceName, common.ValueTypeInt32, s.xRotation)
		case "Yrotation":
			cv, _ = sdkModels.NewCommandValue(req.ResourceName, common.ValueTypeInt32, s.yRotation)
		case "Zrotation":
			cv, _ = sdkModels.NewCommandValue(req.ResourceName, common.ValueTypeInt32, s.zRotation)
		case "Image":
			// Show a binary/image representation of the switch's on/off value
			buf := new(bytes.Buffer)
			if s.switchButton {
				err = getImageBytes(s.serviceConfig.SimpleCustom.OnImageLocation, buf)
			} else {
				err = getImageBytes(s.serviceConfig.SimpleCustom.OffImageLocation, buf)
			}
			cv, _ = sdkModels.NewCommandValue(req.ResourceName, common.ValueTypeBinary, buf.Bytes())
		case "StringArray":
			cv, _ = sdkModels.NewCommandValue(req.ResourceName, common.ValueTypeStringArray, s.stringArray)
		case "Uint8Array":
			cv, _ = sdkModels.NewCommandValue(req.ResourceName, common.ValueTypeUint8Array, []uint8{0, 1, 2})
		case "Counter":
			cv, _ = sdkModels.NewCommandValue(req.ResourceName, common.ValueTypeObject, s.counter)
		}

		res = append(res, cv)
	}

	s.readCommandsExecuted.Inc(1)

	return
}

// HandleWriteCommands passes a slice of CommandRequest struct each representing
// a ResourceOperation for a specific node resource.
// Since the commands are actuation commands, params provide parameters for the individual
// command.
func (s *SimpleDriver) HandleWriteCommands(nodeName string, protocols map[string]models.ProtocolProperties, reqs []sdkModels.CommandRequest,
	params []*sdkModels.CommandValue) error {
	var err error

	for i, r := range reqs {
		s.lc.Debugf("SimpleDriver.HandleWriteCommands: protocols: %v, resource: %v, parameters: %v, attributes: %v", protocols, reqs[i].ResourceName, params[i], reqs[i].Attributes)
		switch r.ResourceName {
		case "SwitchButton":
			if s.switchButton, err = params[i].BoolValue(); err != nil {
				err := fmt.Errorf("SimpleDriver.HandleWriteCommands; the data type of parameter should be Boolean, parameter: %s", params[0].String())
				return err
			}
		case "Xrotation":
			if s.xRotation, err = params[i].Int32Value(); err != nil {
				err := fmt.Errorf("SimpleDriver.HandleWriteCommands; the data type of parameter should be Int32, parameter: %s", params[i].String())
				return err
			}
		case "Yrotation":
			if s.yRotation, err = params[i].Int32Value(); err != nil {
				err := fmt.Errorf("SimpleDriver.HandleWriteCommands; the data type of parameter should be Int32, parameter: %s", params[i].String())
				return err
			}
		case "Zrotation":
			if s.zRotation, err = params[i].Int32Value(); err != nil {
				err := fmt.Errorf("SimpleDriver.HandleWriteCommands; the data type of parameter should be Int32, parameter: %s", params[i].String())
				return err
			}
		case "StringArray":
			if s.stringArray, err = params[i].StringArrayValue(); err != nil {
				err := fmt.Errorf("SimpleDriver.HandleWriteCommands; the data type of parameter should be string array, parameter: %s", params[i].String())
				return err
			}
		case "Uint8Array":
			v, err := params[i].Uint8ArrayValue()
			if err == nil {
				s.lc.Debugf("Uint8 array value from write command: ", v)
			} else {
				return err
			}
		case "Counter":
			if s.counter, err = params[i].ObjectValue(); err != nil {
				err := fmt.Errorf("SimpleDriver.HandleWriteCommands; the data type of parameter should be Object, parameter: %s", params[i].String())
				return err
			}
		}
	}

	return nil
}

// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (s *SimpleDriver) Stop(force bool) error {
	// Then Logging Client might not be initialized
	if s.lc != nil {
		s.lc.Debugf("SimpleDriver.Stop called: force=%v", force)
	}
	return nil
}

func (s *SimpleDriver) Start() error {
	return nil
}

// AddNode is a callback function that is invoked
// when a new Node associated with this Node Service is added
func (s *SimpleDriver) AddNode(nodeName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	s.lc.Debugf("a new Node is added: %s", nodeName)
	return nil
}

// UpdateNode is a callback function that is invoked
// when a Node associated with this Node Service is updated
func (s *SimpleDriver) UpdateNode(nodeName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	s.lc.Debugf("Node %s is updated", nodeName)
	return nil
}

// RemoveNode is a callback function that is invoked
// when a Node associated with this Node Service is removed
func (s *SimpleDriver) RemoveNode(nodeName string, protocols map[string]models.ProtocolProperties) error {
	s.lc.Debugf("Node %s is removed", nodeName)
	return nil
}

// Discover triggers protocol specific node discovery, which is an asynchronous operation.
// Nodes found as part of this discovery operation are written to the channel nodes.
func (s *SimpleDriver) Discover() error {
	proto := make(map[string]models.ProtocolProperties)
	proto["other"] = map[string]any{"Address": "simple02", "Port": 301}

	node2 := sdkModels.DiscoveredNode{
		Name:        "Simple-Node02",
		Protocols:   proto,
		Description: "found by discovery",
		Labels:      []string{"auto-discovery"},
	}

	proto = make(map[string]models.ProtocolProperties)
	proto["other"] = map[string]any{"Address": "simple03", "Port": 399}

	node3 := sdkModels.DiscoveredNode{
		Name:        "Simple-Node03",
		Protocols:   proto,
		Description: "found by discovery",
		Labels:      []string{"auto-discovery"},
	}

	res := []sdkModels.DiscoveredNode{node2}
	time.Sleep(time.Duration(s.serviceConfig.SimpleCustom.Writable.DiscoverSleepDurationSecs) * time.Second)
	s.sdk.PublishNodeDiscoveryProgressSystemEvent(50, len(res), "")
	defer s.setStopNodeDiscovery(false)
	if !s.getStopNodeDiscovery() {
		time.Sleep(time.Duration(s.serviceConfig.SimpleCustom.Writable.DiscoverSleepDurationSecs) * time.Second)
		res = append(res, node3)
	}
	s.nodeCh <- res
	return nil
}

func (s *SimpleDriver) ValidateNode(node models.Node) error {
	protocol, ok := node.Protocols["other"]
	if !ok {
		return errors.New("missing 'other' protocols")
	}

	addr, ok := protocol["Address"]
	if !ok {
		return errors.New("missing 'Address' information")
	} else if addr == "" {
		return errors.New("address must not empty")
	}

	port, ok := protocol["Port"]
	if !ok {
		return errors.New("missing 'Port' information")
	} else {
		portString := fmt.Sprintf("%v", port)
		_, err := strconv.ParseUint(portString, 10, 64)
		if err != nil {
			return errors.New("port must be a number")
		}
	}

	return nil
}

func (s *SimpleDriver) ProfileScan(payload requests.ProfileScanRequest) (models.Profile, error) {
	time.Sleep(time.Duration(s.serviceConfig.SimpleCustom.Writable.DiscoverSleepDurationSecs) * time.Second)
	s.sdk.PublishProfileScanProgressSystemEvent(payload.RequestId, 50, "")
	if s.getStopProfileScan(payload.NodeName) {
		return models.Profile{}, fmt.Errorf("profile scanning is stopped")
	}
	time.Sleep(time.Duration(s.serviceConfig.SimpleCustom.Writable.DiscoverSleepDurationSecs) * time.Second)
	return models.Profile{ProfileBasicInfo: models.ProfileBasicInfo{Name: payload.ProfileName}}, nil
}

func (s *SimpleDriver) StopNodeDiscovery(options map[string]any) {
	s.lc.Debugf("StopNodeDiscovery called: options=%v", options)
	s.setStopNodeDiscovery(true)
}

func (s *SimpleDriver) StopProfileScan(node string, options map[string]any) {
	s.lc.Debugf("StopProfileScan called: options=%v", options)
	s.setStopProfileScan(node, true)
}

func (s *SimpleDriver) getStopNodeDiscovery() bool {
	s.stopDiscovery.locker.RLock()
	defer s.stopDiscovery.locker.RUnlock()
	return s.stopDiscovery.stop
}

func (s *SimpleDriver) setStopNodeDiscovery(stop bool) {
	s.stopDiscovery.locker.Lock()
	defer s.stopDiscovery.locker.Unlock()
	s.stopDiscovery.stop = stop
	s.lc.Debugf("set stopNodeDiscovery to %v", stop)
}

func (s *SimpleDriver) getStopProfileScan(node string) bool {
	s.stopProfileScan.locker.RLock()
	defer s.stopProfileScan.locker.RUnlock()
	return s.stopProfileScan.stop[node]
}

func (s *SimpleDriver) setStopProfileScan(node string, stop bool) {
	s.stopProfileScan.locker.Lock()
	defer s.stopProfileScan.locker.Unlock()
	s.stopProfileScan.stop[node] = stop
	s.lc.Debugf("set stopProfileScan to %v", stop)
}
