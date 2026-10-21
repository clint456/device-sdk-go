//
// Copyright (C) 2024 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package models

type Progress struct {
	RequestId string `json:"requestId"`
	Progress  int    `json:"progress"`
	Message   string `json:"message,omitempty"`
}
type NodeDiscoveryProgress struct {
	Progress            `json:",inline"`
	DiscoveredNodeCount int `json:"discoveredNodeCount,omitempty"`
}
