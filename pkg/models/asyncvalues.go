// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package models

// AsyncValues is the struct for sending Node readings asynchronously via ProtocolDrivers
type AsyncValues struct {
	NodeName      string
	SourceName    string
	CommandValues []*CommandValue
}
