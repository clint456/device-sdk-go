// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2020-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package container

import (
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"

	"github.com/edgexfoundry/south-sdk-go/v4/pkg/interfaces"
)

// NodeServiceName contains the name of node service struct in the DIC.
var NodeServiceName = di.TypeInstanceToName(models.NodeService{})

// ProtocolDriverName contains the name of protocol driver implementation in the DIC.
var ProtocolDriverName = di.TypeInstanceToName((*interfaces.ProtocolDriver)(nil))

// AutoEventManagerName contains the name of autoevent manager implementation in the DIC
var AutoEventManagerName = di.TypeInstanceToName((*interfaces.AutoEventManager)(nil))

// ExtendedProtocolDriverName contains the name of extended protocol driver implementation in the DIC.
var ExtendedProtocolDriverName = di.TypeInstanceToName((*interfaces.ExtendedProtocolDriver)(nil))

// NodeServiceFrom helper function queries the DIC and returns node service struct.
func NodeServiceFrom(get di.Get) *models.NodeService {
	return get(NodeServiceName).(*models.NodeService)
}

// ProtocolDriverFrom helper function queries the DIC and returns protocol driver implementation.
func ProtocolDriverFrom(get di.Get) interfaces.ProtocolDriver {
	return get(ProtocolDriverName).(interfaces.ProtocolDriver)
}

// AutoEventManagerFrom helper function queries the DIC and returns autoevent manager implementation
func AutoEventManagerFrom(get di.Get) interfaces.AutoEventManager {
	return get(AutoEventManagerName).(interfaces.AutoEventManager)
}

// ExtendedProtocolDriverFrom helper function queries the DIC and returns extended protocol driver implementation.
func ExtendedProtocolDriverFrom(get di.Get) interfaces.ExtendedProtocolDriver {
	casted, ok := get(ExtendedProtocolDriverName).(interfaces.ExtendedProtocolDriver)
	if ok {
		return casted
	}
	return nil
}

// DiscoveryRequestIdName contains the name of discovery request id implementation in the DIC.
var DiscoveryRequestIdName = di.TypeInstanceToName(new(string))

// DiscoveryRequestIdFrom helper function queries the DIC and returns discovery request id.
func DiscoveryRequestIdFrom(get di.Get) string {
	id, ok := get(DiscoveryRequestIdName).(string)
	if !ok {
		return ""
	}
	return id
}

// AllowedRequestFailuresTrackerName contains the name of allowed request failures tracker in the DIC.
var AllowedRequestFailuresTrackerName = di.TypeInstanceToName(AllowedFailuresTracker{})

// AllowedRequestFailuresTrackerFrom helper function queries the DIC and returns a node request failures tracker.
func AllowedRequestFailuresTrackerFrom(get di.Get) AllowedFailuresTracker {
	return get(AllowedRequestFailuresTrackerName).(AllowedFailuresTracker)
}
