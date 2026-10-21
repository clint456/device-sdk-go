//
// Copyright (C) 2022-2023 Intel Corporation
// Copyright (C) 2023-2026 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package interfaces

import (
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"

	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"

	sdkModels "github.com/edgexfoundry/south-sdk-go/v4/pkg/models"

	"github.com/labstack/echo/v4"
)

// UpdatableConfig interface allows services to have custom configuration populated from configuration stored
// in the Configuration Provider (aka Consul). Services using custom configuration must implement this interface
// on their custom configuration, even if they do not use Configuration Provider. If they do not use the
// Configuration Provider they can have a dummy implementation of this interface.
// This wraps the actual interface from go-mod-bootstrap so node service code doesn't have to have the additional
// direct import of go-mod-bootstrap.
type UpdatableConfig interface {
	interfaces.UpdatableConfig
}

// Authentication is a typed boolean for AddRoute calls
type Authentication bool

const (
	Unauthenticated Authentication = false
	Authenticated   Authentication = true
)

// NodeServiceSDK defines the interface for an EdgeX Node Service SDK
type NodeServiceSDK interface {
	// AddNode adds a new Node to the Node Service and Core Metadata
	// Returns new Node id or non-nil error.
	AddNode(node models.Node) (string, error)
	// Nodes return all managed Nodes from cache
	Nodes() []models.Node
	// GetNodeByName returns the Node by its name if it exists in the cache, or returns an error.
	GetNodeByName(name string) (models.Node, error)
	// UpdateNode updates the Node in the cache and ensures that the
	// copy in Core Metadata is also updated.
	UpdateNode(node models.Node) error
	// RemoveNodeByName removes the specified Node by name from the cache and ensures that the
	// instance in Core Metadata is also removed.
	RemoveNodeByName(name string) error
	// AddProfile adds a new Profile to the Node Service and Core Metadata
	// Returns new Profile id or non-nil error.
	AddProfile(profile models.Profile) (string, error)
	// Profiles return all managed Profiles from cache
	Profiles() []models.Profile
	// GetProfileByName returns the Profile by its name if it exists in the cache, or returns an error.
	GetProfileByName(name string) (models.Profile, error)
	// UpdateProfile updates the Profile in the cache and ensures that the
	// copy in Core Metadata is also updated.
	UpdateProfile(profile models.Profile) error
	// RemoveProfileByName removes the specified Profile by name from the cache and ensures that the
	// instance in Core Metadata is also removed.
	RemoveProfileByName(name string) error
	// AddProvisionWatcher adds a new Watcher to the cache and Core Metadata
	// Returns new Watcher id or non-nil error.
	AddProvisionWatcher(watcher models.ProvisionWatcher) (string, error)
	// ProvisionWatchers return all managed Watchers from cache
	ProvisionWatchers() []models.ProvisionWatcher
	// GetProvisionWatcherByName returns the Watcher by its name if it exists in the cache, or returns an error.
	GetProvisionWatcherByName(name string) (models.ProvisionWatcher, error)
	// UpdateProvisionWatcher updates the Watcher in the cache and ensures that the
	// copy in Core Metadata is also updated.
	UpdateProvisionWatcher(watcher models.ProvisionWatcher) error
	// RemoveProvisionWatcher removes the specified Watcher by name from the cache and ensures that the
	// instance in Core Metadata is also removed.
	RemoveProvisionWatcher(name string) error
	// Resource retrieves the specific Resource instance from cache according to
	// the Node name and Node Resource name
	Resource(nodeName string, resource string) (models.Resource, bool)
	// Command retrieves the specific Command instance from cache according to
	// the Node name and Command name
	Command(nodeName string, commandName string) (models.Command, bool)
	// AddNodeAutoEvent adds a new AutoEvent to the Node with given name
	AddNodeAutoEvent(nodeName string, event models.AutoEvent) error
	// RemoveNodeAutoEvent removes an AutoEvent from the Node with given name
	RemoveNodeAutoEvent(nodeName string, event models.AutoEvent) error
	// UpdateNodeOperatingState updates the OperatingState for the Node with given name
	// in Core Metadata
	UpdateNodeOperatingState(name string, state models.OperatingState) error
	// NodeExistsForName returns true if a node exists in cache with the specified name, otherwise it returns false.
	NodeExistsForName(name string) bool
	// PatchNode patches the specified node properties in Core Metadata. Node name is required
	// to be provided in the UpdateNode. Note that all properties of UpdateNode are pointers
	// and anything that is nil will not modify the node. In the case of Arrays and Maps, the whole new value
	// must be sent, as it is applied as an overwrite operation.
	PatchNode(updateNode dtos.UpdateNode) error

	// Run starts this Node Service. This should not be called directly by a node service. Instead,
	// call startup.Bootstrap(...)
	Run() error

	// Name returns the name of this Node Service
	Name() string

	// Version returns the version number of this Node Service
	Version() string

	// AsyncReadingsEnabled returns a bool value to indicate whether the asynchronous reading is enabled.
	AsyncReadingsEnabled() bool

	// AsyncValuesChannel returns a channel to allow developer send asynchronous reading back to SDK.
	AsyncValuesChannel() chan *sdkModels.AsyncValues

	// DiscoveredNodeChannel returns a channel to allow developer send discovered nodes back to SDK.
	DiscoveredNodeChannel() chan []sdkModels.DiscoveredNode

	// NodeDiscoveryEnabled returns a bool value to indicate whether node discovery is enabled.
	NodeDiscoveryEnabled() bool

	// DriverConfigs retrieves the driver specific configuration
	DriverConfigs() map[string]string

	// AddCustomRoute allows leveraging the existing internal web server to add routes specific to Node Service.
	AddCustomRoute(route string, authentication Authentication, handler func(e echo.Context) error, methods ...string) error

	// LoadCustomConfig uses the Config Processor from go-mod-bootstrap to attempt to load service's
	// custom configuration. It uses the same command line flags to process the custom config in the same manner
	// as the standard configuration.
	LoadCustomConfig(customConfig UpdatableConfig, sectionName string) error

	// ListenForCustomConfigChanges uses the Config Processor from go-mod-bootstrap to attempt to listen for
	// changes to the specified custom configuration section. LoadCustomConfig must be called previously so that
	// the instance of sdkService.configProcessor has already been set.
	ListenForCustomConfigChanges(configToWatch interface{}, sectionName string, changedCallback func(interface{})) error

	// LoggingClient returns the logger.LoggingClient.
	LoggingClient() logger.LoggingClient

	// SecretProvider returns the interfaces.SecretProvider.
	SecretProvider() interfaces.SecretProvider

	// MetricsManager returns the Metrics Manager used to register counter, gauge, gaugeFloat64 or timer metric types from
	// github.com/rcrowley/go-metrics
	MetricsManager() interfaces.MetricsManager

	// PublishNodeDiscoveryProgressSystemEvent publishes a node discovery progress system event through the EdgeX message bus
	PublishNodeDiscoveryProgressSystemEvent(progress, discoveredNodeCount int, message string)

	// PublishProfileScanProgressSystemEvent publishes a profile scan progress system event through the EdgeX message bus.
	PublishProfileScanProgressSystemEvent(reqId string, progress int, message string)

	// PublishGenericSystemEvent publishes a generic system event through the EdgeX message bus
	PublishGenericSystemEvent(eventType, action string, details any)
}

// NodeServiceSDKExt extends NodeServiceSDK with additional methods that bypass node validation.
type NodeServiceSDKExt interface {
	NodeServiceSDK
	// AddNodeWithoutValidation adds a new Node to the Node Service and Core Metadata
	// with bypassValidation=true to skip node validation.
	// Returns new Node id or non-nil error.
	AddNodeWithoutValidation(node models.Node) (string, error)
	// UpdateNodeWithoutValidation updates the Node in Core Metadata with bypassValidation=true
	// to skip node validation.
	UpdateNodeWithoutValidation(node models.Node) error
	// PatchNodeWithoutValidation patches the specified node properties in Core Metadata with bypassValidation=true
	// to skip node validation. Node name is required to be provided in the UpdateNode. Note that all properties
	// of UpdateNode are pointers and anything that is nil will not modify the node. In the case of Arrays and Maps,
	// the whole new value must be sent, as it is applied as an overwrite operation.
	PatchNodeWithoutValidation(updateNode dtos.UpdateNode) error
}
