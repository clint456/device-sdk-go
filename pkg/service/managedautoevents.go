// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2020 VMware
// Copyright (C) 2021-2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"fmt"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"

	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
)

// AddNodeAutoEvent adds a new AutoEvent to the Node with given name
func (s *nodeService) AddNodeAutoEvent(nodeName string, event models.AutoEvent) error {
	found := false
	node, ok := cache.Nodes().ForName(nodeName)
	if !ok {
		msg := fmt.Sprintf("failed to find node %s in cache", nodeName)
		s.lc.Error(msg)
		return errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, msg, nil)
	}

	for _, e := range node.AutoEvents {
		if e.SourceName == event.SourceName {
			s.lc.Debugf("Updating existing AutoEvent %s for node %s", e.SourceName, nodeName)
			e.Interval = event.Interval
			e.OnChange = event.OnChange
			found = true
			break
		}
	}

	if !found {
		s.lc.Debugf("Adding new AutoEvent %s to node %s", event.SourceName, nodeName)
		node.AutoEvents = append(node.AutoEvents, event)
		err := cache.Nodes().Update(node)
		if err != nil {
			s.lc.Errorf("failed to update node %s with AutoEvent change", nodeName)
			return err
		}
	}
	s.autoEventManager.RestartForNode(nodeName)

	return nil
}

// RemoveNodeAutoEvent removes an AutoEvent from the Node with given name
func (s *nodeService) RemoveNodeAutoEvent(nodeName string, event models.AutoEvent) error {
	node, ok := cache.Nodes().ForName(nodeName)
	if !ok {
		msg := fmt.Sprintf("failed to find node %s cannot in cache", nodeName)
		s.lc.Error(msg)
		return errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, msg, nil)
	}

	for i, e := range node.AutoEvents {
		if e.SourceName == event.SourceName {
			s.lc.Debugf("Removing AutoEvent %s for node %s", e.SourceName, nodeName)
			node.AutoEvents = append(node.AutoEvents[:i], node.AutoEvents[i+1:]...)
			break
		}
	}

	err := cache.Nodes().Update(node)
	if err != nil {
		s.lc.Errorf("failed to update node %s with AutoEvent change", nodeName)
		return err
	}
	s.autoEventManager.RestartForNode(nodeName)

	return nil
}
