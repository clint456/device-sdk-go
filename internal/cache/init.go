// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2020-2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"

	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
)

// InitCache Init basic state for cache
func InitCache(instanceName string, baseServiceName string, dic *di.Container) errors.EdgeX {
	dc := bootstrapContainer.NodeClientFrom(dic.Get)
	dpc := bootstrapContainer.ProfileClientFrom(dic.Get)
	pwc := bootstrapContainer.ProvisionWatcherClientFrom(dic.Get)

	// init node cache
	nodeRes, err := dc.NodesByServiceName(context.Background(), instanceName, 0, -1)
	if err != nil {
		return err
	}
	nodes := make([]models.Node, len(nodeRes.Nodes))
	for i := range nodeRes.Nodes {
		nodes[i] = dtos.ToNodeModel(nodeRes.Nodes[i])
	}
	newNodeCache(nodes, dic)

	// init profile cache
	profiles := make([]models.Profile, 0, len(nodes))
	for _, d := range nodes {
		if len(d.ProfileName) == 0 {
			continue
		}
		res, err := dpc.ProfileByName(context.Background(), d.ProfileName)
		if err != nil {
			return err
		}
		profiles = append(profiles, dtos.ToProfileModel(res.Profile))
	}
	newProfileCache(profiles)

	// init provision watcher cache
	// baseServiceName is the service name w/o the instance portion added when -i/--instance flag is used.
	// Using baseServiceName here since ProvisionWatchers are used by all instances of the node service and thus have the ServiceName set to the baseServiceName.
	pwRes, err := pwc.ProvisionWatchersByServiceName(context.Background(), baseServiceName, 0, -1)
	if err != nil {
		return err
	}
	pws := make([]models.ProvisionWatcher, len(pwRes.ProvisionWatchers))
	for i := range pwRes.ProvisionWatchers {
		pws[i] = dtos.ToProvisionWatcherModel(pwRes.ProvisionWatchers[i])
	}
	newProvisionWatcherCache(pws)

	return nil
}
