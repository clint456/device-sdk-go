// -*- Mode: Go; indent-tabs-mode: t -*-
//
// # Copyright (C) 2023 Intel Corporation
// # Copyright (C) 2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0
package provision

import (
	"net/url"
	"path"
	"testing"

	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_processNodes(t *testing.T) {
	tests := []struct {
		name                string
		path                string
		update              bool
		secretProvider      interfaces.SecretProvider
		expectedNumNodes    int
		expectedUpdateNodes int
	}{
		{"valid load node from file", path.Join("..", "..", "example", "cmd", "node-simple", "res", "nodes", "simple-node.yml"), false, nil, 2, 0},
		{"valid load nodes from uri", "https://raw.githubusercontent.com/edgexfoundry/node-virtual-go/main/cmd/res/nodes/nodes.yaml", false, nil, 6, 0},
		{"valid overwrite node from file", path.Join("..", "..", "example", "cmd", "node-simple", "res", "nodes", "simple-node.yml"), true, nil, 1, 1},
		{"invalid load node empty path", "", false, nil, 0, 0},
		{"invalid load node from file", path.Join("..", "..", "example", "cmd", "node-simple", "res", "nodes", "bogus.yml"), false, nil, 0, 0},
		{"invalid load node invalid uri", "https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/example/cmd/node-simple/res/nodes/bogus.yml", false, nil, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := logger.MockLogger{}
			dic, _ := NewMockDIC()
			err := cache.InitCache(TestNodeService, TestNodeService, dic)
			if tt.update {
				err := cache.Nodes().Add(models.Node{
					Name: "Simple-Node01",
				})
				require.NoError(t, err)
			}

			require.NoError(t, err)
			addNodeRequests, updateNodeRequests := processNodes(tt.path, tt.path, TestNodeService, tt.update, tt.secretProvider, lc)
			assert.Equal(t, tt.expectedNumNodes, len(addNodeRequests))
			if tt.update {
				assert.Equal(t, tt.expectedUpdateNodes, len(updateNodeRequests))
			}
		})
	}
}

func Test_loadNodesFromURI(t *testing.T) {
	tests := []struct {
		name                string
		path                string
		serviceName         string
		secretProvider      interfaces.SecretProvider
		expectedNumNodes    int
		expectedEdgexErrMsg string
	}{
		{"valid load from uri",
			"https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/internal/provision/uri-test-files/nodes/index.json",
			"TestNode",
			nil,
			2, ""},
		{"invalid load from uri",
			"https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/internal/provision/uri-test-files/nodes/bogus.json",
			"TestNode",
			nil,
			0, "failed to load Nodes list from URI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var addNodeReq []requests.AddNodeRequest
			var updateNodeReq []requests.UpdateNodeRequest
			lc := logger.MockLogger{}
			dic, _ := NewMockDIC()
			edgexErr := cache.InitCache(TestNodeService, TestNodeService, dic)
			require.NoError(t, edgexErr)
			parsedURI, err := url.Parse(tt.path)
			require.NoError(t, err)
			addNodeReq, updateNodeReq, edgexErr = loadNodesFromURI(tt.path, parsedURI, tt.serviceName, false, tt.secretProvider, lc)
			assert.Equal(t, tt.expectedNumNodes, len(addNodeReq))
			assert.Equal(t, 0, len(updateNodeReq))
			if edgexErr != nil {
				assert.Contains(t, edgexErr.Error(), tt.expectedEdgexErrMsg)
			}
		})
	}
}
