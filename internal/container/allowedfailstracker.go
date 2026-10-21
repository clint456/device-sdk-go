// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2025 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package container

// AllowedFailuresTracker wraps a map of node names to atomic integers that track the number of allowed request
// failures for each node.
type AllowedFailuresTracker struct {
	data map[string]*AtomicInt
}

// NewAllowedFailuresTracker creates and initializes a new tracker.
func NewAllowedFailuresTracker() AllowedFailuresTracker {
	return AllowedFailuresTracker{
		data: make(map[string]*AtomicInt),
	}
}

// Get retrieves the AtomicInt for a given node name.
// Returns nil if the node does not exist.
func (aft *AllowedFailuresTracker) Get(nodeName string) *AtomicInt {
	return aft.data[nodeName]
}

// Set initializes or updates the AtomicInt for a given node.
func (aft *AllowedFailuresTracker) Set(nodeName string, value int) {
	if _, exists := aft.data[nodeName]; !exists {
		aft.data[nodeName] = &AtomicInt{}
	}
	aft.data[nodeName].Set(value)
}

// Decrease decreases the AtomicInt value for a given node by 1.
// Returns the updated value or -1 if the node does not exist.
func (aft *AllowedFailuresTracker) Decrease(nodeName string) int {
	if atomicInt, exists := aft.data[nodeName]; exists {
		if atomicInt.Value() >= 0 {
			return atomicInt.Decrease()
		}
	}
	return -1
}

// Value retrieves the current value of the AtomicInt for a node.
// Returns -1 if the node does not exist.
func (aft *AllowedFailuresTracker) Value(nodeName string) int {
	if atomicInt, exists := aft.data[nodeName]; exists {
		return atomicInt.Value()
	}
	return -1
}

// Remove deletes the entry for a given node name from the tracker.
func (aft *AllowedFailuresTracker) Remove(nodeName string) {
	delete(aft.data, nodeName)
}
