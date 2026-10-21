// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2019-2021 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package models

// CommandRequest is the struct for requesting a command to ProtocolDrivers
type CommandRequest struct {
	// ResourceName is the name of Node Resource for this command
	ResourceName string
	// Attributes is a key/value map to represent the attributes of the Node Resource
	Attributes map[string]interface{}
	// Type is the data type of the Node Resource
	Type string
}
