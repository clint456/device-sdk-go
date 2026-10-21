// -*- Mode: Go; indent-tabs-mode: t -*-
//
// # Copyright (C) 2023 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package provision

import (
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/url"
	"path"
	"testing"
)

func Test_GetFileType(t *testing.T) {
	tests := []struct {
		name             string
		path             string
		expectedFileType FileType
	}{
		{"valid get Yaml file type", path.Join("..", "..", "example", "cmd", "node-simple", "res", "nodes", "simple-node.yml"), YAML},
		{"valid get Json file type", path.Join("..", "..", "example", "cmd", "node-simple", "res", "nodes", "simple-node.json"), JSON},
		{"valid get other file type", path.Join("..", "..", "example", "cmd", "node-simple", "res", "nodes", "simple-node.bogus"), OTHER},
		{"valid EdgeX Username/Password URI get Yaml file type", "http://edgexuser:edgexpasswd@httpd-auth:80/http_files/simple-node.yaml", YAML},
		{"valid EdgeX Username/Password URI get Json file type", "http://edgexuser:edgexpasswd@httpd-auth:80/http_files/simple-node.json", JSON},
		{"valid EdgeX Username/Password URI get other file type", "http://edgexuser:edgexpasswd@httpd-auth:80/http_files/simple-node.bogus", OTHER},
		{"valid EdgeX Secret URI get Yaml file type", "http://httpd-auth:80/http_files/simple-node.yaml?edgexSecretName=httpserver", YAML},
		{"valid EdgeX Secret URI get Json file type", "http://httpd-auth:80/http_files/simple-node.json?edgexSecretName=httpserver", JSON},
		{"valid EdgeX Secret URI get other file type", "http://httpd-auth:80/http_files/simple-node.bogus?edgexSecretName=httpserver", OTHER},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualType := GetFileType(tt.path)
			assert.Equal(t, tt.expectedFileType, actualType)
		})
	}
}

func Test_GetFullAndRedactedURI(t *testing.T) {
	tests := []struct {
		name             string
		baseURI          string
		file             string
		expectedURI      string
		expectedRedacted string
	}{
		{"valid no secret uri", "https://raw.githubusercontent.com/edgexfoundry/node-virtual-go/main/cmd/res/nodes/nodes.yaml", "node-simple.yaml", "https://raw.githubusercontent.com/edgexfoundry/node-virtual-go/main/cmd/res/nodes/node-simple.yaml", "https://raw.githubusercontent.com/edgexfoundry/node-virtual-go/main/cmd/res/nodes/node-simple.yaml"},
		{"valid query secret uri", "https://raw.githubusercontent.com/edgexfoundry/node-simple/main/nodes/index.json?edgexSecretName=githubCredentials", "node-simple.yaml", "https://raw.githubusercontent.com/edgexfoundry/node-simple/main/nodes/node-simple.yaml?edgexSecretName=githubCredentials", "https://raw.githubusercontent.com/edgexfoundry/node-simple/main/nodes/node-simple.yaml?edgexSecretName=githubCredentials"},
		{"valid query secret uri", "https://myuser:mypassword@raw.githubusercontent.com/edgexfoundry/node-simple/main/nodes/index.json", "node-simple.yaml", "https://myuser:mypassword@raw.githubusercontent.com/edgexfoundry/node-simple/main/nodes/node-simple.yaml", "https://myuser:xxxxx@raw.githubusercontent.com/edgexfoundry/node-simple/main/nodes/node-simple.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testURI, err := url.Parse(tt.baseURI)
			require.NoError(t, err)
			lc := logger.MockLogger{}
			actualURI, actualRedacted := GetFullAndRedactedURI(testURI, tt.file, "test", lc)
			assert.Equal(t, tt.expectedURI, actualURI)
			assert.Equal(t, tt.expectedRedacted, actualRedacted)
		})
	}
}
