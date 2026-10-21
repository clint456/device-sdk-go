// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2020-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
)

var (
	pc *profileCache
)

type ProfileCache interface {
	ForName(name string) (models.Profile, bool)
	All() []models.Profile
	Add(profile models.Profile) errors.EdgeX
	Update(profile models.Profile) errors.EdgeX
	RemoveByName(name string) errors.EdgeX
	Resource(profileName string, resourceName string) (models.Resource, bool)
	ResourcesByRegex(profileName string, regex *regexp.Regexp) ([]models.Resource, bool)
	Command(profileName string, commandName string) (models.Command, bool)
	ResourceOperation(profileName string, resource string) (models.ResourceOperation, errors.EdgeX)
	CheckAndAdd(profile models.Profile) errors.EdgeX
}

type profileCache struct {
	profileMap  map[string]*models.Profile // key is Profile name
	resourceMap map[string]map[string]models.Resource
	commandMap  map[string]map[string]models.Command
	mutex       sync.RWMutex
}

func newProfileCache(profiles []models.Profile) ProfileCache {
	defaultSize := len(profiles)
	dpMap := make(map[string]*models.Profile, defaultSize)
	drMap := make(map[string]map[string]models.Resource, defaultSize)
	dcMap := make(map[string]map[string]models.Command, defaultSize)
	for i, dp := range profiles {
		dpMap[dp.Name] = &profiles[i]
		drMap[dp.Name] = resourceSliceToMap(dp.Resources)
		dcMap[dp.Name] = commandSliceToMap(dp.Commands)
	}

	pc = &profileCache{
		profileMap:  dpMap,
		resourceMap: drMap,
		commandMap:  dcMap,
	}
	return pc
}

// ForName returns a profile with the given profile name.
func (p *profileCache) ForName(name string) (models.Profile, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	profile, ok := p.profileMap[name]
	if !ok {
		return models.Profile{}, false
	}
	// As the cache profile contains pointer fields(map and slice), directly return profile may cause concurrent map read
	// or write if the invoker spawn another goroutine to manipulate the profile, so returning the clone of profile here
	return profile.Clone(), ok
}

// All returns the current list of profiles in the cache.
func (p *profileCache) All() []models.Profile {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	i := 0
	ps := make([]models.Profile, len(p.profileMap))
	for _, profile := range p.profileMap {
		ps[i] = profile.Clone()
		i += 1
	}
	return ps
}

// Add adds a new profile to the cache. This method is used to populate the
// profile cache with pre-existing or recently-added profiles from Core Metadata.
func (p *profileCache) Add(profile models.Profile) errors.EdgeX {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.add(profile)
}

func (p *profileCache) CheckAndAdd(profile models.Profile) errors.EdgeX {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, ok := p.profileMap[profile.Name]; ok {
		return nil
	} else {
		return p.add(profile)
	}
}

func (p *profileCache) add(profile models.Profile) errors.EdgeX {
	if _, ok := p.profileMap[profile.Name]; ok {
		errMsg := fmt.Sprintf("Profile %s has already existed in cache", profile.Name)
		return errors.NewCommonEdgeX(errors.KindDuplicateName, errMsg, nil)
	}

	p.profileMap[profile.Name] = &profile
	p.resourceMap[profile.Name] = resourceSliceToMap(profile.Resources)
	p.commandMap[profile.Name] = commandSliceToMap(profile.Commands)
	return nil
}

func resourceSliceToMap(resources []models.Resource) map[string]models.Resource {
	result := make(map[string]models.Resource, len(resources))
	for _, dr := range resources {
		result[dr.Name] = dr
	}

	return result
}

func commandSliceToMap(commands []models.Command) map[string]models.Command {
	result := make(map[string]models.Command, len(commands))
	for _, dc := range commands {
		result[dc.Name] = dc
	}

	return result
}

// Update updates the profile in the cache
func (p *profileCache) Update(profile models.Profile) errors.EdgeX {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if err := p.removeByName(profile.Name); err != nil {
		return err
	}

	return p.add(profile)
}

// RemoveByName removes the specified profile by name from the cache.
func (p *profileCache) RemoveByName(name string) errors.EdgeX {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.removeByName(name)
}

func (p *profileCache) removeByName(name string) errors.EdgeX {
	_, ok := p.profileMap[name]
	if !ok {
		errMsg := fmt.Sprintf("failed to find Profile %s in cache", name)
		return errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
	}

	delete(p.profileMap, name)
	delete(p.resourceMap, name)
	delete(p.commandMap, name)
	return nil
}

// Resource returns the Resource with given profileName and resourceName
func (p *profileCache) Resource(profileName string, resourceName string) (models.Resource, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	drs, ok := p.resourceMap[profileName]
	if !ok {
		return models.Resource{}, false
	}

	dr, ok := drs[resourceName]
	return dr, ok
}

// ResourcesByRegex returns matched Resource list with given profileName and regex pattern
func (p *profileCache) ResourcesByRegex(profileName string, regex *regexp.Regexp) ([]models.Resource, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	drs, ok := p.resourceMap[profileName]
	if !ok {
		return nil, false
	}

	var res []models.Resource
	for _, dr := range drs {
		if dr.Name == regex.String() {
			res = append(res, dr)
			continue
		}

		matchString := regex.FindString(dr.Name)
		if matchString == dr.Name {
			res = append(res, dr)
		}
	}

	return res, true
}

// Command returns the Command with given profileName and commandName
func (p *profileCache) Command(profileName string, commandName string) (models.Command, bool) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	dcs, ok := p.commandMap[profileName]
	if !ok {
		return models.Command{}, false
	}

	dc, ok := dcs[commandName]
	return dc, ok
}

// ResourceOperation returns the first matched ResourceOperation with given resourceName
func (p *profileCache) ResourceOperation(profileName string, resourceName string) (models.ResourceOperation, errors.EdgeX) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if err := p.verifyProfileExists(profileName); err != nil {
		return models.ResourceOperation{}, err
	}

	commandMap := p.commandMap[profileName]
	for _, dc := range commandMap {
		for _, ro := range dc.ResourceOperations {
			if ro.Resource == resourceName {
				return ro, nil
			}
		}
	}

	errMsg := fmt.Sprintf("failed to find ResourceOpertaion with Resource %s in Profile %s", resourceName, profileName)
	return models.ResourceOperation{}, errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
}

func (p *profileCache) verifyProfileExists(profileName string) errors.EdgeX {
	if _, ok := p.profileMap[profileName]; !ok {
		errMsg := fmt.Sprintf("failed to find Profile %s in cache", profileName)
		return errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, errMsg, nil)
	}
	return nil
}

func Profiles() ProfileCache {
	return pc
}
