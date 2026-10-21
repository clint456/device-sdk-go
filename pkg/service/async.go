// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018-2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"regexp"

	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	contractsCommon "github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"

	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/common"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/container"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/transformer"
	sdkModels "github.com/edgexfoundry/south-sdk-go/v4/pkg/models"
)

// processAsyncResults processes readings that are pushed from
// a DS implementation. Each is reading is optionally transformed
// before being pushed to Core Data.
// In this function, AsyncBufferSize is used to create a buffer for
// processing AsyncValues concurrently, so that events may arrive
// out-of-order in core-data / app service when AsyncBufferSize value
// is greater than or equal to two. Alternatively, we can process
// AsyncValues one by one in the same order by changing the AsyncBufferSize
// value to one.
func (s *nodeService) processAsyncResults(ctx context.Context, dic *di.Container) {
	working := make(chan bool, s.config.Node.AsyncBufferSize)
	for {
		select {
		case <-ctx.Done():
			return
		case acv := <-s.asyncCh:
			go s.sendAsyncValues(acv, working, dic)
		}
	}
}

// sendAsyncValues convert AsyncValues to event and send the event to CoreData
func (s *nodeService) sendAsyncValues(acv *sdkModels.AsyncValues, working chan bool, dic *di.Container) {
	working <- true
	defer func() {
		<-working
	}()

	// Update the LastConnected metric in nodeCache
	cache.Nodes().SetLastConnectedByName(acv.NodeName)

	if len(acv.CommandValues) == 0 {
		s.lc.Error("Skip sending AsyncValues because the CommandValues is empty.")
		return
	}
	if len(acv.CommandValues) > 1 && acv.SourceName == "" {
		s.lc.Error("Skip sending AsyncValues because the SourceName is empty.")
		return
	}
	// We can use the first reading's ResourceName as the SourceName
	// when the CommandValues contains only one reading and the AsyncValues's SourceName is empty.
	if len(acv.CommandValues) == 1 && acv.SourceName == "" {
		acv.SourceName = acv.CommandValues[0].ResourceName
	}

	configuration := container.ConfigurationFrom(dic.Get)
	event, err := transformer.CommandValuesToEventDTO(acv.CommandValues, acv.NodeName, acv.SourceName, configuration.Node.DataTransform, dic)
	if err != nil {
		s.lc.Errorf("failed to transform CommandValues to Event: %v", err)
		return
	}

	common.SendEvent(event, "", dic)
}

// processAsyncFilterAndAdd filter and add nodes discovered by
// node service protocol discovery.
func (s *nodeService) processAsyncFilterAndAdd(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case nodes := <-s.nodeCh:
			ctx := context.Background()
			pws := cache.ProvisionWatchers().All()
			for _, d := range nodes {
				for _, pw := range pws {
					if pw.AdminState == models.Locked {
						s.lc.Debugf("Skip th locked provision watcher %v", pw.Name)
						continue
					}
					if s.checkAllowList(d, pw) && s.checkBlockList(d, pw) {
						if _, ok := cache.Nodes().ForName(d.Name); ok {
							s.lc.Debugf("Candidate discovered node %s already existed", d.Name)
							break
						}

						s.lc.Infof("Adding discovered node %s to Metadata", d.Name)
						node := models.Node{
							Name:           d.Name,
							Description:    d.Description,
							ProfileName:    pw.DiscoveredNode.ProfileName,
							Protocols:      d.Protocols,
							Labels:         d.Labels,
							ServiceName:    s.serviceKey,
							AdminState:     pw.DiscoveredNode.AdminState,
							OperatingState: models.Up,
							Direction:      models.South,
							AutoEvents:     pw.DiscoveredNode.AutoEvents,
							Properties:     pw.DiscoveredNode.Properties,
						}

						req := requests.NewAddNodeRequest(dtos.FromNodeModelToDTO(node))
						_, err := bootstrapContainer.NodeClientFrom(s.dic.Get).AddWithQueryParams(ctx, []requests.AddNodeRequest{req},
							map[string]string{common.BypassValidationQueryParam: contractsCommon.ValueTrue})
						if err != nil {
							s.lc.Errorf("failed to create discovered node %s: %v", node.Name, err)
						} else {
							break
						}
					}
				}
			}
			s.lc.Debug("Filtered node addition finished")
		}
	}
}

func (s *nodeService) checkAllowList(d sdkModels.DiscoveredNode, pw models.ProvisionWatcher) bool {
	// ignore the node protocol properties name
	for _, protocol := range d.Protocols {
		matchedCount := 0
		for name, regex := range pw.Identifiers {
			if value, ok := protocol[name]; ok {
				valueString := fmt.Sprintf("%v", value)
				if valueString == "" {
					s.lc.Debugf("Skipping identifier %s, cannot transform %s value '%v' to string type for discovered node %s", name, name, value, d.Name)
					continue
				}
				matched, err := regexp.MatchString(regex, valueString)
				if !matched || err != nil {
					s.lc.Debugf("Discovered Node %s %s value '%v' did not match PW identifier: %s", d.Name, name, value, regex)
					break
				}
				matchedCount += 1
			}
		}
		// match succeed on all identifiers
		if matchedCount == len(pw.Identifiers) {
			return true
		}
	}
	return false
}

func (s *nodeService) checkBlockList(d sdkModels.DiscoveredNode, pw models.ProvisionWatcher) bool {
	// a candidate should match none of the blocking identifiers
	for name, blacklist := range pw.BlockingIdentifiers {
		// ignore the node protocol properties name
		for _, protocol := range d.Protocols {
			if value, ok := protocol[name]; ok {
				valueString := fmt.Sprintf("%v", value)
				if valueString == "" {
					s.lc.Debugf("Skipping identifier %s, cannot transform %s value '%v' to string type for discovered node %s", name, name, value, d.Name)
					continue
				}
				for _, v := range blacklist {
					if valueString == v {
						s.lc.Debugf("Discovered Node %s %s value cannot be %v", d.Name, name, value)
						return false
					}
				}
			}
		}
	}
	return true
}
