// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2020-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"fmt"
	"strings"
	"sync"
	"time"

	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"

	gometrics "github.com/rcrowley/go-metrics"
)

const (
	nodeNameText        = "{NodeName}"
	lastConnectedPrefix = "LastConnected-" + nodeNameText
)

var (
	dc *nodeCache
)

type NodeCache interface {
	ForName(name string) (models.Node, bool)
	All() []models.Node
	Add(node models.Node) errors.EdgeX
	Update(node models.Node) errors.EdgeX
	RemoveByName(name string) errors.EdgeX
	UpdateAdminState(name string, state models.AdminState) errors.EdgeX
	SetLastConnectedByName(name string)
	GetLastConnectedByName(name string) int64
}

type nodeCache struct {
	nodeMap       map[string]*models.Node // key is Node name
	mutex         sync.RWMutex
	dic           *di.Container
	lastConnected map[string]gometrics.Gauge
}

func newNodeCache(nodes []models.Node, dic *di.Container) NodeCache {
	defaultSize := len(nodes)
	dMap := make(map[string]*models.Node, defaultSize)
	dc = &nodeCache{nodeMap: dMap, dic: dic}
	lastConnectedMetrics := make(map[string]gometrics.Gauge)
	for _, d := range nodes {
		dMap[d.Name] = &d
		nodeMetric := gometrics.NewGauge()
		registerMetric(d.Name, nodeMetric, dic)
		lastConnectedMetrics[d.Name] = nodeMetric
	}
	dc.lastConnected = lastConnectedMetrics

	return dc
}

func registerMetric(nodeName string, metric interface{}, dic *di.Container) {
	metricsManager := bootstrapContainer.MetricsManagerFrom(dic.Get)
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	registeredName := strings.Replace(lastConnectedPrefix, nodeNameText, nodeName, 1)

	err := metricsManager.Register(registeredName, metric, map[string]string{"node": nodeName})
	if err != nil {
		lc.Warnf("Unable to register %s metric. Metric will not be reported : %s", registeredName, err.Error())
	} else {
		lc.Infof("%s metric has been registered and will be reported (if enabled)", registeredName)
	}
}

func unregisterMetric(nodeName string, dic *di.Container) {
	metricsManager := bootstrapContainer.MetricsManagerFrom(dic.Get)
	registeredName := strings.Replace(lastConnectedPrefix, nodeNameText, nodeName, 1)

	metricsManager.Unregister(registeredName)
}

// ForName returns a Node with the given node name.
func (d *nodeCache) ForName(name string) (models.Node, bool) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	node, ok := d.nodeMap[name]
	if !ok {
		return models.Node{}, false
	}
	// As the cache node contains pointer fields(map and slice), directly return node may cause concurrent map read
	// or write if the invoker spawn another goroutine to manipulate the node, so returning the clone of node here
	return node.Clone(), ok
}

// All returns the current list of nodes in the cache.
func (d *nodeCache) All() []models.Node {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	i := 0
	nodes := make([]models.Node, len(d.nodeMap))
	for _, node := range d.nodeMap {
		nodes[i] = node.Clone()
		i++
	}
	return nodes
}

// Add adds a new node to the cache. This method is used to populate the
// node cache with pre-existing or recently-added nodes from Core Metadata.
func (d *nodeCache) Add(node models.Node) errors.EdgeX {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.add(node)
}

func (d *nodeCache) add(node models.Node) errors.EdgeX {
	if _, ok := d.nodeMap[node.Name]; ok {
		errMsg := fmt.Sprintf("Node %s has already existed in cache", node.Name)
		return errors.NewCommonEdgeX(errors.KindDuplicateName, errMsg, nil)
	}

	d.nodeMap[node.Name] = &node

	// register the lastConnected metric for the new added node
	nodeMetric := gometrics.NewGauge()
	registerMetric(node.Name, nodeMetric, d.dic)
	d.lastConnected[node.Name] = nodeMetric
	return nil
}

// Update updates the node in the cache
func (d *nodeCache) Update(node models.Node) errors.EdgeX {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if err := d.removeByName(node.Name); err != nil {
		return err
	}
	return d.add(node)
}

// RemoveByName removes the specified node by name from the cache.
func (d *nodeCache) RemoveByName(name string) errors.EdgeX {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.removeByName(name)
}

func (d *nodeCache) removeByName(name string) errors.EdgeX {
	_, ok := d.nodeMap[name]
	if !ok {
		errMsg := fmt.Sprintf("failed to find Node %s in cache", name)
		return errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
	}

	delete(d.nodeMap, name)

	// unregister the lastConnected metric for the removed node
	unregisterMetric(name, d.dic)
	delete(d.lastConnected, name)

	return nil
}

// UpdateAdminState updates the node admin state in cache by name. This method
// is used by the UpdateHandler to trigger update node admin state that's been
// updated directly to Core Metadata.
func (d *nodeCache) UpdateAdminState(name string, state models.AdminState) errors.EdgeX {
	if state != models.Locked && state != models.Unlocked {
		return errors.NewCommonEdgeX(errors.KindContractInvalid, "invalid AdminState", nil)
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	_, ok := d.nodeMap[name]
	if !ok {
		errMsg := fmt.Sprintf("failed to find Node %s in cache", name)
		return errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
	}

	d.nodeMap[name].AdminState = state
	return nil
}

func CheckProfileNotUsed(profileName string) bool {
	for _, node := range dc.nodeMap {
		if node.ProfileName == profileName {
			return false
		}
	}

	return true
}

func Nodes() NodeCache {
	return dc
}

// currentTimestamp returns the current timestamp in nanoseconds
var currentTimestamp = func() int64 {
	return time.Now().UnixNano()
}

func (d *nodeCache) SetLastConnectedByName(name string) {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	g := d.lastConnected[name]
	g.Update(currentTimestamp())
}

func (d *nodeCache) GetLastConnectedByName(name string) int64 {
	g := d.lastConnected[name]
	return g.Value()
}
