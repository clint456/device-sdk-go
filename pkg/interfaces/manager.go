//
// Copyright (C) 2021 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package interfaces

type AutoEventManager interface {
	// StartAutoEvents starts all the AutoEvents of the node service
	StartAutoEvents()
	// RestartForNode restarts all the AutoEvents of the specific node
	RestartForNode(name string)
	// StopForNode stops all the AutoEvents of the specific node
	StopForNode(name string)
}
