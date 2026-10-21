// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018-2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// Package interfaces defines interfaces and structs used to build an EdgeX Foundry Node
// Service.  The interfaces provide an abstraction layer for the node
// or protocol specific logic of a Node Service, and the structs represents request
// and response data format used by the protocol driver.
package interfaces

import (
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"

	sdkModels "github.com/edgexfoundry/south-sdk-go/v4/pkg/models"
)

// ProtocolDriver is a low-level node-specific interface used by
// other components of an EdgeX Node Service to interact with
// a specific class of nodes.
type ProtocolDriver interface {
	// Initialize performs protocol-specific initialization for the node service.
	// The given *AsyncValues channel can be used to push asynchronous events and
	// readings to Core Data. The given []DiscoveredNode channel is used to send
	// discovered nodes that will be filtered and added to Core Metadata asynchronously.
	Initialize(sdk NodeServiceSDK) error

	// HandleReadCommands passes a slice of CommandRequest struct each representing
	// a ResourceOperation for a specific node resource.
	HandleReadCommands(nodeName string, protocols map[string]models.ProtocolProperties, reqs []sdkModels.CommandRequest) ([]*sdkModels.CommandValue, error)

	// HandleWriteCommands passes a slice of CommandRequest struct each representing
	// a ResourceOperation for a specific node resource.
	// Since the commands are actuation commands, params provide parameters for the individual
	// command.
	HandleWriteCommands(nodeName string, protocols map[string]models.ProtocolProperties, reqs []sdkModels.CommandRequest, params []*sdkModels.CommandValue) error

	// Stop instructs the protocol-specific DS code to shutdown gracefully, or
	// if the force parameter is 'true', immediately. The driver is responsible
	// for closing any in-use channels, including the channel used to send async
	// readings (if supported).
	Stop(force bool) error

	// Start runs Node Service startup tasks after the SDK has been completely initialized.
	// This allows Node Service to safely use NodeServiceSDK interface features in this function call.
	Start() error

	// AddNode is a callback function that is invoked
	// when a new Node associated with this Node Service is added
	AddNode(nodeName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error

	// UpdateNode is a callback function that is invoked
	// when a Node associated with this Node Service is updated
	UpdateNode(nodeName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error

	// RemoveNode is a callback function that is invoked
	// when a Node associated with this Node Service is removed
	RemoveNode(nodeName string, protocols map[string]models.ProtocolProperties) error

	// Discover triggers protocol specific node discovery, asynchronously
	// writes the results to the channel which is passed to the implementation
	// via ProtocolDriver.Initialize(). The results may be added to the node service
	// based on a set of acceptance criteria (i.e. Provision Watchers).
	Discover() error

	// ValidateNode triggers node's protocol properties validation, returns error
	// if validation failed and the incoming node will not be added into EdgeX.
	ValidateNode(node models.Node) error
}
