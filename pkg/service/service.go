// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018-2025 IOTech Ltd
// Copyright (C) 2019,2023 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package service provides a basic EdgeX Foundry node service implementation
// meant to be embedded in an application, similar in approach to the builtin
// net/http package.
package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/controller"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/handlers"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/startup"
	"github.com/panjf2000/ants/v2"

	"github.com/edgexfoundry/south-sdk-go/v4/internal/autodiscovery"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/autoevent"
	sdkCommon "github.com/edgexfoundry/south-sdk-go/v4/internal/common"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/config"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/container"
	restController "github.com/edgexfoundry/south-sdk-go/v4/internal/controller/http"
	sdkUtils "github.com/edgexfoundry/south-sdk-go/v4/internal/utils"
	"github.com/edgexfoundry/south-sdk-go/v4/pkg/interfaces"
	sdkModels "github.com/edgexfoundry/south-sdk-go/v4/pkg/models"

	bootstrapConfig "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/config"
	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/flags"
	bootstrapInterfaces "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/interfaces"
	bootstrapTypes "github.com/edgexfoundry/go-mod-bootstrap/v4/config"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	edgexErr "github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const EnvInstanceName = "EDGEX_INSTANCE_NAME"

type nodeService struct {
	serviceKey        string
	baseServiceName   string
	lc                logger.LoggingClient
	driver            interfaces.ProtocolDriver
	extdriver         interfaces.ExtendedProtocolDriver
	autoEventManager  interfaces.AutoEventManager
	commonController  *controller.CommonController
	controller        *restController.RestController
	asyncCh           chan *sdkModels.AsyncValues
	nodeCh            chan []sdkModels.DiscoveredNode
	flags             *flags.Default
	overwriteNodes    bool
	overwriteProfiles bool
	nodeServiceModel  *models.NodeService
	config            *config.ConfigurationStruct
	configProcessor   *bootstrapConfig.Processor
	wg                *sync.WaitGroup
	ctx               context.Context
	dic               *di.Container
	pool              *ants.Pool
}

// NewNodeService returns an implementation of interfaces.NodeServiceSDKExt for the specified key, version, and driver.
func NewNodeService(serviceKey string, serviceVersion string, driver interfaces.ProtocolDriver) (interfaces.NodeServiceSDK, error) {
	var service nodeService
	if serviceKey == "" {
		return nil, errors.New("please specify node service name")
	}
	service.serviceKey = serviceKey

	if serviceVersion == "" {
		return nil, errors.New("please specify node service version")
	}
	sdkCommon.ServiceVersion = serviceVersion

	service.driver = driver

	if extdriver, ok := driver.(interfaces.ExtendedProtocolDriver); ok {
		service.extdriver = extdriver
	} else {
		service.extdriver = nil
	}

	service.config = &config.ConfigurationStruct{}
	return interfaces.NodeServiceSDK(&service), nil
}

func (s *nodeService) Run() error {
	var instanceName string
	var overwriteNodes bool
	var overwriteProfiles bool
	startupTimer := startup.NewStartUpTimer(s.serviceKey)

	additionalUsage :=
		"    -op, --overwriteProfiles     Overwrite core-metadata with the versions of the associated node profile files.\n" +
			"                                 *** Use with cation *** Use will clobber existing profiles in core-metadata,\n" +
			"                                 problematic if those profiles were edited by hand intentionally or used by other services.\n" +
			"    -od, --overwriteNodes      Overwrite core-metadata with the versions of the associated node files.\n" +
			"                                 *** Use with cation *** Use will clobber existing nodes in core-metadata,\n" +
			"                                 problematic if those nodes were edited by hand intentionally.\n" +
			"    -i, --instance                  Provides a service name suffix which allows unique instance to be created\n" +
			"                                    If the option is provided, service name will be replaced with \"<name>_<instance>\"\n"
	s.flags = flags.NewWithUsage(additionalUsage)
	s.flags.FlagSet.StringVar(&instanceName, "instance", "", "")
	s.flags.FlagSet.StringVar(&instanceName, "i", "", "")
	s.flags.FlagSet.BoolVar(&overwriteProfiles, "op", false, "")
	s.flags.FlagSet.BoolVar(&overwriteProfiles, "overwriteProfiles", false, "")
	s.flags.FlagSet.BoolVar(&overwriteNodes, "od", false, "")
	s.flags.FlagSet.BoolVar(&overwriteNodes, "overwriteNodes", false, "")
	s.flags.Parse(os.Args[1:])
	s.setServiceName(instanceName)
	s.overwriteNodes = overwriteNodes
	s.overwriteProfiles = overwriteProfiles

	s.config = &config.ConfigurationStruct{}
	s.nodeServiceModel = &models.NodeService{Name: s.serviceKey}

	s.dic = di.NewContainer(di.ServiceConstructorMap{
		container.ConfigurationName: func(get di.Get) any {
			return s.config
		},
		container.NodeServiceName: func(get di.Get) any {
			return s.nodeServiceModel
		},
		container.ProtocolDriverName: func(get di.Get) any {
			return s.driver
		},
		container.ExtendedProtocolDriverName: func(get di.Get) any {
			return s.extdriver
		},
	})

	// set poolSize to config.Node.AsyncBufferSize
	config := container.ConfigurationFrom(s.dic.Get)
	pool, err := ants.NewPool(config.Node.AsyncBufferSize)
	if err != nil {
		return err
	}
	s.pool = pool

	router := echo.New()
	httpServer := handlers.NewHttpServer(router, true, s.serviceKey)

	ctx, cancel := context.WithCancel(context.Background())
	wg, deferred, successful := bootstrap.RunAndReturnWaitGroup(
		ctx,
		cancel,
		s.flags,
		s.serviceKey,
		common.ConfigStemNode,
		s.config,
		nil,
		startupTimer,
		s.dic,
		true,
		bootstrapTypes.ServiceTypeNode,
		[]bootstrapInterfaces.BootstrapHandler{
			newMessageBusBootstrap(s.baseServiceName).messageBusBootstrapHandler,
			handlers.NewServiceMetrics(s.serviceKey).BootstrapHandler, // Must be after Messaging
			handlers.NewClientsBootstrap().BootstrapHandler,
			autoevent.NewBootstrap(s.pool).BootstrapHandler,
			NewBootstrap(s, router).BootstrapHandler,
			autodiscovery.BootstrapHandler,
			handlers.NewStartMessage(s.serviceKey, sdkCommon.ServiceVersion).BootstrapHandler,
			httpServer.BootstrapHandler,
		})

	defer func() {
		deferred()
		s.Stop(false)
		s.pool.Release()
	}()

	if !successful {
		cancel()
		return errors.New("bootstrapping failed")
	}

	err = s.driver.Start()
	if err != nil {
		cancel()
		return fmt.Errorf("failed to Start ProtocolDriver: %v", err)
	}

	wg.Wait()
	return nil
}

// Name returns the name of this Node Service
func (s *nodeService) Name() string {
	return s.serviceKey
}

// Version returns the version number of this Node Service
func (s *nodeService) Version() string {
	return sdkCommon.ServiceVersion
}

// SecretProvider returns the SecretProvider
func (s *nodeService) SecretProvider() bootstrapInterfaces.SecretProvider {
	return bootstrapContainer.SecretProviderFrom(s.dic.Get)
}

// MetricsManager returns the Metrics Manager used to register counter, gauge, gaugeFloat64 or timer metric types from
// github.com/rcrowley/go-metrics
func (s *nodeService) MetricsManager() bootstrapInterfaces.MetricsManager {
	return bootstrapContainer.MetricsManagerFrom(s.dic.Get)
}

// LoggingClient returns the logger.LoggingClient
func (s *nodeService) LoggingClient() logger.LoggingClient {
	if s.lc == nil {
		s.lc = bootstrapContainer.LoggingClientFrom(s.dic.Get)
	}
	return s.lc
}

// AsyncReadingsEnabled returns a bool value to indicate whether the asynchronous reading is enabled.
func (s *nodeService) AsyncReadingsEnabled() bool {
	return s.config.Node.EnableAsyncReadings
}

func (s *nodeService) AsyncValuesChannel() chan *sdkModels.AsyncValues {
	return s.asyncCh
}

// NodeDiscoveryEnabled returns a bool value to indicate whether the node discovery is enabled.
func (s *nodeService) NodeDiscoveryEnabled() bool {
	return s.config.Node.Discovery.Enabled
}

func (s *nodeService) DiscoveredNodeChannel() chan []sdkModels.DiscoveredNode {
	return s.nodeCh
}

// AddCustomRoute allows leveraging the existing internal web server to add routes specific to Node Service.
func (s *nodeService) AddCustomRoute(route string, authentication interfaces.Authentication, handler func(e echo.Context) error, methods ...string) error {
	if authentication == interfaces.Authenticated {
		authenticationHook := handlers.AutoConfigAuthenticationFunc(s.dic)

		return s.controller.AddRoute(route, handler, methods, authenticationHook)
	}
	return s.controller.AddRoute(route, handler, methods)
}

// LoadCustomConfig uses the Config Processor from go-mod-bootstrap to attempt to load service's
// custom configuration. It uses the same command line flags to process the custom config in the same manner
// as the standard configuration.
func (s *nodeService) LoadCustomConfig(customConfig interfaces.UpdatableConfig, sectionName string) error {
	if s.configProcessor == nil {
		s.configProcessor = bootstrapConfig.NewProcessorForCustomConfig(s.flags, s.ctx, s.wg, s.dic)
	}

	if err := s.configProcessor.LoadCustomConfigSection(customConfig, sectionName); err != nil {
		return err
	}

	s.controller.SetCustomConfigInfo(customConfig)
	s.commonController.SetCustomConfigInfo(customConfig)

	return nil
}

// ListenForCustomConfigChanges uses the Config Processor from go-mod-bootstrap to attempt to listen for
// changes to the specified custom configuration section. LoadCustomConfig must be called previously so that
// the instance of svc.configProcessor has already been set.
func (s *nodeService) ListenForCustomConfigChanges(
	configToWatch interface{},
	sectionName string,
	changedCallback func(interface{})) error {
	if s.configProcessor == nil {
		return fmt.Errorf(
			"custom configuration must be loaded before '%s' section can be watched for changes",
			sectionName)
	}

	s.configProcessor.ListenForCustomConfigChanges(configToWatch, sectionName, changedCallback)
	return nil
}

// selfRegister register node service itself onto metadata.
func (s *nodeService) selfRegister() edgexErr.EdgeX {
	localNodeService := models.NodeService{
		Name:        s.serviceKey,
		Labels:      s.config.Node.Labels,
		BaseAddress: bootstrapTypes.DefaultHttpProtocol + "://" + s.config.Service.Host + ":" + strconv.FormatInt(int64(s.config.Service.Port), 10),
		AdminState:  models.Unlocked,
		ServiceType: models.ServiceTypeSouth,
		Properties:  make(map[string]any),
	}
	*s.nodeServiceModel = localNodeService
	ctx := context.WithValue(context.Background(), common.CorrelationHeader, uuid.NewString()) // nolint:staticcheck
	dsc := bootstrapContainer.NodeServiceClientFrom(s.dic.Get)

	s.lc.Debugf("trying to find node service %s", localNodeService.Name)
	res, err := dsc.NodeServiceByName(ctx, localNodeService.Name)
	if err != nil {
		if edgexErr.Kind(err) == edgexErr.KindEntityDoesNotExist {
			s.lc.Infof("node service %s doesn't exist, creating a new one", localNodeService.Name)
			req := requests.NewAddNodeServiceRequest(dtos.FromNodeServiceModelToDTO(localNodeService))
			idRes, err := dsc.Add(ctx, []requests.AddNodeServiceRequest{req})
			if err != nil {
				s.lc.Errorf("failed to add node service %s: %v", localNodeService.Name, err)
				return err
			}
			s.nodeServiceModel.Id = idRes[0].Id
			s.lc.Debugf("new node service id: %s", localNodeService.Id)
		} else {
			s.lc.Errorf("failed to find node service %s", localNodeService.Name)
			return err
		}
	} else {
		s.lc.Infof("node service %s exists, updating it", s.serviceKey)
		req := requests.NewUpdateNodeServiceRequest(dtos.FromNodeServiceModelToUpdateDTO(localNodeService))
		req.Service.Id = nil
		_, err = dsc.Update(ctx, []requests.UpdateNodeServiceRequest{req})
		if err != nil {
			s.lc.Errorf("failed to update node service %s with local config: %v", localNodeService.Name, err)
			oldNodeService := dtos.ToNodeServiceModel(res.Service)
			*s.nodeServiceModel = oldNodeService
		}
	}

	return nil
}

// DriverConfigs retrieves the driver specific configuration
func (s *nodeService) DriverConfigs() map[string]string {
	return s.config.Driver
}

// Stop shuts down the Service
func (s *nodeService) Stop(force bool) {
	err := s.driver.Stop(force)
	if err != nil {
		s.lc.Errorf(err.Error())
	}
}

func (s *nodeService) setServiceName(instanceName string) {
	envValue := os.Getenv(EnvInstanceName)
	if len(envValue) > 0 {
		instanceName = envValue
	}

	// Need to capture the base service name to use when loading Provision Watchers so that all instances find the defined provision watchers.
	s.baseServiceName = s.serviceKey

	if len(instanceName) > 0 {
		s.serviceKey = s.serviceKey + "_" + instanceName
	}
}

// PublishNodeDiscoveryProgressSystemEvent sends a system event to the EdgeX Message Bus
// indicating the progress of the node discovery process.
func (s *nodeService) PublishNodeDiscoveryProgressSystemEvent(progress, discoveredNodeCount int, message string) {
	reqId := container.DiscoveryRequestIdFrom(s.dic.Get)
	sdkUtils.PublishNodeDiscoveryProgressSystemEvent(reqId, progress, discoveredNodeCount, message, s.ctx, s.dic)
}

// PublishProfileScanProgressSystemEvent sends a system event to the EdgeX Message Bus
// indicating the progress of the profile scan process.
func (s *nodeService) PublishProfileScanProgressSystemEvent(reqId string, progress int, message string) {
	sdkUtils.PublishProfileScanProgressSystemEvent(reqId, progress, message, s.ctx, s.dic)
}

// PublishGenericSystemEvent sends a system event to the EdgeX Message Bus
func (s *nodeService) PublishGenericSystemEvent(eventType, action string, details any) {
	sdkUtils.PublishGenericSystemEvent(eventType, action, details, s.ctx, s.dic)
}
