// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018-2026 IOTech Ltd
// Copyright (C) 2023 Intel
//
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	commonDTO "github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
	internalCommon "github.com/edgexfoundry/south-sdk-go/v4/internal/common"

	"github.com/google/uuid"
)

// AddNode adds a new Node to the Node Service and Core Metadata
// Returns new Node id or non-nil error.
func (s *nodeService) AddNode(node models.Node) (string, error) {
	return s.addNode(node, false)
}

// AddNodeWithoutValidation adds a new Node to the Node Service and Core Metadata
// with bypassValidation=true to skip node validation.
// Returns new Node id or non-nil error.
func (s *nodeService) AddNodeWithoutValidation(node models.Node) (string, error) {
	return s.addNode(node, true)
}

func (s *nodeService) addNode(node models.Node, bypassValidation bool) (string, error) {
	if d, ok := cache.Nodes().ForName(node.Name); ok {
		return d.Id, errors.NewCommonEdgeX(errors.KindDuplicateName, fmt.Sprintf("name conflicted, Node %s exists", node.Name), nil)
	}

	node.ServiceName = s.serviceKey
	if node.Type == "" {
		node.Type = models.NodeTypeDevice
	}

	if bypassValidation {
		s.lc.Debugf("Adding managed Node %s without validation", node.Name)
	} else {
		s.lc.Debugf("Adding managed Node %s", node.Name)
	}
	req := requests.NewAddNodeRequest(dtos.FromNodeModelToDTO(node))
	ctx := context.WithValue(context.Background(), common.CorrelationHeader, uuid.NewString()) // nolint:staticcheck
	var (
		res []commonDTO.BaseWithIdResponse
		err error
	)
	if bypassValidation {
		res, err = container.NodeClientFrom(s.dic.Get).AddWithQueryParams(ctx, []requests.AddNodeRequest{req},
			map[string]string{internalCommon.BypassValidationQueryParam: common.ValueTrue})
	} else {
		res, err = container.NodeClientFrom(s.dic.Get).Add(ctx, []requests.AddNodeRequest{req})
	}
	if err != nil {
		s.lc.Errorf("failed to add Node %s to Core Metadata: %v", node.Name, err)
		return "", err
	}

	return res[0].Id, nil
}

// Nodes return all managed Nodes from cache
func (s *nodeService) Nodes() []models.Node {
	return cache.Nodes().All()
}

// GetNodeByName returns the Node by its name if it exists in the cache, or returns an error.
func (s *nodeService) GetNodeByName(name string) (models.Node, error) {
	node, ok := cache.Nodes().ForName(name)
	if !ok {
		msg := fmt.Sprintf("failed to find Node %s in cache", name)
		s.lc.Error(msg)
		return models.Node{}, errors.NewCommonEdgeX(errors.KindEntityDoesNotExist, msg, nil)
	}
	return node, nil
}

// NodeExistsForName returns true if a node exists in cache with the specified name, otherwise it returns false.
func (s *nodeService) NodeExistsForName(name string) bool {
	_, ok := cache.Nodes().ForName(name)
	return ok
}

// RemoveNodeByName removes the specified Node by name from the cache and ensures that the
// instance in Core Metadata is also removed.
func (s *nodeService) RemoveNodeByName(name string) error {
	if _, err := s.GetNodeByName(name); err != nil {
		return err
	}

	s.lc.Debugf("Removing managed Node %s", name)
	ctx := context.WithValue(context.Background(), common.CorrelationHeader, uuid.NewString()) // nolint:staticcheck
	_, err := container.NodeClientFrom(s.dic.Get).DeleteNodeByName(ctx, name)
	if err != nil {
		s.lc.Errorf("failed to delete Node %s in Core Metadata", name)
	}

	return err
}

// UpdateNode updates the Node in Core Metadata
func (s *nodeService) UpdateNode(node models.Node) error {
	return s.PatchNode(dtos.FromNodeModelToUpdateDTO(node))
}

// UpdateNodeWithoutValidation updates the Node in Core Metadata with bypassValidation=true
// to skip node validation.
func (s *nodeService) UpdateNodeWithoutValidation(node models.Node) error {
	return s.PatchNodeWithoutValidation(dtos.FromNodeModelToUpdateDTO(node))
}

// UpdateNodeOperatingState updates the OperatingState for the Node with given name
// in Core Metadata
func (s *nodeService) UpdateNodeOperatingState(name string, state models.OperatingState) error {
	stateString := string(state)
	return s.PatchNodeWithoutValidation(dtos.UpdateNode{
		Name:           &name,
		OperatingState: &stateString,
	})
}

// PatchNode patches the specified node properties in Core Metadata. Node name is required
// to be provided in the UpdateNode. Note that all properties of UpdateNode are pointers
// and anything that is nil will not modify the node. In the case of Arrays and Maps, the whole new value
// must be sent, as it is applied as an overwrite operation.
func (s *nodeService) PatchNode(updateNode dtos.UpdateNode) error {
	return s.patchNode(updateNode, false)
}

// PatchNodeWithoutValidation patches the specified node properties in Core Metadata with bypassValidation=true
// to skip node validation. Node name is required to be provided in the UpdateNode. Note that all properties
// of UpdateNode are pointers and anything that is nil will not modify the node. In the case of Arrays and Maps,
// the whole new value must be sent, as it is applied as an overwrite operation.
func (s *nodeService) PatchNodeWithoutValidation(updateNode dtos.UpdateNode) error {
	return s.patchNode(updateNode, true)
}

func (s *nodeService) patchNode(updateNode dtos.UpdateNode, bypassValidation bool) error {
	if updateNode.Name == nil {
		msg := "missing node name for patch node call"
		s.lc.Error(msg)
		return errors.NewCommonEdgeX(errors.KindContractInvalid, msg, nil)
	}

	if _, err := s.GetNodeByName(*updateNode.Name); err != nil {
		return err
	}

	if bypassValidation {
		s.lc.Debugf("Patching managed Node %s without validation", *updateNode.Name)
	} else {
		s.lc.Debugf("Patching managed Node %s", *updateNode.Name)
	}
	req := requests.UpdateNodeRequest{
		BaseRequest: commonDTO.NewBaseRequest(),
		Node:        updateNode,
	}
	ctx := context.WithValue(context.Background(), common.CorrelationHeader, uuid.NewString()) // nolint:staticcheck
	var err error
	if bypassValidation {
		_, err = container.NodeClientFrom(s.dic.Get).UpdateWithQueryParams(ctx, []requests.UpdateNodeRequest{req},
			map[string]string{internalCommon.BypassValidationQueryParam: common.ValueTrue})
	} else {
		_, err = container.NodeClientFrom(s.dic.Get).Update(ctx, []requests.UpdateNodeRequest{req})
	}
	if err != nil {
		s.lc.Errorf("failed to update Node %s in Core Metadata: %v", *updateNode.Name, err)
	}

	return err
}
