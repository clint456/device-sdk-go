//
// Copyright (C) 2024-2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"sync"

	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	contractsCommon "github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/common"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/container"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/controller/http/correlation"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/utils"
	"github.com/edgexfoundry/south-sdk-go/v4/pkg/interfaces"
)

type profileScanLocker struct {
	mux     sync.Mutex
	busyMap map[string]bool
}

var locker = profileScanLocker{busyMap: make(map[string]bool)}

func ProfileScanWrapper(busy chan bool, extdriver interfaces.ExtendedProtocolDriver, req requests.ProfileScanRequest, ctx context.Context, dic *di.Container) {
	locker.mux.Lock()
	b := locker.busyMap[req.NodeName]
	busy <- b
	if b {
		locker.mux.Unlock()
		return
	}
	locker.busyMap[req.NodeName] = true
	locker.mux.Unlock()

	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	dpc := bootstrapContainer.ProfileClientFrom(dic.Get)
	dc := bootstrapContainer.NodeClientFrom(dic.Get)
	if correlation.IdFromContext(ctx) != req.RequestId {
		// ensure the correlation id matches the request id.
		ctx = context.WithValue(ctx, common.CorrelationHeaderKey, req.RequestId)
	}

	utils.PublishProfileScanProgressSystemEvent(req.RequestId, 0, "", ctx, dic)
	lc.Debugf("profile scan triggered with node name '%s' and profile name '%s', Correlation Id: %s", req.NodeName, req.ProfileName, req.RequestId)
	profile, err := extdriver.ProfileScan(req)
	if err != nil {
		errMsg := fmt.Sprintf("failed to trigger profile scan: %v, Correlation Id: %s", err.Error(), req.RequestId)
		utils.PublishProfileScanProgressSystemEvent(req.RequestId, -1, errMsg, ctx, dic)
		lc.Error(errMsg)
		releaseLock(req.NodeName)
		return
	}
	// Add profile to metadata
	profileReq := requests.NewProfileRequest(dtos.FromProfileModelToDTO(profile))
	_, err = dpc.Add(ctx, []requests.ProfileRequest{profileReq})
	if err != nil {
		errMsg := fmt.Sprintf("failed to add node profile '%s': %v, Correlation Id: %s", profile.Name, err, req.RequestId)
		utils.PublishProfileScanProgressSystemEvent(req.RequestId, -1, errMsg, ctx, dic)
		lc.Error(errMsg)
		releaseLock(req.NodeName)
		return
	}
	// Update node
	nodeReq := requests.NewUpdateNodeRequest(dtos.UpdateNode{Name: &req.NodeName, ProfileName: &profile.Name})
	_, err = dc.UpdateWithQueryParams(ctx, []requests.UpdateNodeRequest{nodeReq}, map[string]string{common.BypassValidationQueryParam: contractsCommon.ValueTrue})
	if err != nil {
		errMsg := fmt.Sprintf("failed to update node '%s' with profile '%s': %v, Correlation Id: %s", req.NodeName, profile.Name, err, req.RequestId)
		utils.PublishProfileScanProgressSystemEvent(req.RequestId, -1, errMsg, ctx, dic)
		lc.Error(errMsg)
	} else {
		utils.PublishProfileScanProgressSystemEvent(req.RequestId, 100, "", ctx, dic)
	}

	// ReleaseLock
	releaseLock(req.NodeName)
}

func releaseLock(nodeName string) {
	locker.mux.Lock()
	locker.busyMap[nodeName] = false
	locker.mux.Unlock()
}

func StopProfileScan(dic *di.Container, nodeName string, options map[string]any) errors.EdgeX {
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	extdriver := container.ExtendedProtocolDriverFrom(dic.Get)
	if extdriver == nil {
		return errors.NewCommonEdgeX(errors.KindNotImplemented, "Stop profile scan is not implemented", nil)
	}

	// check node service AdminState
	ds := container.NodeServiceFrom(dic.Get)
	if ds.AdminState == models.Locked {
		return errors.NewCommonEdgeX(errors.KindServiceLocked, "service locked", nil)
	}

	// check requested node exists
	_, exist := cache.Nodes().ForName(nodeName)
	if !exist {
		return errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, fmt.Sprintf("node %s not found", nodeName), nil)
	}

	locker.mux.Lock()
	busy := locker.busyMap[nodeName]
	locker.mux.Unlock()
	if !busy {
		lc.Debugf("no active profile scan process was found")
		return nil
	}

	lc.Debugf("Stopping profile scan for node – %s", nodeName)
	extdriver.StopProfileScan(nodeName, options)
	lc.Debugf("Profile scan for node – %s stop signal is sent", nodeName)

	return nil
}
