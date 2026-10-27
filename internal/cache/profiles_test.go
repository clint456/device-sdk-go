//
// Copyright (C) 2021-2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"regexp"
	"testing"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testProfile = models.Profile{
	ProfileBasicInfo: models.ProfileBasicInfo{Name: TestProfile},
	Resources: []models.Resource{
		models.Resource{Name: TestResource},
		models.Resource{Name: TestResource + "suffix"},
	},
	Commands: []models.Command{
		models.Command{
			Name:      TestCommand,
			ReadWrite: common.ReadWrite_R,
			ResourceOperations: []models.ResourceOperation{
				models.ResourceOperation{Resource: TestResource},
			},
		},
	},
}

var newProfile = models.Profile{
	ProfileBasicInfo: models.ProfileBasicInfo{Name: "newProfile"},
	Resources: []models.Resource{
		models.Resource{Name: "newResource"},
	},
	Commands: []models.Command{
		models.Command{
			Name:      "newCommand",
			ReadWrite: common.ReadWrite_R,
			ResourceOperations: []models.ResourceOperation{
				models.ResourceOperation{Resource: "newResource"},
			},
		},
	},
}

func Test_profileCache_ForName(t *testing.T) {
	newProfileCache([]models.Profile{testProfile})

	tests := []struct {
		name        string
		profileName string
		profile     models.Profile
		expected    bool
	}{
		{"Invalid - empty name", "", models.Profile{}, false},
		{"Invalid - nonexistent Profile name", "nil", models.Profile{}, false},
		{"Valid", TestProfile, testProfile, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, ok := pc.ForName(tt.profileName)
			assert.Equal(t, res, tt.profile, "ForName returns wrong Profile")
			assert.Equal(t, ok, tt.expected, "ForName returns opposite result")
		})
	}
}

func Test_profileCache_All(t *testing.T) {
	newProfileCache([]models.Profile{testProfile})

	res := pc.All()
	assert.Equal(t, len(res), len(pc.profileMap))
}

func Test_profileCache_Add(t *testing.T) {
	newProfileCache([]models.Profile{testProfile})

	tests := []struct {
		name          string
		expectedError bool
	}{
		{"Valid", false},
		{"Invalid - duplicate Profile", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pc.Add(newProfile)
			if tt.expectedError {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_profileCache_RemoveByName(t *testing.T) {
	newProfileCache([]models.Profile{testProfile})

	tests := []struct {
		name          string
		expectedError bool
	}{
		{"Valid", false},
		{"Invalid - nonexistent Profile name", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pc.RemoveByName(TestProfile)
			if tt.expectedError {
				assert.NotNil(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_profileCache_Resource(t *testing.T) {
	newProfileCache([]models.Profile{testProfile})

	tests := []struct {
		name         string
		profileName  string
		resourceName string
		resource     models.Resource
		expected     bool
	}{
		{"Invalid - nonexistent Profile name", "nil", TestResource, models.Resource{}, false},
		{"Invalid - nonexistent Resource name", TestProfile, "nil", models.Resource{}, false},
		{"Valid", TestProfile, TestResource, testProfile.Resources[0], true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, ok := pc.Resource(tt.profileName, tt.resourceName)
			assert.Equal(t, res, tt.resource, "Resource returns wrong resource")
			assert.Equal(t, ok, tt.expected, "Resource returns opposite result")
		})
	}
}

func Test_profileCache_Command(t *testing.T) {
	newProfileCache([]models.Profile{testProfile})

	tests := []struct {
		name        string
		profileName string
		commandName string
		command     models.Command
		expected    bool
	}{
		{"Invalid - nonexistent Profile name", "nil", TestCommand, models.Command{}, false},
		{"Invalid - nonexistent Command name", TestProfile, "nil", models.Command{}, false},
		{"Valid", TestProfile, TestCommand, testProfile.Commands[0], true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, ok := pc.Command(tt.profileName, tt.commandName)
			assert.Equal(t, res, tt.command, "Resource returns wrong resource")
			assert.Equal(t, ok, tt.expected, "Resource returns opposite result")
		})
	}
}

func Test_profileCache_ResourceOperation(t *testing.T) {
	newProfileCache([]models.Profile{testProfile})

	tests := []struct {
		name          string
		profile       string
		resource      string
		res           models.ResourceOperation
		expectedError bool
	}{
		{"Invalid - nonexistent Profile name", "nil", TestResource, models.ResourceOperation{}, true},
		{"Invalid - nonexistent Resource name", TestProfile, "nil", models.ResourceOperation{}, true},
		{"Valid", TestProfile, TestResource, testProfile.Commands[0].ResourceOperations[0], false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ro, err := pc.ResourceOperation(tt.profile, tt.resource)
			if tt.expectedError {
				assert.NotNil(t, err)
				assert.Equal(t, ro, tt.res)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, ro, tt.res)
			}
		})
	}
}

func Test_profileCache_ResourcesByRegex(t *testing.T) {
	newProfileCache([]models.Profile{testProfile})

	tests := []struct {
		name              string
		profileName       string
		regexResourceName string
		matchedResources  int
		expected          bool
	}{
		{"valid - resources found by resource name", TestProfile, TestResource, 1, true},
		{"valid - resources found by regex pattern", TestProfile, "testResource.*", 2, true},
		{"Valid -  resources not found by regex pattern", TestProfile, "^resource.*", 0, true},
		{"invalid - profile not found", "nil", "%2E%2B-resource", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regex, err := regexp.CompilePOSIX(tt.regexResourceName)
			require.NoError(t, err)

			res, ok := pc.ResourcesByRegex(tt.profileName, regex)
			assert.Equal(t, len(res), tt.matchedResources, "ResourcesByRegex returns wrong resource")
			assert.Equal(t, ok, tt.expected, "ResourcesByRegex returns opposite result")
		})
	}
}
