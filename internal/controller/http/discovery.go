// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2020-2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	commonDTO "github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/application"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/autodiscovery"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/container"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/controller/http/correlation"
)

func (c *RestController) Discovery(e echo.Context) error {
	request := e.Request()
	writer := e.Response()
	ctx := request.Context()
	ds := container.NodeServiceFrom(c.dic.Get)
	if ds.AdminState == models.Locked {
		err := errors.NewCommonEdgeX(errors.KindServiceLocked, "service locked", nil)
		return c.sendEdgexError(writer, request, err, common.ApiDiscoveryRoute)
	}

	configuration := container.ConfigurationFrom(c.dic.Get)
	if !configuration.Node.Discovery.Enabled {
		err := errors.NewCommonEdgeX(errors.KindServiceUnavailable, "node discovery disabled", nil)
		return c.sendEdgexError(writer, request, err, common.ApiDiscoveryRoute)
	}

	driver := container.ProtocolDriverFrom(c.dic.Get)

	// Use correlation id as request id since there is no request body
	requestId := correlation.IdFromContext(ctx)
	go func() {
		c.lc.Infof("Discovery triggered. Correlation Id: %s", requestId)
		autodiscovery.DiscoveryWrapper(driver, ctx, c.dic)
		c.lc.Infof("Discovery end. Correlation Id: %s", requestId)
	}()

	response := commonDTO.NewBaseResponse(requestId, "Node Discovery is triggered.", http.StatusAccepted)
	return c.sendResponse(writer, request, common.ApiDiscoveryRoute, response, http.StatusAccepted)
}

func (c *RestController) ProfileScan(e echo.Context) error {
	request := e.Request()
	writer := e.Response()
	ctx := request.Context()
	if request.Body != nil {
		defer func() { _ = request.Body.Close() }()
	}

	body, err := io.ReadAll(request.Body)
	if err != nil {
		edgexErr := errors.NewCommonEdgeX(errors.KindServerError, "Failed to read request body", err)
		return c.sendEdgexError(writer, request, edgexErr, common.ApiProfileScanRoute)
	}

	extdriver := container.ExtendedProtocolDriverFrom(c.dic.Get)
	if extdriver == nil {
		edgexErr := errors.NewCommonEdgeX(errors.KindNotImplemented, "Profile scan is not implemented", nil)
		return c.sendEdgexError(writer, request, edgexErr, common.ApiProfileScanRoute)
	}

	req, edgexErr := profileScanValidation(body, ctx, c.dic)
	if edgexErr != nil {
		return c.sendEdgexError(writer, request, edgexErr, common.ApiProfileScanRoute)
	}

	// Detach from the request context so the background scan is not cancelled
	// when the handler returns.
	scanCtx := context.WithoutCancel(ctx)
	busy := make(chan bool)
	go func() {
		c.lc.Infof("Profile scanning is triggered. Correlation Id: %s", req.RequestId)
		application.ProfileScanWrapper(busy, extdriver, req, scanCtx, c.dic)
		c.lc.Infof("Profile scanning is end. Correlation Id: %s", req.RequestId)
	}()
	b := <-busy
	if b {
		edgexErr := errors.NewCommonEdgeX(errors.KindStatusConflict, fmt.Sprintf("Another profile scan process for %s is currently running", req.NodeName), nil)
		return c.sendEdgexError(writer, request, edgexErr, common.ApiProfileScanRoute)
	}

	response := commonDTO.NewBaseResponse(req.RequestId, "Node ProfileScan is triggered.", http.StatusAccepted)
	return c.sendResponse(writer, request, common.ApiProfileScanRoute, response, http.StatusAccepted)
}

func profileScanValidation(request []byte, ctx context.Context, dic *di.Container) (requests.ProfileScanRequest, errors.EdgeX) {
	var req requests.ProfileScanRequest
	// check node service AdminState
	ds := container.NodeServiceFrom(dic.Get)
	if ds.AdminState == models.Locked {
		return req, errors.NewCommonEdgeX(errors.KindServiceLocked, "service locked", nil)
	}

	// parse request payload
	err := req.UnmarshalJSON(request)
	if err != nil {
		return req, errors.NewCommonEdgeX(errors.KindContractInvalid, "failed to parse request body", err)
	}

	// check requested node exists
	_, exist := cache.Nodes().ForName(req.NodeName)
	if !exist {
		return req, errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, fmt.Sprintf("node %s not found", req.NodeName), nil)
	}

	// check profile should not exist
	if len(req.ProfileName) > 0 {
		if _, exist := cache.Profiles().ForName(req.ProfileName); exist {
			return req, errors.NewCommonEdgeX(errors.KindStatusConflict, fmt.Sprintf("profile name %s is duplicated", req.ProfileName), nil)
		}
	} else {
		req.ProfileName = fmt.Sprintf("%s_profile_%d", req.NodeName, time.Now().UnixMilli())
	}

	requestId := req.RequestId
	if len(requestId) == 0 {
		// Use correlation id as request id if request id is not provided
		requestId = correlation.IdFromContext(ctx)
	}

	req = requests.ProfileScanRequest{
		BaseRequest: commonDTO.BaseRequest{
			Versionable: commonDTO.NewVersionable(),
			RequestId:   requestId,
		},
		NodeName:    req.NodeName,
		ProfileName: req.ProfileName,
		Options:     req.Options,
	}

	return req, nil
}

func (c *RestController) StopNodeDiscovery(e echo.Context) error {
	request := e.Request()
	writer := e.Response()

	// URL parameters
	requestId := e.Param(common.RequestId)
	queryParams, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		edgexErr := errors.NewCommonEdgeX(errors.KindServerError, "failed to parse query parameter", err)
		return c.sendEdgexErrorWithRequestId(writer, request, edgexErr, common.ApiDiscoveryByIdRoute, requestId)
	}
	options := make(map[string]any)
	for key, vals := range queryParams {
		options[key] = vals
	}
	edgexErr := autodiscovery.StopNodeDiscovery(c.dic, requestId, options)
	if edgexErr != nil {
		return c.sendEdgexErrorWithRequestId(writer, request, edgexErr, common.ApiDiscoveryByIdRoute, requestId)
	}

	res := commonDTO.NewBaseResponse(requestId, "", http.StatusOK)
	return c.sendResponse(writer, request, common.ApiDiscoveryByIdRoute, res, http.StatusOK)
}

func (c *RestController) StopProfileScan(e echo.Context) error {
	request := e.Request()
	writer := e.Response()

	// URL parameters
	nodeName := e.Param(common.Name)
	queryParams, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		edgexErr := errors.NewCommonEdgeX(errors.KindServerError, "failed to parse query parameter", err)
		return c.sendEdgexError(writer, request, edgexErr, common.ApiProfileScanByNodeNameRoute)
	}
	options := make(map[string]any)
	for key, vals := range queryParams {
		options[key] = vals
	}
	edgexErr := application.StopProfileScan(c.dic, nodeName, options)
	if edgexErr != nil {
		return c.sendEdgexError(writer, request, edgexErr, common.ApiProfileScanByNodeNameRoute)
	}

	res := commonDTO.NewBaseResponse("", "", http.StatusOK)
	return c.sendResponse(writer, request, common.ApiProfileScanByNodeNameRoute, res, http.StatusOK)
}
