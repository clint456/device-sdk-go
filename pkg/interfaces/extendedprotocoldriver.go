// Copyright (C) 2024 IOTech Ltd

package interfaces

import (
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	model "github.com/edgexfoundry/go-mod-core-contracts/v4/models"
)

// ExtendedProtocolDriver is a low-level node-specific interface implemented
// by node services that support extended ProtocolDriver features.
type ExtendedProtocolDriver interface {
	// ProfileScan triggers specific node to discover node profile.
	ProfileScan(req requests.ProfileScanRequest) (model.Profile, error)
	// StopNodeDiscovery stops the ongoing node discovery process.
	StopNodeDiscovery(options map[string]any)
	// StopProfileScan stops the ongoing node profile scan process for a specific node.
	StopProfileScan(nodeName string, options map[string]any)
}
