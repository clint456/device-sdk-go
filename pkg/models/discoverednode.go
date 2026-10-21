//
// Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package models

import "github.com/edgexfoundry/go-mod-core-contracts/v4/models"

// DiscoveredNode defines the required information for a found node.
type DiscoveredNode struct {
	Name        string
	Protocols   map[string]models.ProtocolProperties
	Description string
	Labels      []string
}
