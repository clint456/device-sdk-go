// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018-2019 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// This package provides a simple example of a node service.
package main

import (
	"github.com/edgexfoundry/south-sdk-go/v4"
	"github.com/edgexfoundry/south-sdk-go/v4/example/driver"
	"github.com/edgexfoundry/south-sdk-go/v4/pkg/startup"
)

const (
	serviceName string = "node-simple"
)

func main() {
	sd := driver.SimpleDriver{}
	startup.Bootstrap(serviceName, node.Version, &sd)
}
