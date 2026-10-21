// -*- Mode: Go; indent-tabs-mode: t -*-
//
// # Copyright (C) 2023 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0
package provision

import (
	"context"
	goErrors "errors"
	"net/url"
	"path"
	"testing"

	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/interfaces"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/responses"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var simpleProfile = responses.ProfileResponse{
	Profile: dtos.Profile{
		ProfileBasicInfo: dtos.ProfileBasicInfo{
			Name:         "Simple-Node",
			Manufacturer: "Simple Corp.",
			Description:  "Example of Simple Node",
			Model:        "SP-01",
			Labels:       []string{"modbus"},
		},
		Resources: []dtos.Resource{
			{
				Description: "Switch On/Off.",
				Name:        "SwitchButton",
				IsHidden:    false,
				Properties: dtos.ResourceProperties{
					ValueType:    "Bool",
					ReadWrite:    "RW",
					DefaultValue: "true",
				},
			},
			{
				Description: "Visual representation of Switch state.",
				Name:        "Image",
				IsHidden:    false,
				Properties: dtos.ResourceProperties{
					ValueType: "Binary",
					ReadWrite: "R",
					MediaType: "image/jpeg",
				},
			},
			{
				Description: "X axis rotation rate",
				Name:        "Xrotation",
				IsHidden:    true,
				Properties: dtos.ResourceProperties{
					ValueType: "Int32",
					ReadWrite: "RW",
					Units:     "rpm",
				},
			},
			{
				Description: "y axis rotation rate",
				Name:        "yrotation",
				IsHidden:    true,
				Properties: dtos.ResourceProperties{
					ValueType: "Int32",
					ReadWrite: "RW",
					Units:     "rpm",
				},
			},
			{
				Description: "z axis rotation rate",
				Name:        "zrotation",
				IsHidden:    true,
				Properties: dtos.ResourceProperties{
					ValueType: "Int32",
					ReadWrite: "RW",
					Units:     "rpm",
				},
			},
			{
				Description: "String array",
				Name:        "StringArray",
				IsHidden:    false,
				Properties: dtos.ResourceProperties{
					ValueType: "StringArray",
					ReadWrite: "RW",
				},
			},
			{
				Description: "Unsigned 8bit array",
				Name:        "Uint8Array",
				IsHidden:    false,
				Properties: dtos.ResourceProperties{
					ValueType: "Uint8Array",
					ReadWrite: "RW",
				},
			},
			{
				Description: "Counter data",
				Name:        "Counter",
				IsHidden:    false,
				Properties: dtos.ResourceProperties{
					ValueType: "Object",
					ReadWrite: "RW",
				},
			},
		},
		Commands: []dtos.Command{
			{
				Name:      "Switch",
				IsHidden:  false,
				ReadWrite: "RW",
				ResourceOperations: []dtos.ResourceOperation{
					{
						Resource:     "SwitchButton",
						DefaultValue: "false",
					},
				},
			},
			{
				Name:      "Image",
				IsHidden:  false,
				ReadWrite: "R",
				ResourceOperations: []dtos.ResourceOperation{
					{
						Resource: "Image",
					},
				},
			},
			{
				Name:      "Rotation",
				IsHidden:  false,
				ReadWrite: "RW",
				ResourceOperations: []dtos.ResourceOperation{
					{
						Resource:     "XRotation",
						DefaultValue: "0",
					},
					{
						Resource:     "YRotation",
						DefaultValue: "0",
					},
					{
						Resource:     "ZRotation",
						DefaultValue: "0",
					},
				},
			},
		},
	},
}

func Test_processProfiles(t *testing.T) {
	tests := []struct {
		name                string
		path                string
		secretProvider      interfaces.SecretProvider
		profileName         string
		overwrite           bool
		dpcMockRes          responses.ProfileResponse
		dpcMockErr          errors.EdgeX
		expectedNumProfiles int
		expectedEdgexErrMsg string
	}{
		{"valid load profile from file, profile exists", path.Join("..", "..", "example", "cmd", "node-simple", "res", "profiles", "Simple-Driver.yaml"), nil, "Simple-Node", false, simpleProfile, nil, 0, ""},
		{"valid load profile from file, profile exists and want to overwrite", path.Join("..", "..", "example", "cmd", "node-simple", "res", "profiles", "Simple-Driver.yaml"), nil, "Simple-Node", true, simpleProfile, nil, 1, ""},
		{"valid load profile from file, profile does not exist in metadata", path.Join("..", "..", "example", "cmd", "node-simple", "res", "profiles", "Simple-Driver.yaml"), nil, "Simple-Node", false, responses.ProfileResponse{}, errors.NewCommonEdgeXWrapper(goErrors.New("could not find profile")), 1, ""},
		{"valid load profile from uri, profile exists", "https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/example/cmd/node-simple/res/profiles/Simple-Driver.yaml", nil, "Simple-Node", false, simpleProfile, nil, 0, ""},
		{"valid load profile from uri, profile exists and want to overwrite", "https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/example/cmd/node-simple/res/profiles/Simple-Driver.yaml", nil, "Simple-Node", true, simpleProfile, nil, 1, ""},
		{"valid load profile from uri, profile does not exist in metadata", "https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/example/cmd/node-simple/res/profiles/Simple-Driver.yaml", nil, "Simple-Node", false, responses.ProfileResponse{}, errors.NewCommonEdgeXWrapper(goErrors.New("could not find profile")), 1, ""},
		{"invalid load empty profile from file", "", nil, "Simple-Node", false, responses.ProfileResponse{}, errors.NewCommonEdgeXWrapper(goErrors.New("could not find profile")), 0, ""},
		{"invalid load profile from bogus file", path.Join("..", "..", "example", "cmd", "node-simple", "res", "profiles", "bogus.yaml"), nil, "Simple-Node", false, responses.ProfileResponse{}, nil, 0, ""},
		{"invalid load profile from bogus uri", "https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/example/cmd/node-simple/res/profiles/bogus.yaml", nil, "Simple-Node", false, responses.ProfileResponse{}, nil, 0, ""},
		{"invalid load profile from file, duplicate profile", path.Join("..", "..", "example", "cmd", "node-simple", "res", "profiles", "Simple-Driver.yaml"), nil, "Simple-Node", false, profile, nil, 0, "Profile testProfile has already existed in cache"},
		{"invalid load profile from uri, duplicate profile", "https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/example/cmd/node-simple/res/profiles/Simple-Driver.yaml", nil, "Simple-Node", false, profile, nil, 0, "Profile testProfile has already existed in cache"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var addProfilesReq []requests.ProfileRequest
			var updateProfilesReq []requests.ProfileRequest
			var edgexErr errors.EdgeX
			lc := logger.MockLogger{}
			dic, dpcMock := NewMockDIC()
			dpcMock.On("ProfileByName", context.Background(), tt.profileName).Return(tt.dpcMockRes, tt.dpcMockErr)
			err := cache.InitCache(TestNodeService, TestNodeService, dic)
			require.NoError(t, err)
			addProfilesReq, updateProfilesReq, edgexErr = processProfiles(tt.path, tt.path, tt.overwrite, tt.secretProvider, lc, dpcMock)

			if tt.overwrite {
				assert.Equal(t, tt.expectedNumProfiles, len(updateProfilesReq))
			} else {
				assert.Equal(t, tt.expectedNumProfiles, len(addProfilesReq))
			}

			if edgexErr != nil {
				assert.Contains(t, edgexErr.Error(), tt.expectedEdgexErrMsg)
			}
		})
	}
}

func Test_loadProfilesFromURI(t *testing.T) {
	tests := []struct {
		name                string
		path                string
		secretProvider      interfaces.SecretProvider
		profileNames        []string
		overwrite           bool
		dpcMockRes          []responses.ProfileResponse
		dpcMockErr          []errors.EdgeX
		expectedNumProfiles int
		expectedEdgexErrMsg string
	}{
		{"valid load from uri, profile exists",
			"https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/internal/provision/uri-test-files/profiles/index.json",
			nil,
			[]string{"Simple-Node", "Simple-Node2"},
			false,
			[]responses.ProfileResponse{simpleProfile, simpleProfile},
			[]errors.EdgeX{nil, nil},
			0, ""},
		{"valid load from uri, profile exists and want to overwrite",
			"https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/internal/provision/uri-test-files/profiles/index.json",
			nil,
			[]string{"Simple-Node", "Simple-Node2"},
			true,
			[]responses.ProfileResponse{simpleProfile, simpleProfile},
			[]errors.EdgeX{nil, nil},
			1, ""},
		{"valid load from uri, profile does not exist in metadata",
			"https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/internal/provision/uri-test-files/profiles/index.json",
			nil,
			[]string{"Simple-Node", "Simple-Node2"},
			false,
			[]responses.ProfileResponse{responses.ProfileResponse{}, responses.ProfileResponse{}},
			[]errors.EdgeX{errors.NewCommonEdgeXWrapper(goErrors.New("could not find profile")), errors.NewCommonEdgeXWrapper(goErrors.New("could not find profile"))},
			2, ""},
		{"valid load where one profile exists and one profile does not exist in metadata",
			"https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/internal/provision/uri-test-files/profiles/index.json",
			nil,
			[]string{"Simple-Node", "Simple-Node2"},
			false,
			[]responses.ProfileResponse{simpleProfile, responses.ProfileResponse{}},
			[]errors.EdgeX{nil, errors.NewCommonEdgeXWrapper(goErrors.New("could not find profile"))},
			1, ""},
		{"invalid load profile path join breaks",
			"https://raw.githubusercontent.com/edgexfoundry/south-sdk-go/main/internal/provision/uri-test-files/profiles/bogus.json",
			nil,
			[]string{},
			false,
			[]responses.ProfileResponse{},
			[]errors.EdgeX{},
			0, "failed to load Node Profile list from URI"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var addProfilesReq []requests.ProfileRequest
			var updateProfilesReq []requests.ProfileRequest
			var edgexErr errors.EdgeX
			lc := logger.MockLogger{}
			dic, dpcMock := NewMockDIC()
			for index := range tt.profileNames {
				dpcMock.On("ProfileByName", context.Background(), tt.profileNames[index]).Return(tt.dpcMockRes[index], tt.dpcMockErr[index])
			}
			edgexErr = cache.InitCache(TestNodeService, TestNodeService, dic)
			require.NoError(t, edgexErr)
			parsedURI, err := url.Parse(tt.path)
			require.NoError(t, err)
			addProfilesReq, updateProfilesReq, edgexErr = loadProfilesFromURI(tt.path, parsedURI, tt.overwrite, dpcMock, tt.secretProvider, lc)
			if tt.overwrite {
				assert.Equal(t, len(updateProfilesReq), tt.expectedNumProfiles)
			} else {
				assert.Equal(t, len(addProfilesReq), tt.expectedNumProfiles)
			}
			if edgexErr != nil {
				assert.Contains(t, edgexErr.Error(), tt.expectedEdgexErrMsg)
			}
		})
	}
}
