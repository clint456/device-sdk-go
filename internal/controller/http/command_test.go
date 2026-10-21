//
// Copyright (C) 2022-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	bootstrapMocks "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/interfaces/mocks"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	clientMocks "github.com/edgexfoundry/go-mod-core-contracts/v4/clients/interfaces/mocks"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	commonDTO "github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/responses"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	messagingMocks "github.com/edgexfoundry/go-mod-messaging/v4/messaging/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
	sdkCommon "github.com/edgexfoundry/south-sdk-go/v4/internal/common"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/config"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/container"
	"github.com/edgexfoundry/south-sdk-go/v4/pkg/interfaces/mocks"
	sdkModels "github.com/edgexfoundry/south-sdk-go/v4/pkg/models"

	"github.com/labstack/echo/v4"
)

const (
	testService = "test-service"
	testProfile = "test-profile"

	testNode        = "test-node"
	lockedNode      = "locked-node"
	downedNode      = "down-node"
	driverErrorNode = "driver-node"

	testCommand      = "test-command"
	readOnlyCommand  = "ro-command"
	writeOnlyCommand = "wo-command"
	exceedCommand    = "exceed-command"

	testResource      = "test-resource"
	readOnlyResource  = "ro-resource"
	writeOnlyResource = "wo-resource"
	objectResource    = "object-resource"

	testRegexResource = "^t.+-resource"
)

func mockDic() *di.Container {
	nodes := []dtos.Node{
		dtos.Node{
			Name:           testNode,
			AdminState:     models.Unlocked,
			OperatingState: models.Up,
			ServiceName:    testService,
			ProfileName:    testProfile,
		},
		dtos.Node{
			Name:           lockedNode,
			AdminState:     models.Locked,
			OperatingState: models.Up,
			ServiceName:    testService,
			ProfileName:    testProfile,
		},
		dtos.Node{
			Name:           downedNode,
			AdminState:     models.Unlocked,
			OperatingState: models.Down,
			ServiceName:    testService,
			ProfileName:    testProfile,
		},
		dtos.Node{
			Name:           driverErrorNode,
			AdminState:     models.Unlocked,
			OperatingState: models.Unlocked,
			ServiceName:    testService,
			ProfileName:    testProfile,
		},
	}
	nodeResponse := responses.NewMultiNodesResponse("", "", http.StatusOK, 4, nodes)
	profile := dtos.Profile{
		ProfileBasicInfo: dtos.ProfileBasicInfo{
			Name: testProfile,
		},
		Resources: []dtos.Resource{
			dtos.Resource{
				Name: testResource,
				Properties: dtos.ResourceProperties{
					ValueType:    common.ValueTypeString,
					ReadWrite:    common.ReadWrite_RW,
					DefaultValue: "default",
				},
			},
			dtos.Resource{
				Name: readOnlyResource,
				Properties: dtos.ResourceProperties{
					ValueType: common.ValueTypeString,
					ReadWrite: common.ReadWrite_R,
				},
			},
			dtos.Resource{
				Name: writeOnlyResource,
				Properties: dtos.ResourceProperties{
					ValueType: common.ValueTypeString,
					ReadWrite: common.ReadWrite_W,
				},
			},
			dtos.Resource{
				Name: objectResource,
				Properties: dtos.ResourceProperties{
					ValueType: common.ValueTypeObject,
					ReadWrite: common.ReadWrite_RW,
				},
			},
		},
		Commands: []dtos.Command{
			dtos.Command{
				Name:               testCommand,
				ReadWrite:          common.ReadWrite_RW,
				ResourceOperations: []dtos.ResourceOperation{{Resource: testResource, DefaultValue: "default"}},
			},
			dtos.Command{
				Name:               readOnlyCommand,
				ReadWrite:          common.ReadWrite_R,
				ResourceOperations: []dtos.ResourceOperation{{Resource: readOnlyResource}},
			},
			dtos.Command{
				Name:               writeOnlyCommand,
				ReadWrite:          common.ReadWrite_W,
				ResourceOperations: []dtos.ResourceOperation{{Resource: writeOnlyResource}},
			},
			dtos.Command{
				Name:               exceedCommand,
				ReadWrite:          common.ReadWrite_RW,
				ResourceOperations: []dtos.ResourceOperation{{Resource: testResource}, {Resource: testResource}},
			},
		},
	}
	profileResponse := responses.NewProfileResponse("", "", http.StatusOK, profile)
	provisionWatcherResponse := responses.NewMultiProvisionWatchersResponse("", "", http.StatusOK, 0, nil)
	commandValue := &sdkModels.CommandValue{
		ResourceName: testResource,
		Type:         common.ValueTypeString,
		Value:        "test",
	}

	mockNodeClient := &clientMocks.NodeClient{}
	mockNodeClient.On("NodesByServiceName", context.Background(), testService, 0, -1).Return(nodeResponse, nil)
	mockProfileClient := &clientMocks.ProfileClient{}
	mockProfileClient.On("ProfileByName", context.Background(), testProfile).Return(profileResponse, nil)
	mockProvisionWatcherClient := &clientMocks.ProvisionWatcherClient{}
	mockProvisionWatcherClient.On("ProvisionWatchersByServiceName", context.Background(), testService, 0, -1).Return(provisionWatcherResponse, nil)
	mockDriver := &mocks.ProtocolDriver{}
	mockDriver.On("HandleReadCommands", testNode, mock.Anything, mock.Anything).Return([]*sdkModels.CommandValue{commandValue}, nil)
	mockDriver.On("HandleReadCommands", driverErrorNode, mock.Anything, mock.Anything).Return(nil, errors.New("ProtocolDriver returned error"))
	mockDriver.On("HandleWriteCommands", testNode, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockDriver.On("HandleWriteCommands", driverErrorNode, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("ProtocolDriver returned error"))

	mockMetricsManager := &bootstrapMocks.MetricsManager{}
	mockMetricsManager.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockMetricsManager.On("Unregister", mock.Anything)

	dic := di.NewContainer(di.ServiceConstructorMap{
		container.ConfigurationName: func(get di.Get) any {
			return &config.ConfigurationStruct{
				Node: config.NodeInfo{
					MaxCmdOps: 1,
				},
			}
		},
		bootstrapContainer.LoggingClientInterfaceName: func(get di.Get) any {
			return logger.NewMockClient()
		},
		bootstrapContainer.NodeClientName: func(get di.Get) any {
			return mockNodeClient
		},
		bootstrapContainer.ProfileClientName: func(get di.Get) any {
			return mockProfileClient
		},
		bootstrapContainer.ProvisionWatcherClientName: func(get di.Get) any {
			return mockProvisionWatcherClient
		},
		container.ProtocolDriverName: func(get di.Get) any {
			return mockDriver
		},
		container.NodeServiceName: func(get di.Get) any {
			return &models.NodeService{
				Name:       testService,
				AdminState: models.Unlocked,
			}
		},
		bootstrapContainer.MetricsManagerInterfaceName: func(get di.Get) interface{} {
			return mockMetricsManager
		},
		container.AllowedRequestFailuresTrackerName: func(get di.Get) any {
			return container.NewAllowedFailuresTracker()
		},
	})

	return dic
}

func TestRestController_GetCommand(t *testing.T) {
	e := echo.New()
	dic := mockDic()

	edgexErr := cache.InitCache(testService, testService, dic)
	require.NoError(t, edgexErr)

	controller := NewRestController(e, dic, testService)
	assert.NotNil(t, controller)

	tests := []struct {
		name               string
		nodeName           string
		commandName        string
		expectedStatusCode int
	}{
		{"valid - read node resource", testNode, testResource, http.StatusOK},
		{"valid - read node command", testNode, testCommand, http.StatusOK},
		{"valid - read regex node resource", testNode, testRegexResource, http.StatusOK},
		{"invalid - node name parameter is empty", "", testResource, http.StatusBadRequest},
		{"invalid - command is empty", testNode, "", http.StatusBadRequest},
		{"invalid - node name not found", "notFound", testCommand, http.StatusNotFound},
		{"invalid - command name not found", testNode, "notFound", http.StatusNotFound},
		{"invalid - node is LOCKED", lockedNode, testResource, http.StatusLocked},
		{"invalid - node OperatingState is DOWN", downedNode, testResource, http.StatusLocked},
		{"invalid - node resource is write-only", testNode, writeOnlyResource, http.StatusMethodNotAllowed},
		{"invalid - node command is write-only", testNode, writeOnlyCommand, http.StatusMethodNotAllowed},
		{"invalid - node command resource operations exceed MaxCmdOps", testNode, exceedCommand, http.StatusInternalServerError},
		{"invalid - error in ProtocolDriver implementation", driverErrorNode, testResource, http.StatusInternalServerError},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, common.ApiNodeNameCommandNameRoute, http.NoBody)

			// Act
			recorder := httptest.NewRecorder()
			c := e.NewContext(req, recorder)
			c.SetParamNames(common.Name, common.Command)
			c.SetParamValues(testCase.nodeName, testCase.commandName)

			err := controller.GetCommand(c)
			assert.NoError(t, err)

			var res responses.EventResponse
			err = json.Unmarshal(recorder.Body.Bytes(), &res)
			require.NoError(t, err)

			// Assert
			assert.Equal(t, common.ApiVersion, res.ApiVersion, "API Version not as expected")
			assert.Equal(t, testCase.expectedStatusCode, recorder.Result().StatusCode, "HTTP status code not as expected")
			assert.Equal(t, testCase.expectedStatusCode, res.StatusCode, "Response status code not as expected")
			if testCase.expectedStatusCode == http.StatusOK {
				assert.Empty(t, res.Message, "Message should be empty when it is successful")
			} else {
				assert.NotEmpty(t, res.Message, "Response message doesn't contain the error message")
			}
		})
	}
}

func TestRestController_GetCommand_ServiceLocked(t *testing.T) {
	e := echo.New()
	dic := mockDic()
	dic.Update(di.ServiceConstructorMap{
		container.NodeServiceName: func(get di.Get) any {
			return &models.NodeService{
				Name:       testService,
				AdminState: models.Locked,
			}
		},
	})

	edgexErr := cache.InitCache(testService, testService, dic)
	require.NoError(t, edgexErr)

	controller := NewRestController(e, dic, testService)
	assert.NotNil(t, controller)

	req := httptest.NewRequest(http.MethodGet, common.ApiNodeNameCommandNameRoute, http.NoBody)

	// Act
	recorder := httptest.NewRecorder()
	c := e.NewContext(req, recorder)
	c.SetParamNames(common.Name, common.Command)
	c.SetParamValues(testNode, testResource)
	err := controller.GetCommand(c)
	assert.NoError(t, err)

	var res responses.EventResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &res)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, common.ApiVersion, res.ApiVersion, "API Version not as expected")
	assert.Equal(t, http.StatusLocked, recorder.Result().StatusCode, "HTTP status code not as expected")
	assert.Equal(t, http.StatusLocked, res.StatusCode, "Response status code not as expected")
	assert.NotEmpty(t, res.Message, "Response message doesn't contain the error message")
}

func TestRestController_GetCommand_ReturnEvent(t *testing.T) {
	e := echo.New()
	dic := mockDic()

	edgexErr := cache.InitCache(testService, testService, dic)
	require.NoError(t, edgexErr)

	controller := NewRestController(e, dic, testService)
	assert.NotNil(t, controller)

	req := httptest.NewRequest(http.MethodGet, common.ApiNodeNameCommandNameRoute, http.NoBody)

	query := req.URL.Query()
	query.Add("ds-returnevent", common.ValueFalse)
	req.URL.RawQuery = query.Encode()
	// Act
	recorder := httptest.NewRecorder()
	c := e.NewContext(req, recorder)
	c.SetParamNames(common.Name, common.Command)
	c.SetParamValues(testNode, testResource)
	err := controller.GetCommand(c)
	assert.NoError(t, err)

	// Assert
	assert.Equal(t, http.StatusOK, recorder.Result().StatusCode, "HTTP status code not as expected")
	assert.Empty(t, recorder.Body.Bytes())
}

func TestRestController_SetCommand(t *testing.T) {
	e := echo.New()
	validRequest := map[string]any{testResource: "value", writeOnlyResource: "value"}
	invalidRequest := map[string]any{"invalid": "test"}
	emptyValueRequest := map[string]any{objectResource: ""}

	dic := mockDic()

	sdkCommon.InitializeSentMetrics(logger.NewMockClient(), dic)
	err := cache.InitCache(testService, testService, dic)
	require.NoError(t, err)

	controller := NewRestController(e, dic, testService)
	assert.NotNil(t, controller)

	tests := []struct {
		name               string
		nodeName           string
		commandName        string
		request            map[string]any
		expectedStatusCode int
	}{
		{"valid - node resource", testNode, testResource, validRequest, http.StatusOK},
		{"valid - write-only node resource", testNode, writeOnlyResource, validRequest, http.StatusOK},
		{"valid - node resource not specified in request body but default value provided", testNode, testResource, invalidRequest, http.StatusOK},
		{"valid - node command", testNode, testCommand, validRequest, http.StatusOK},
		{"valid - write-only node command", testNode, writeOnlyCommand, validRequest, http.StatusOK},
		{"valid - node command not specified in request body but default value provided", testNode, testCommand, invalidRequest, http.StatusOK},
		{"invalid - node name parameter is empty", "", testResource, validRequest, http.StatusBadRequest},
		{"invalid - command is empty", testNode, "", validRequest, http.StatusBadRequest},
		{"invalid - node name not found", "notFound", testResource, validRequest, http.StatusNotFound},
		{"invalid - command name not found", testNode, "notFound", validRequest, http.StatusNotFound},
		{"invalid - node is LOCKED", lockedNode, testResource, validRequest, http.StatusLocked},
		{"invalid - node OperatingState is DOWN", downedNode, testResource, validRequest, http.StatusLocked},
		{"invalid - node resource is read-only", testNode, readOnlyResource, validRequest, http.StatusMethodNotAllowed},
		{"invalid - node command is read-only", testNode, readOnlyCommand, validRequest, http.StatusMethodNotAllowed},
		{"invalid - node command resource operations exceed MaxCmdOps", testNode, exceedCommand, validRequest, http.StatusInternalServerError},
		{"invalid - write empty string to non string node resource", testNode, objectResource, emptyValueRequest, http.StatusBadRequest},
		{"invalid - error in ProtocolDriver implementation", driverErrorNode, testResource, validRequest, http.StatusInternalServerError},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			jsonData, err := json.Marshal(testCase.request)
			require.NoError(t, err)

			reader := strings.NewReader(string(jsonData))
			req := httptest.NewRequest(http.MethodPut, common.ApiNodeNameCommandNameRoute, reader)

			var wg sync.WaitGroup
			if testCase.commandName != writeOnlyCommand && testCase.commandName != writeOnlyResource {
				wg.Add(1)
			}
			messagingClientMock := &messagingMocks.MessageClient{}
			messagingClientMock.On("PublishWithSizeLimit", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
				go func() {
					defer wg.Done()
				}()
			}).Return(nil)
			dic.Update(di.ServiceConstructorMap{
				bootstrapContainer.MessagingClientName: func(get di.Get) any {
					return messagingClientMock
				},
			})

			// Act
			recorder := httptest.NewRecorder()
			c := e.NewContext(req, recorder)
			c.SetParamNames(common.Name, common.Command)
			c.SetParamValues(testCase.nodeName, testCase.commandName)
			err = controller.SetCommand(c)
			assert.NoError(t, err)

			var res commonDTO.BaseResponse
			err = json.Unmarshal(recorder.Body.Bytes(), &res)
			require.NoError(t, err)

			// Assert
			assert.Equal(t, common.ApiVersion, res.ApiVersion, "API Version not as expected")
			assert.Equal(t, testCase.expectedStatusCode, recorder.Result().StatusCode, "HTTP status code not as expected")
			assert.Equal(t, testCase.expectedStatusCode, res.StatusCode, "Response status code not as expected")
			if testCase.expectedStatusCode == http.StatusOK {
				assert.Empty(t, res.Message, "Message should be empty when it is successful")

				wg.Wait()
				if testCase.commandName != writeOnlyCommand && testCase.commandName != writeOnlyResource {
					messagingClientMock.AssertNumberOfCalls(t, "PublishWithSizeLimit", 1)
				}
			} else {
				assert.NotEmpty(t, res.Message, "Response message doesn't contain the error message")
			}
		})
	}
}

func TestRestController_SetCommand_ServiceLocked(t *testing.T) {
	e := echo.New()
	dic := mockDic()
	dic.Update(di.ServiceConstructorMap{
		container.NodeServiceName: func(get di.Get) any {
			return &models.NodeService{
				Name:       testService,
				AdminState: models.Locked,
			}
		},
	})
	validRequest := map[string]any{testResource: "value"}

	edgexErr := cache.InitCache(testService, testService, dic)
	require.NoError(t, edgexErr)

	controller := NewRestController(e, dic, testService)
	assert.NotNil(t, controller)

	jsonData, err := json.Marshal(validRequest)
	require.NoError(t, err)

	reader := strings.NewReader(string(jsonData))
	req := httptest.NewRequest(http.MethodPut, common.ApiNodeNameCommandNameRoute, reader)

	// Act
	recorder := httptest.NewRecorder()
	c := e.NewContext(req, recorder)
	c.SetParamNames(common.Name, common.Command)
	c.SetParamValues(testNode, testResource)
	err = controller.SetCommand(c)
	assert.NoError(t, err)

	var res commonDTO.BaseResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &res)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, common.ApiVersion, res.ApiVersion, "API Version not as expected")
	assert.Equal(t, http.StatusLocked, recorder.Result().StatusCode, "HTTP status code not as expected")
	assert.Equal(t, http.StatusLocked, res.StatusCode, "Response status code not as expected")
	assert.NotEmpty(t, res.Message, "Response message doesn't contain the error message")
}
