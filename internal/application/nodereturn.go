// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"time"

	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"

	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
	sdkCommon "github.com/edgexfoundry/south-sdk-go/v4/internal/common"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/container"
)

func nodeReturn(nodeName string, dic *di.Container) {
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	dc := bootstrapContainer.NodeClientFrom(dic.Get)
	config := container.ConfigurationFrom(dic.Get)

	for {
	LOOP:
		time.Sleep(time.Duration(config.Node.NodeDownTimeout) * time.Second) // #nosec G115
		lc.Infof("Checking operational state for node: %s", nodeName)

		d, found := cache.Nodes().ForName(nodeName)
		if !found {
			lc.Warnf("Node %s not found. Exiting retry loop.", nodeName)
			return
		}

		if d.OperatingState == models.Up {
			lc.Infof("Node %s is already operational. Exiting retry loop.", nodeName)
			return
		}

		p, found := cache.Profiles().ForName(d.ProfileName)
		if !found {
			lc.Warnf("Node %s has no profile. Cannot set operational state automatically.", nodeName)
			return
		}

		for _, dr := range p.Resources {
			if dr.Properties.ReadWrite == common.ReadWrite_R ||
				dr.Properties.ReadWrite == common.ReadWrite_RW ||
				dr.Properties.ReadWrite == common.ReadWrite_WR {
				_, err := GetCommand(context.Background(), nodeName, dr.Name, "", true, dic)
				if err == nil {
					lc.Infof("Node %s responsive: setting operational state to up.", nodeName)
					sdkCommon.UpdateOperatingState(nodeName, models.Up, lc, dc)
					return
				} else {
					lc.Errorf("Node %s unresponsive: retrying in %v seconds.", nodeName, config.Node.NodeDownTimeout)
					goto LOOP
				}
			}
		}
		lc.Infof("Node %s has no readable resources. Setting operational state to up without checking.", nodeName)
		sdkCommon.UpdateOperatingState(nodeName, models.Up, lc, dc)
		return
	}
}

func NodeRequestFailed(nodeName string, dic *di.Container) {
	config := container.ConfigurationFrom(dic.Get)
	if config.Node.AllowedFails > 0 {
		lc := bootstrapContainer.LoggingClientFrom(dic.Get)
		dc := bootstrapContainer.NodeClientFrom(dic.Get)
		reqFailsTracker := container.AllowedRequestFailuresTrackerFrom(dic.Get)

		if reqFailsTracker.Decrease(nodeName) == 0 {
			d, ok := cache.Nodes().ForName(nodeName)
			if !ok {
				return
			}
			if d.OperatingState != models.Down {
				lc.Infof("Marking node %s non-operational", nodeName)
				sdkCommon.UpdateOperatingState(nodeName, models.Down, lc, dc)
			}
			if config.Node.NodeDownTimeout > 0 {
				lc.Warnf("Will retry node %s in %v seconds", nodeName, config.Node.NodeDownTimeout)
				go nodeReturn(nodeName, dic)
			}
			return
		}
	}
}

func NodeRequestSucceeded(d models.Node, dic *di.Container) {
	config := container.ConfigurationFrom(dic.Get)
	reqFailsTracker := container.AllowedRequestFailuresTrackerFrom(dic.Get)
	allowFailsNum := int(config.Node.AllowedFails) // #nosec G115
	if config.Node.AllowedFails > 0 && reqFailsTracker.Value(d.Name) < allowFailsNum {
		reqFailsTracker.Set(d.Name, allowFailsNum)
		if d.OperatingState == models.Down {
			lc := bootstrapContainer.LoggingClientFrom(dic.Get)
			dc := bootstrapContainer.NodeClientFrom(dic.Get)
			sdkCommon.UpdateOperatingState(d.Name, models.Up, lc, dc)
		}
	}
}
