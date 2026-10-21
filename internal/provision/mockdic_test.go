// -*- Mode: Go; indent-tabs-mode: t -*-
//
// # Copyright (C) 2023 Intel Corporation
// # Copyright (C) 2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0
package provision

import (
	"context"
	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	bootstrapMocks "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/interfaces/mocks"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	clientMocks "github.com/edgexfoundry/go-mod-core-contracts/v4/clients/interfaces/mocks"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/responses"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/config"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/container"
	"github.com/stretchr/testify/mock"
)

const (
	TestNodeService         = "testNodeService"
	TestNodeWithTags        = "testNodeWithTags"
	TestNodeWithoutTags     = "testNodeWithoutTags"
	TestProfile             = "testProfile"
	TestResourceWithTags    = "testResourceWithTags"
	TestResourceWithoutTags = "testResourceWithoutTags"
	TestCommandWithTags     = "testCommandWithTags"
	TestCommandWithoutTags  = "testCommandWithoutTags"
	TestResourceTagName     = "testResourceTagName"
	TestResourceTagValue    = "testResourceTagValue"
	TestCommandTagName      = "testCommandTagName"
	TestCommandTagValue     = "testCommandTagValue"
	TestNodeTagName         = "testNodeTagName"
	TestNodeTagValue        = "testNodeTagValue"
	TestDuplicateTagName    = "testDuplicateTagName"
)

var profile = responses.ProfileResponse{
	Profile: dtos.Profile{
		ProfileBasicInfo: dtos.ProfileBasicInfo{Name: TestProfile},
		Resources: []dtos.Resource{
			{
				Name: TestResourceWithTags,
				Tags: dtos.Tags{
					TestResourceTagName: TestResourceTagValue,
				},
			},
			{
				Name: TestResourceWithoutTags,
			},
		},
		Commands: []dtos.Command{
			{
				Name: TestCommandWithTags,
				Tags: dtos.Tags{
					TestCommandTagName:   TestCommandTagValue,
					TestDuplicateTagName: TestCommandTagValue,
				},
			},
			{
				Name: TestCommandWithoutTags,
			},
		},
	},
}

func NewMockDIC() (*di.Container, *clientMocks.ProfileClient) {
	configuration := &config.ConfigurationStruct{
		Node: config.NodeInfo{MaxCmdOps: 1},
	}
	nodeService := &models.NodeService{Name: TestNodeService}

	nodes := responses.MultiNodesResponse{
		Nodes: []dtos.Node{
			{
				Name:        TestNodeWithTags,
				ProfileName: TestProfile,
				Tags: dtos.Tags{
					TestNodeTagName:      TestNodeTagValue,
					TestDuplicateTagName: TestNodeTagValue,
				},
			},
			{
				Name:        TestNodeWithoutTags,
				ProfileName: TestProfile,
			},
		},
	}
	dcMock := &clientMocks.NodeClient{}
	dcMock.On("NodesByServiceName", context.Background(), TestNodeService, 0, -1).Return(nodes, nil)

	dpcMock := &clientMocks.ProfileClient{}
	dpcMock.On("ProfileByName", context.Background(), TestProfile).Return(profile, nil)

	pwcMock := &clientMocks.ProvisionWatcherClient{}
	pwcMock.On("ProvisionWatchersByServiceName", context.Background(), TestNodeService, 0, -1).Return(responses.MultiProvisionWatchersResponse{}, nil)

	mockMetricsManager := &bootstrapMocks.MetricsManager{}
	mockMetricsManager.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockMetricsManager.On("Unregister", mock.Anything)

	return di.NewContainer(di.ServiceConstructorMap{
		container.ConfigurationName: func(get di.Get) interface{} {
			return configuration
		},
		container.NodeServiceName: func(get di.Get) any {
			return nodeService
		},
		bootstrapContainer.LoggingClientInterfaceName: func(get di.Get) interface{} {
			return logger.NewMockClient()
		},
		bootstrapContainer.NodeClientName: func(get di.Get) interface{} {
			return dcMock
		},
		bootstrapContainer.ProfileClientName: func(get di.Get) interface{} {
			return dpcMock
		},
		bootstrapContainer.ProvisionWatcherClientName: func(get di.Get) interface{} {
			return pwcMock
		},
		bootstrapContainer.MetricsManagerInterfaceName: func(get di.Get) interface{} {
			return mockMetricsManager
		},
	}), dpcMock
}
