//
// Copyright (C) 2022-2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"errors"
	"testing"

	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/interfaces/mocks"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	clientMocks "github.com/edgexfoundry/go-mod-core-contracts/v4/clients/interfaces/mocks"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	mocks2 "github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger/mocks"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/responses"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	msgMocks "github.com/edgexfoundry/go-mod-messaging/v4/messaging/mocks"

	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/config"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

	testUUIDString = "ca93c8fa-9919-4ec5-85d3-f81b2b6a7bc1"
)

var TestProtocols map[string]dtos.ProtocolProperties

func buildEvent() dtos.Event {
	event := dtos.NewEvent(TestProfile, TestNodeWithTags, TestCommandWithTags)
	value := string(make([]byte, 1000))
	_ = event.AddSimpleReading(TestResourceWithTags, common.ValueTypeString, value)
	event.Id = testUUIDString
	event.Readings[0].Id = testUUIDString
	return event
}

func NewMockDIC() *di.Container {
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

	profile := responses.ProfileResponse{
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
	dpcMock := &clientMocks.ProfileClient{}
	dpcMock.On("ProfileByName", context.Background(), TestProfile).Return(profile, nil)

	pwcMock := &clientMocks.ProvisionWatcherClient{}
	pwcMock.On("ProvisionWatchersByServiceName", context.Background(), TestNodeService, 0, -1).Return(responses.MultiProvisionWatchersResponse{}, nil)

	mockMetricsManager := &mocks.MetricsManager{}
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
	})
}

func TestSendEvent(t *testing.T) {
	event := buildEvent()
	req := requests.NewAddEventRequest(event)
	bytes, _, err := req.Encode()
	require.NoError(t, err)
	eventSize := len(bytes) / 1024 // to kilobyte

	tests := []struct {
		name          string
		event         *dtos.Event
		maxEventSize  int64
		eventTooLarge bool
	}{
		{"Valid, unlimited max event size", &event, 0, false},
		{"Valid, publish to message bus", &event, int64(eventSize + 1), false},
		{"Invalid, over max event size", &event, int64(eventSize - 1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dic := NewMockDIC()
			mcMock := &msgMocks.MessageClient{}
			mcMock.On("PublishWithSizeLimit", mock.Anything, mock.Anything, int64(0)).Return(nil)
			mcMock.On("PublishWithSizeLimit", mock.Anything, mock.Anything, int64(eventSize+1)).Return(nil)
			mcMock.On("PublishWithSizeLimit", mock.Anything, mock.Anything, int64(eventSize-1)).Return(errors.New("message size exceed limit"))

			dic.Update(di.ServiceConstructorMap{
				container.ConfigurationName: func(get di.Get) interface{} {
					return &config.ConfigurationStruct{
						MaxEventSize: tt.maxEventSize,
					}
				},
				bootstrapContainer.MessagingClientName: func(get di.Get) interface{} {
					return mcMock
				},
			})

			InitializeSentMetrics(logger.NewMockClient(), dic)

			SendEvent(tt.event, testUUIDString, dic)
			mcMock.AssertNumberOfCalls(t, "PublishWithSizeLimit", 1)
			if tt.eventTooLarge {
				assert.Equal(t, int64(0), eventsSent.Count())
				assert.Equal(t, int64(0), readingsSent.Count())
			} else {
				assert.Equal(t, int64(1), eventsSent.Count())
				assert.Equal(t, int64(1), readingsSent.Count())
			}
		})
	}
}

func TestInitializeSentMetrics(t *testing.T) {

	tests := []struct {
		Name                    string
		MetricsManagerAvailable bool
		RegisterError           bool
	}{
		{"Happy Path", true, false},
		{"No MetricsManager", false, false},
		{"Error Registering", true, true},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			eventsSent = nil
			readingsSent = nil

			mockManager := &mocks.MetricsManager{}
			mockLogger := &mocks2.LoggingClient{}

			if test.RegisterError {
				mockManager.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("failed"))
			} else {
				mockManager.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			}

			mockLogger.On("Warn", mock.Anything).Return()
			mockLogger.On("Debugf", mock.Anything, mock.Anything).Return()
			mockLogger.On("Errorf", mock.Anything, mock.Anything, mock.Anything).Return()

			dic := di.NewContainer(di.ServiceConstructorMap{
				bootstrapContainer.LoggingClientInterfaceName: func(get di.Get) interface{} {
					return mockLogger
				},
				bootstrapContainer.MetricsManagerInterfaceName: func(get di.Get) interface{} {
					if test.MetricsManagerAvailable {
						return mockManager
					}

					return nil
				},
			})

			InitializeSentMetrics(mockLogger, dic)

			if test.MetricsManagerAvailable {
				mockManager.AssertNumberOfCalls(t, "Register", 2)
				mockLogger.AssertNumberOfCalls(t, "Warn", 0)

				if test.RegisterError {
					mockLogger.AssertNumberOfCalls(t, "Errorf", 2)
					mockLogger.AssertNumberOfCalls(t, "Debugf", 0)

				} else {
					mockLogger.AssertNumberOfCalls(t, "Errorf", 0)
					mockLogger.AssertNumberOfCalls(t, "Debugf", 2)
				}

			} else {
				mockManager.AssertNumberOfCalls(t, "Register", 0)
				mockLogger.AssertNumberOfCalls(t, "Warn", 1)
				mockLogger.AssertNumberOfCalls(t, "Debugf", 0)
			}
		})
	}
}

func TestAddReadingTags(t *testing.T) {
	dic := NewMockDIC()
	edgexErr := cache.InitCache(TestNodeService, TestNodeService, dic)
	require.NoError(t, edgexErr)
	readingWithTags, err := dtos.NewSimpleReading(TestProfile, TestNodeWithTags, TestResourceWithTags, common.ValueTypeString, "")
	require.NoError(t, err)
	readingWithoutTags, err := dtos.NewSimpleReading(TestProfile, TestNodeWithTags, TestResourceWithoutTags, common.ValueTypeString, "")
	require.NoError(t, err)
	readingResourceNotFound, err := dtos.NewSimpleReading(TestProfile, TestNodeWithTags, "notFound", common.ValueTypeString, "")
	require.NoError(t, err)

	tests := []struct {
		Name         string
		Reading      *dtos.BaseReading
		ExpectedTags dtos.Tags
	}{
		{"Happy Path", &readingWithTags, dtos.Tags{TestResourceTagName: TestResourceTagValue}},
		{"No Tags", &readingWithoutTags, nil},
		{"Resource Not Found", &readingResourceNotFound, nil},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			AddReadingTags(test.Reading)
			if test.ExpectedTags != nil {
				require.NotEmpty(t, test.Reading.Tags)
				assert.Equal(t, test.ExpectedTags, test.Reading.Tags)
			} else {
				assert.Empty(t, test.Reading.Tags)
			}
		})
	}
}

func TestAddEventTags(t *testing.T) {
	dic := NewMockDIC()
	edgexErr := cache.InitCache(TestNodeService, TestNodeService, dic)
	require.NoError(t, edgexErr)

	expectedNodeTags := dtos.Tags{TestNodeTagName: TestNodeTagValue, TestDuplicateTagName: TestNodeTagValue}
	expectedCommandTags := dtos.Tags{TestCommandTagName: TestCommandTagValue, TestDuplicateTagName: TestCommandTagValue}

	tests := []struct {
		Name                    string
		Event                   dtos.Event
		ExpectToHaveNodeTags    bool
		ExpectToHaveCommandTags bool
	}{
		{"Happy Path", dtos.NewEvent(TestProfile, TestNodeWithTags, TestCommandWithTags), true, true},
		{"No Tags", dtos.NewEvent(TestProfile, TestNodeWithoutTags, TestCommandWithoutTags), false, false},
		{"No Node Tags", dtos.NewEvent(TestProfile, TestNodeWithoutTags, TestCommandWithTags), false, true},
		{"No Command Tags", dtos.NewEvent(TestProfile, TestNodeWithTags, TestCommandWithoutTags), true, false},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			event := test.Event
			AddEventTags(&event)
			if test.ExpectToHaveNodeTags && test.ExpectToHaveCommandTags {
				expectedCommandTags[TestDuplicateTagName] = TestNodeTagValue
			} else {
				expectedCommandTags[TestDuplicateTagName] = TestCommandTagValue
			}
			if !test.ExpectToHaveNodeTags && !test.ExpectToHaveCommandTags {
				require.Empty(t, event.Tags)
			} else {
				require.NotEmpty(t, event.Tags)
			}
			if test.ExpectToHaveNodeTags {
				assert.Subset(t, event.Tags, expectedNodeTags)
			} else {
				assert.NotSubset(t, event.Tags, expectedNodeTags)
			}
			if test.ExpectToHaveCommandTags {
				assert.Subset(t, event.Tags, expectedCommandTags)
			} else {
				assert.NotSubset(t, event.Tags, expectedCommandTags)
			}
		})
	}
}
