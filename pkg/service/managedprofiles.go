// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018-2023 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/google/uuid"

	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
)

// AddProfile adds a new Profile to the Node Service and Core Metadata
// Returns new Profile id or non-nil error.
func (s *nodeService) AddProfile(profile models.Profile) (string, error) {
	if p, ok := cache.Profiles().ForName(profile.Name); ok {
		return p.Id, errors.NewCommonEdgeX(errors.KindDuplicateName, fmt.Sprintf("name conflicted, Profile %s exists", profile.Name), nil)
	}

	// Assign model ids and normalize the model side of the resource/command references.
	profile.EnsureIds()

	s.lc.Debugf("Adding managed Profile %s", profile.Name)
	req := requests.NewProfileRequest(dtos.FromProfileModelToDTO(profile))
	ctx := context.WithValue(context.Background(), common.CorrelationHeader, uuid.NewString()) // nolint:staticcheck
	res, err := container.ProfileClientFrom(s.dic.Get).Add(ctx, []requests.ProfileRequest{req})
	if err != nil {
		s.lc.Errorf("failed to add Profile %s to Core Metadata: %v", profile.Name, err)
		return "", err
	}
	err = cache.Profiles().Add(profile)
	if err != nil {
		s.lc.Errorf("failed to add Profile %s to cache", profile.Name)
		return "", err
	}

	return res[0].Id, nil
}

// Profiles return all managed Profiles from cache
func (s *nodeService) Profiles() []models.Profile {
	return cache.Profiles().All()
}

// GetProfileByName returns the Profile by its name if it exists in the cache, or returns an error.
func (s *nodeService) GetProfileByName(name string) (models.Profile, error) {
	profile, ok := cache.Profiles().ForName(name)
	if !ok {
		msg := fmt.Sprintf("failed to find Profile %s in cache", name)
		s.lc.Error(msg)
		return models.Profile{}, errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, msg, nil)
	}
	return profile, nil
}

// RemoveProfileByName removes the specified Profile by name from the cache and ensures that the
// instance in Core Metadata is also removed.
func (s *nodeService) RemoveProfileByName(name string) error {
	profile, ok := cache.Profiles().ForName(name)
	if !ok {
		msg := fmt.Sprintf("failed to find Profile %s in cache", name)
		s.lc.Error(msg)
		return errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, msg, nil)
	}

	s.lc.Debugf("Removing managed Profile %s", profile.Name)
	ctx := context.WithValue(context.Background(), common.CorrelationHeader, uuid.NewString()) // nolint:staticcheck
	_, err := container.ProfileClientFrom(s.dic.Get).DeleteByName(ctx, name)
	if err != nil {
		s.lc.Errorf("failed to delete Profile %s in Core Metadata", name)
		return err
	}

	err = cache.Profiles().RemoveByName(name)
	return err
}

// UpdateProfile updates the Profile in the cache and ensures that the
// copy in Core Metadata is also updated.
func (s *nodeService) UpdateProfile(profile models.Profile) error {
	_, ok := cache.Profiles().ForName(profile.Name)
	if !ok {
		msg := fmt.Sprintf("failed to find Profile %s in cache", profile.Name)
		s.lc.Error(msg)
		return errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, msg, nil)
	}

	s.lc.Debugf("Updating managed Profile %s", profile.Name)
	// Assign model ids for newly added resources/commands and normalize references.
	profile.EnsureIds()
	req := requests.NewProfileRequest(dtos.FromProfileModelToDTO(profile))
	ctx := context.WithValue(context.Background(), common.CorrelationHeader, uuid.NewString()) // nolint:staticcheck
	_, err := container.ProfileClientFrom(s.dic.Get).Update(ctx, []requests.ProfileRequest{req})
	if err != nil {
		s.lc.Errorf("failed to update Profile %s in Core Metadata: %v", profile.Name, err)
		return err
	}

	return err
}

// Command retrieves the specific Command instance from cache according to
// the Node name and Command name
func (s *nodeService) Command(nodeName string, commandName string) (models.Command, bool) {
	node, ok := cache.Nodes().ForName(nodeName)
	if !ok {
		s.lc.Errorf("failed to find node %s in cache", nodeName)
		return models.Command{}, false
	}

	dc, ok := cache.Profiles().Command(node.ProfileName, commandName)
	if !ok {
		return dc, false
	}
	return dc, true
}

// Resource retrieves the specific Resource instance from cache according to
// the Node name and Node Resource name
func (s *nodeService) Resource(nodeName string, resource string) (models.Resource, bool) {
	node, ok := cache.Nodes().ForName(nodeName)
	if !ok {
		s.lc.Errorf("failed to find node %s in cache", nodeName)
		return models.Resource{}, false
	}

	dr, ok := cache.Profiles().Resource(node.ProfileName, resource)
	if !ok {
		return dr, false
	}
	return dr, true
}
