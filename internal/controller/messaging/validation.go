//
// Copyright (C) 2023-2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"context"

	bootstrapContainer "github.com/edgexfoundry/go-mod-bootstrap/v4/bootstrap/container"
	"github.com/edgexfoundry/go-mod-bootstrap/v4/di"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/dtos/requests"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/errors"
	"github.com/edgexfoundry/go-mod-messaging/v4/pkg/types"

	"github.com/edgexfoundry/south-sdk-go/v4/internal/container"
)

func SubscribeNodeValidation(ctx context.Context, dic *di.Container) errors.EdgeX {
	lc := bootstrapContainer.LoggingClientFrom(dic.Get)
	configuration := container.ConfigurationFrom(dic.Get)
	messageBusInfo := container.ConfigurationFrom(dic.Get).MessageBus
	serviceName := container.NodeServiceFrom(dic.Get).Name

	requestTopic := common.NewPathBuilder().EnableNameFieldEscape(configuration.Service.EnableNameFieldEscape).
		SetPath(messageBusInfo.GetBaseTopicPrefix()).SetNameFieldPath(serviceName).SetPath(common.ValidateNodeSubscribeTopic).BuildPath()
	lc.Infof("Subscribing to node validation requests on topic: %s", requestTopic)

	responseTopicPrefix := common.NewPathBuilder().EnableNameFieldEscape(configuration.Service.EnableNameFieldEscape).
		SetPath(messageBusInfo.GetBaseTopicPrefix()).SetPath(common.ResponseTopic).SetNameFieldPath(serviceName).BuildPath()
	lc.Infof("Responses to node validation requests will be published on topic: %s/<requestId>", responseTopicPrefix)

	messages := make(chan types.MessageEnvelope, 1)
	messageErrors := make(chan error, 1)
	topics := []types.TopicChannel{
		{
			Topic:    requestTopic,
			Messages: messages,
		},
	}

	messageBus := bootstrapContainer.MessagingClientFrom(dic.Get)
	err := messageBus.Subscribe(topics, messageErrors)
	if err != nil {
		return errors.NewCommonEdgeXWrapper(err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				lc.Infof("Exiting waiting for MessageBus '%s' topic messages", requestTopic)
				return
			case err = <-messageErrors:
				lc.Error(err.Error())
			case msgEnvelope := <-messages:
				lc.Debugf("Node validation request received on message queue. Topic: %s, Correlation-id: %s", msgEnvelope.ReceivedTopic, msgEnvelope.CorrelationID)

				responseTopic := common.BuildTopic(responseTopicPrefix, msgEnvelope.RequestID)

				driver := container.ProtocolDriverFrom(dic.Get)

				var nodeRequest requests.AddNodeRequest
				nodeRequest, err = types.GetMsgPayload[requests.AddNodeRequest](msgEnvelope)
				if err != nil {
					lc.Errorf("Failed to JSON decoding AddNodeRequest: %s", err.Error())
					res := types.NewMessageEnvelopeWithError(msgEnvelope.RequestID, err.Error())
					err = messageBus.Publish(res, responseTopic)
					if err != nil {
						lc.Errorf("Failed to publish node validation error response: %s", err.Error())
					}
					continue
				}

				err = driver.ValidateNode(dtos.ToNodeModel(nodeRequest.Node))
				if err != nil {
					lc.Errorf("Node validation failed: %s", err.Error())
					res := types.NewMessageEnvelopeWithError(msgEnvelope.RequestID, err.Error())
					err = messageBus.Publish(res, responseTopic)
					if err != nil {
						lc.Errorf("Failed to publish node validation error response: %s", err.Error())
					}
					continue
				}

				res, err := types.NewMessageEnvelopeForResponse(nil, msgEnvelope.RequestID, msgEnvelope.CorrelationID, common.ContentTypeJSON)
				if err != nil {
					lc.Errorf("Failed to create node validation response envelope: %s", err.Error())
					continue
				}

				err = messageBus.Publish(res, responseTopic)
				if err != nil {
					lc.Errorf("Failed to publish node validation response: %s", err.Error())
					continue
				}

				lc.Debugf("Node validation response published on message queue. Topic: %s, Correlation-id: %s", responseTopic, msgEnvelope.CorrelationID)
			}
		}
	}()

	return nil
}
