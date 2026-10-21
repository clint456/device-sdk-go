// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2017-2018 Canonical Ltd
// Copyright (C) 2018-2023 IOTech Ltd
// Copyright (C) 2023 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package provision

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/file"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/interfaces"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"gopkg.in/yaml.v3"

	"github.com/edgexfoundry/south-sdk-go/v4/internal/cache"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/common"
	"github.com/edgexfoundry/south-sdk-go/v4/internal/container"
)

func LoadNodes(path string, overwrite bool, dic *di.Container) errors.EdgeX {
	var addNodesReq []requests.AddNodeRequest
	var updateNodesReq []requests.UpdateNodeRequest
	var edgexErr errors.EdgeX
	if path == "" {
		return nil
	}

	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	serviceName := container.NodeServiceFrom(dic.Get).Name
	parsedUrl, err := url.Parse(path)
	if err != nil {
		return errors.NewCommonEdgeX(errors.KindServerError, "failed to parse Nodes path as a URI", err)
	}
	if parsedUrl.Scheme == "http" || parsedUrl.Scheme == "https" {
		secretProvider := bootstrapContainer.SecretProviderFrom(dic.Get)
		addNodesReq, updateNodesReq, edgexErr = loadNodesFromURI(path, parsedUrl, serviceName, overwrite, secretProvider, lc)
		if edgexErr != nil {
			return edgexErr
		}
	} else {
		addNodesReq, updateNodesReq, edgexErr = loadNodesFromFile(path, serviceName, overwrite, lc)
		if edgexErr != nil {
			return edgexErr
		}
	}

	if len(addNodesReq) == 0 && len(updateNodesReq) == 0 {
		return nil
	}
	dc := bootstrapContainer.NodeClientFrom(dic.Get)
	ctx := context.WithValue(context.Background(), common.CorrelationHeaderKey, uuid.NewString())
	responses, edgexErr := dc.Add(ctx, addNodesReq)
	if edgexErr != nil {
		return edgexErr
	}

	err = nil
	for _, response := range responses {
		if response.StatusCode != http.StatusCreated {
			if response.StatusCode == http.StatusConflict {
				lc.Warnf("%s. Node may be owned by other Node service instance.", response.Message)
				continue
			}

			err = multierror.Append(err, fmt.Errorf("add Node failed: %s", response.Message))
		}
	}

	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}

	updateResponses, edgexErr := dc.Update(ctx, updateNodesReq)
	if edgexErr != nil {
		return edgexErr
	}

	err = nil
	for _, response := range updateResponses {
		if response.StatusCode != http.StatusOK {
			if response.StatusCode == http.StatusConflict {
				lc.Warnf("%s. Node may be owned by other Node service instance.", response.Message)
				continue
			}

			err = multierror.Append(err, fmt.Errorf("update Node failed: %s", response.Message))
		}
	}

	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}

	return nil
}

func loadNodesFromFile(path, serviceName string, overwrite bool, lc logger.LoggingClient) ([]requests.AddNodeRequest, []requests.UpdateNodeRequest, errors.EdgeX) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, errors.NewCommonEdgeX(errors.KindServerError, "failed to create absolute path for Nodes", err)
	}

	files, err := os.ReadDir(absPath)
	if err != nil {
		return nil, nil, errors.NewCommonEdgeX(errors.KindServerError, "failed to read directory for Nodes", err)
	}

	if len(files) == 0 {
		return nil, nil, nil
	}

	lc.Infof("Loading pre-defined Nodes from %s(%d files found)", absPath, len(files))
	var addNodesReq, processedNodesReq []requests.AddNodeRequest
	var updateNodesReq, processedUpdateNodesReq []requests.UpdateNodeRequest
	for _, file := range files {
		fullPath := filepath.Join(absPath, file.Name())
		processedNodesReq, processedUpdateNodesReq = processNodes(fullPath, fullPath, serviceName, overwrite, nil, lc)
		if len(processedNodesReq) > 0 {
			addNodesReq = append(addNodesReq, processedNodesReq...)
		}
		if len(processedUpdateNodesReq) > 0 {
			updateNodesReq = append(updateNodesReq, processedUpdateNodesReq...)
		}
	}
	return addNodesReq, updateNodesReq, nil
}

func loadNodesFromURI(inputURI string, parsedURI *url.URL, serviceName string, overwrite bool, secretProvider interfaces.SecretProvider, lc logger.LoggingClient) ([]requests.AddNodeRequest, []requests.UpdateNodeRequest, errors.EdgeX) {
	// the input URI contains the index file containing the Node list to be loaded
	bytes, err := file.Load(inputURI, secretProvider, lc)
	if err != nil {
		return nil, nil, errors.NewCommonEdgeX(errors.KindServerError, fmt.Sprintf("failed to load Nodes list from URI %s", parsedURI.Redacted()), err)
	}

	var files []string
	err = json.Unmarshal(bytes, &files)
	if err != nil {
		return nil, nil, errors.NewCommonEdgeX(errors.KindServerError, "could not unmarshal Nodes list contents", err)
	}

	if len(files) == 0 {
		lc.Infof("Index file %s for Nodes list is empty", parsedURI.Redacted())
		return nil, nil, nil
	}

	lc.Infof("Loading pre-defined nodes from %s(%d files found)", parsedURI.Redacted(), len(files))
	var addNodesReq, processedNodesReq []requests.AddNodeRequest
	var updateNodesReq, processedUpdateNodesReq []requests.UpdateNodeRequest
	for _, file := range files {
		fullPath, redactedPath := GetFullAndRedactedURI(parsedURI, file, "Node", lc)
		processedNodesReq, processedUpdateNodesReq = processNodes(fullPath, redactedPath, serviceName, overwrite, secretProvider, lc)
		if len(processedNodesReq) > 0 {
			addNodesReq = append(addNodesReq, processedNodesReq...)
		}
		if len(processedUpdateNodesReq) > 0 {
			updateNodesReq = append(updateNodesReq, processedUpdateNodesReq...)
		}
	}
	return addNodesReq, updateNodesReq, nil
}

func processNodes(fullPath, displayPath, serviceName string, overwrite bool, secretProvider interfaces.SecretProvider, lc logger.LoggingClient) ([]requests.AddNodeRequest, []requests.UpdateNodeRequest) {
	var nodes []dtos.Node
	var addNodesReq []requests.AddNodeRequest
	var updateNodesReq []requests.UpdateNodeRequest

	fileType := GetFileType(fullPath)

	// if the file type is not yaml or json, it cannot be parsed - just return to not break the loop for other nodes
	if fileType == OTHER {
		return nil, nil
	}

	content, err := file.Load(fullPath, secretProvider, lc)
	if err != nil {
		lc.Errorf("Failed to read Nodes from %s: %v", displayPath, err)
		return nil, nil
	}

	switch fileType {
	case YAML:
		d := struct {
			NodeList []dtos.Node `yaml:"nodeList"`
		}{}
		err = yaml.Unmarshal(content, &d)
		if err != nil {
			lc.Errorf("Failed to YAML decode Nodes from %s: %v", displayPath, err)
			return nil, nil
		}
		nodes = d.NodeList
	case JSON:
		err = json.Unmarshal(content, &nodes)
		if err != nil {
			lc.Errorf("Failed to JSON decode Nodes from %s: %v", displayPath, err)
			return nil, nil
		}
	}

	for _, node := range nodes {
		if cachedDev, ok := cache.Nodes().ForName(node.Name); ok {
			if overwrite {
				lc.Infof("Overwriting existing node %s with one in local files", node.Name)

				node.ServiceName = cachedDev.ServiceName
				node.AdminState = string(cachedDev.AdminState)
				node.OperatingState = string(cachedDev.OperatingState)
				node.Direction = models.South

				update := dtos.FromNodeModelToUpdateDTO(dtos.ToNodeModel(node))
				req := requests.NewUpdateNodeRequest(update)
				updateNodesReq = append(updateNodesReq, req)
			} else {
				lc.Infof("Node %s exists, using the existing one", node.Name)
			}
		} else {
			lc.Infof("Node %s not found in Metadata, adding it ...", node.Name)
			node.ServiceName = serviceName
			node.AdminState = models.Unlocked
			node.OperatingState = models.Up
			node.Direction = models.South
			req := requests.NewAddNodeRequest(node)
			addNodesReq = append(addNodesReq, req)
		}
	}
	return addNodesReq, updateNodesReq
}
