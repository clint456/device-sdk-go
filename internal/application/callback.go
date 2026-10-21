// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2020-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"

	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/container"

	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
)

func UpdateProfile(profileRequest requests.ProfileRequest, dic *di.Container) errors.EdgeX {
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)

	_, ok := cache.Profiles().ForName(profileRequest.Profile.Name)
	if !ok {
		errMsg := fmt.Sprintf("failed to find profile %s", profileRequest.Profile.Name)
		return errors.NewCommonEdgeX(errors.KindInvalidId, errMsg, nil)
	}

	err := cache.Profiles().Update(dtos.ToProfileModel(profileRequest.Profile))
	if err != nil {
		errMsg := fmt.Sprintf("failed to update profile %s", profileRequest.Profile.Name)
		return errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
	}

	lc.Debugf("profile %s updated", profileRequest.Profile.Name)

	driver := container.ProtocolDriverFrom(dic.Get)
	nodes := cache.Nodes().All()
	for _, d := range nodes {
		if d.ProfileName == profileRequest.Profile.Name {
			if err := driver.UpdateNode(d.Name, d.Protocols, d.AdminState); err != nil {
				errMsg := fmt.Sprintf("driver.UpdateNode callback failed for %s", d.Name)
				return errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
			}
			lc.Debugf("Invoked driver.UpdateNode callback for %s", d.Name)
		}
	}

	return nil
}

func DeleteProfile(profileName string, dic *di.Container) errors.EdgeX {
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	if cache.CheckProfileNotUsed(profileName) {
		err := cache.Profiles().RemoveByName(profileName)
		if err != nil {
			errMsg := fmt.Sprintf("failed to remove node profile %s", profileName)
			return errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		lc.Debugf("profile %s is removed from cache", profileName)
	} else {
		lc.Warnf("received Profile Deletion System Event for %s, but the profile is still used by some nodes", profileName)
	}

	return nil
}

func AddNode(addNodeRequest requests.AddNodeRequest, dic *di.Container) errors.EdgeX {
	node := dtos.ToNodeModel(addNodeRequest.Node)
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	var edgexErr errors.EdgeX
	if addNodeRequest.Node.ProfileName != "" {
		edgexErr = updateAssociatedProfile(node.ProfileName, dic)
		if edgexErr != nil {
			return errors.NewCommonEdgeXWrapper(edgexErr)
		}
	}

	edgexErr = cache.Nodes().Add(node)
	if edgexErr != nil {
		errMsg := fmt.Sprintf("failed to add node %s", node.Name)
		return errors.NewCommonEdgeX(errors.KindServerError, errMsg, edgexErr)
	}
	lc.Debugf("node %s added", node.Name)

	driver := container.ProtocolDriverFrom(dic.Get)
	err := driver.AddNode(node.Name, node.Protocols, node.AdminState)
	if err == nil {
		lc.Debugf("Invoked driver.AddNode callback for %s", node.Name)
	} else {
		errMsg := fmt.Sprintf("driver.AddNode callback failed for %s", node.Name)
		return errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
	}

	config := container.ConfigurationFrom(dic.Get)
	reqFailsTracker := container.AllowedRequestFailuresTrackerFrom(dic.Get)
	reqFailsTracker.Set(node.Name, int(config.Node.AllowedFails)) // #nosec G115

	lc.Debugf("starting AutoEvents for node %s", node.Name)
	container.AutoEventManagerFrom(dic.Get).RestartForNode(node.Name)
	return nil
}

func UpdateNode(updateNodeRequest requests.UpdateNodeRequest, dic *di.Container) errors.EdgeX {
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	ds := container.NodeServiceFrom(dic.Get)

	node, exist := cache.Nodes().ForName(*updateNodeRequest.Node.Name)
	if !exist {
		// scenario that node migrates from another node service to here
		if ds.Name == *updateNodeRequest.Node.ServiceName {
			var newNode models.Node
			requests.ReplaceNodeModelFieldsWithDTO(&newNode, updateNodeRequest.Node)
			newNode.Name = *updateNodeRequest.Node.Name
			newNode.Id = *updateNodeRequest.Node.Id
			req := requests.NewAddNodeRequest(dtos.FromNodeModelToDTO(newNode))
			return AddNode(req, dic)
		} else {
			errMsg := fmt.Sprintf("failed to find node %s", *updateNodeRequest.Node.ServiceName)
			return errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
		}
	}
	// scenario that node is moving to another node service
	if ds.Name != *updateNodeRequest.Node.ServiceName {
		return DeleteNode(*updateNodeRequest.Node.Name, dic)
	}

	requests.ReplaceNodeModelFieldsWithDTO(&node, updateNodeRequest.Node)
	var edgexErr errors.EdgeX
	if node.ProfileName != "" {
		edgexErr = updateAssociatedProfile(node.ProfileName, dic)
		if edgexErr != nil {
			return errors.NewCommonEdgeXWrapper(edgexErr)
		}
	}

	edgexErr = cache.Nodes().Update(node)
	if edgexErr != nil {
		errMsg := fmt.Sprintf("failed to update node %s", node.Name)
		return errors.NewCommonEdgeX(errors.KindServerError, errMsg, edgexErr)
	}
	lc.Debugf("node %s updated", node.Name)

	driver := container.ProtocolDriverFrom(dic.Get)
	err := driver.UpdateNode(node.Name, node.Protocols, node.AdminState)
	if err == nil {
		lc.Debugf("Invoked driver.UpdateNode callback for %s", node.Name)
	} else {
		errMsg := fmt.Sprintf("driver.UpdateNode callback failed for %s", node.Name)
		return errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
	}

	autoEventManager := container.AutoEventManagerFrom(dic.Get)
	if node.AdminState == models.Locked {
		lc.Debugf("stopping AutoEvents for the locked node %s", node.Name)
		autoEventManager.StopForNode(node.Name)
	} else {
		lc.Debugf("starting AutoEvents for node %s", node.Name)
		autoEventManager.RestartForNode(node.Name)
	}
	return nil
}

func DeleteNode(name string, dic *di.Container) errors.EdgeX {
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	// check the node exist and stop its autoevents
	node, ok := cache.Nodes().ForName(name)
	if ok {
		lc.Debugf("stopping AutoEvents for node %s", node.Name)
		container.AutoEventManagerFrom(dic.Get).StopForNode(node.Name)
	} else {
		errMsg := fmt.Sprintf("failed to find node %s", name)
		return errors.NewCommonEdgeX(errors.KindInvalidId, errMsg, nil)
	}

	// remove the node in cache
	edgexErr := cache.Nodes().RemoveByName(name)
	if edgexErr != nil {
		errMsg := fmt.Sprintf("failed to remove node %s", node.Name)
		return errors.NewCommonEdgeX(errors.KindServerError, errMsg, edgexErr)
	}
	lc.Debugf("Removed node: %s", node.Name)

	driver := container.ProtocolDriverFrom(dic.Get)
	err := driver.RemoveNode(node.Name, node.Protocols)
	if err == nil {
		lc.Debugf("Invoked driver.RemoveNode callback for %s", node.Name)
	} else {
		errMsg := fmt.Sprintf("driver.RemoveNode callback failed for %s", node.Name)
		return errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
	}

	reqFailsTracker := container.AllowedRequestFailuresTrackerFrom(dic.Get)
	reqFailsTracker.Remove(node.Name)

	return nil
}

func AddProvisionWatcher(addProvisionWatcherRequest requests.AddProvisionWatcherRequest, dic *di.Container) errors.EdgeX {
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	provisionWatcher := dtos.ToProvisionWatcherModel(addProvisionWatcherRequest.ProvisionWatcher)

	edgexErr := updateAssociatedProfile(provisionWatcher.DiscoveredNode.ProfileName, dic)
	if edgexErr != nil {
		return errors.NewCommonEdgeXWrapper(edgexErr)
	}

	edgexErr = cache.ProvisionWatchers().Add(provisionWatcher)
	if edgexErr != nil {
		errMsg := fmt.Sprintf("failed to add provision watcher %s", provisionWatcher.Name)
		return errors.NewCommonEdgeX(errors.KindServerError, errMsg, edgexErr)
	}

	lc.Debugf("provision watcher %s added", provisionWatcher.Name)
	return nil
}

func UpdateProvisionWatcher(updateProvisionWatcherRequest requests.UpdateProvisionWatcherRequest, dic *di.Container) errors.EdgeX {
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	ds := container.NodeServiceFrom(dic.Get)

	provisionWatcher, exist := cache.ProvisionWatchers().ForName(*updateProvisionWatcherRequest.ProvisionWatcher.Name)
	if !exist {
		if ds.Name == *updateProvisionWatcherRequest.ProvisionWatcher.ServiceName {
			var newProvisionWatcher models.ProvisionWatcher
			requests.ReplaceProvisionWatcherModelFieldsWithDTO(&newProvisionWatcher, updateProvisionWatcherRequest.ProvisionWatcher)
			req := requests.NewAddProvisionWatcherRequest(dtos.FromProvisionWatcherModelToDTO(newProvisionWatcher))
			return AddProvisionWatcher(req, dic)
		} else {
			errMsg := fmt.Sprintf("failed to find provision watcher %s", *updateProvisionWatcherRequest.ProvisionWatcher.ServiceName)
			return errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
		}
	}
	if ds.Name != *updateProvisionWatcherRequest.ProvisionWatcher.ServiceName {
		return DeleteProvisionWatcher(*updateProvisionWatcherRequest.ProvisionWatcher.Name, dic)
	}

	requests.ReplaceProvisionWatcherModelFieldsWithDTO(&provisionWatcher, updateProvisionWatcherRequest.ProvisionWatcher)
	edgexErr := updateAssociatedProfile(provisionWatcher.DiscoveredNode.ProfileName, dic)
	if edgexErr != nil {
		return errors.NewCommonEdgeXWrapper(edgexErr)
	}

	edgexErr = cache.ProvisionWatchers().Update(provisionWatcher)
	if edgexErr != nil {
		errMsg := fmt.Sprintf("failed to update provision watcher %s", provisionWatcher.Name)
		return errors.NewCommonEdgeX(errors.KindServerError, errMsg, edgexErr)
	}

	lc.Debugf("provision watcher %s updated", provisionWatcher.Name)
	return nil
}

func DeleteProvisionWatcher(name string, dic *di.Container) errors.EdgeX {
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	err := cache.ProvisionWatchers().RemoveByName(name)
	if err != nil {
		errMsg := fmt.Sprintf("failed to remove provision watcher %s", name)
		return errors.NewCommonEdgeX(errors.KindInvalidId, errMsg, err)
	}

	lc.Debugf("removed provision watcher %s", name)
	return nil
}

func UpdateNodeService(updateNodeServiceRequest requests.UpdateNodeServiceRequest, dic *di.Container) errors.EdgeX {
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	ds := container.NodeServiceFrom(dic.Get)

	// we use request.Service.Name to identify the node service (i.e. it's not updatable)
	// so if the request's service name is inconsistent with node service name
	// we should not update it.
	if ds.Name != *updateNodeServiceRequest.Service.Name {
		errMsg := fmt.Sprintf("failed to identify node service %s", *updateNodeServiceRequest.Service.Name)
		return errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
	}

	ds.AdminState = models.AdminState(*updateNodeServiceRequest.Service.AdminState)
	ds.Labels = updateNodeServiceRequest.Service.Labels

	lc.Debug("node service updated")
	return nil
}

// updateAssociatedProfile updates the profile specified in AddNodeRequest or UpdateNodeRequest or AddProvisionWatcherRequest or UpdateProvisionWatcherRequest
// to stay consistent with core metadata.
func updateAssociatedProfile(profileName string, dic *di.Container) errors.EdgeX {
	dpc := bootstrapContainer.ProfileClientFrom(dic.Get)

	res, err := dpc.ProfileByName(context.Background(), profileName)
	if err != nil {
		errMsg := fmt.Sprintf("failed to retrieve profile %s from metadata", profileName)
		return errors.NewCommonEdgeX(errors.KindInvalidId, errMsg, err)
	}

	_, exist := cache.Profiles().ForName(profileName)
	if !exist {
		err = cache.Profiles().Add(dtos.ToProfileModel(res.Profile))
		if err != nil {
			errMsg := fmt.Sprintf("failed to add profile %s", profileName)
			return errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
		}
		return nil
	}
	err = cache.Profiles().Update(dtos.ToProfileModel(res.Profile))
	if err != nil {
		errMsg := fmt.Sprintf("failed to to update profile %s in cache, using the original one", profileName)
		return errors.NewCommonEdgeX(errors.KindServerError, errMsg, err)
	}

	return nil
}
