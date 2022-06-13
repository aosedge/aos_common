// SPDX-License-Identifier: Apache-2.0
//
// Copyright (C) 2022 Renesas Electronics Corporation.
// Copyright (C) 2022 EPAM Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// spaceallocator helper used to allocate disk space.
package spaceallocator

import "time"

/***********************************************************************************************************************
 * Types
 **********************************************************************************************************************/

// Allocator space allocator.
type Allocator struct{}

// Space allocated space.
type Space struct{}

// ItemRemover requests to remove item in order to free space.
type ItemRemover func(id string) error

/***********************************************************************************************************************
 * Public
 **********************************************************************************************************************/

// New creates new space allocator.
func New(path string, partLimit uint, remover ItemRemover) (*Allocator, error) {
	return nil, nil
}

// Close closes space allocator.
func (allocator *Allocator) Close() error {
	return nil
}

// AllocateSpace allocates space in storage.
func (allocator *Allocator) AllocateSpace(size uint64) (*Space, error) {
	return nil, nil
}

// Accept accepts previously allocated space.
func (space *Space) Accept() error {
	return nil
}

// Release releases previously allocated space.
func (space *Space) Release() error {
	return nil
}

// FreeSpace frees space in storage.
// This function should be called when storage item is removed by owner.
func (allocator *Allocator) FreeSpace(size uint64) {
}

// AddOutdatedItem adds outdated item.
// If there is no space to allocate, spaceallocator will try to free some space by calling ItemRemover function for
// outdated items. Item owner should remove this item. ItemRemover function is called based on item timestamp:
// oldest item should be removed first. After calling ItemRemover the item is automatically removed from outdated
// item list.
func (allocator *Allocator) AddOutdatedItem(id string, size uint64, timestamp time.Time) error {
	return nil
}

// RestoreOutdatedItem removes item from outdated item list.
func (allocator *Allocator) RestoreOutdatedItem(id string) {
}
