// -*- mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018-2026 IOTech Ltd
// Copyright (c) 2021 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/edgexfoundry/go-mod-bootstrap/v4/config"
)

// WritableInfo is a struct which contains configuration settings that can be changed in the Registry .
type WritableInfo struct {
	// Level is the logging level of writing log message
	LogLevel        string
	InsecureSecrets config.InsecureSecrets
	Reading         Reading
	Telemetry       config.TelemetryInfo
}

// Reading is a struct which contains reading configuration settings.
type Reading struct {
	// ReadingUnits specifies whether or not to indicate the units of measure for the value in the reading
	ReadingUnits bool
}

// NodeInfo is a struct which contains node specific configuration settings.
type NodeInfo struct {
	// DataTransform specifies whether or not the DS perform transformations
	// specified by value descriptor on a actuation or query command.
	DataTransform bool
	// MaxCmdOps defines the maximum number of resource operations that
	// can be sent to a Driver in a single command.
	MaxCmdOps int
	// ProfilesDir specifies a directory which contains node profiles
	// files which should be imported on startup.
	ProfilesDir string
	// NodesDir specifies a directory contains nodes files which should be imported on startup.
	NodesDir string
	// ProvisionWatchersDir specifies a directory contains provision watcher files which should be imported on startup.
	ProvisionWatchersDir string
	Discovery            DiscoveryInfo
	// AsyncBufferSize defines the size of asynchronous channel
	AsyncBufferSize int
	// EnableAsyncReadings to determine whether the Node Service would deal with the asynchronous readings
	EnableAsyncReadings bool
	// Labels are properties applied to the node service to help with searching
	Labels []string
	// AllowedFails specifies the number of failed requests allowed before a node is marked as down.
	AllowedFails uint
	// NodeDownTimeout specifies the duration in seconds that the Node Service will try to contact a node if it is marked as down.
	NodeDownTimeout uint
	// AutoEvents defines the configuration related to the generated auto events for the node service
	AutoEvents AutoEventInfo
}

// DiscoveryInfo is a struct which contains configuration of node auto discovery.
type DiscoveryInfo struct {
	// Enabled controls whether or not node discovery is enabled.
	Enabled bool
	// Interval indicates how often the discovery process will be triggered.
	// It represents as a duration string.
	Interval string
}

// Telemetry provides metrics (on a given node service) to system management.
type Telemetry struct {
	Alloc,
	TotalAlloc,
	Sys,
	Mallocs,
	Frees,
	LiveObjects uint64
}

type AutoEventInfo struct {
	// If true, only updated readings compared to the previous event are included in the generated auto event
	SendChangedReadingsOnly bool
}
