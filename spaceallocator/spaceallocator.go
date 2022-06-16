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

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/aoscloud/aos_common/aoserrors"
	"github.com/aoscloud/aos_common/utils/fs"
	log "github.com/sirupsen/logrus"
)

/***********************************************************************************************************************
 * Types
 **********************************************************************************************************************/

// Allocator space allocator.
type Allocator struct {
	path    string
	part    *partition
	remover ItemRemover
}

// Space allocated space.
type Space struct {
	size uint64
	path string
	part *partition
}

// ItemRemover requests to remove item in order to free space.
type ItemRemover func(id string) error

type partition struct {
	sync.Mutex

	mountPoint      string
	allocatorCount  uint
	allocationCount uint
	availableSize   uint64
	outdatedItems   []outdatedItem
}

type outdatedItem struct {
	id        string
	size      uint64
	timestamp time.Time
	remove    ItemRemover
}

type byTimestamp []outdatedItem

/***********************************************************************************************************************
 * Vars
 **********************************************************************************************************************/

// nolint:gochecknoglobals // common partitions storage
var (
	partsMutex sync.Mutex
	partsMap   map[string]*partition
)

var (
	ErrNoAllocation = errors.New("no allocation in progress")
	ErrNoSpace      = errors.New("not enough space")
)

/***********************************************************************************************************************
 * Public
 **********************************************************************************************************************/

// New creates new space allocator.
func New(path string, partLimit uint, remover ItemRemover) (*Allocator, error) {
	partsMutex.Lock()
	defer partsMutex.Unlock()

	mountPoint, err := fs.GetMountPoint(path)
	if err != nil {
		return nil, aoserrors.Wrap(err)
	}

	log.WithFields(log.Fields{"path": path, "partLimit": partLimit, "mountPoint": mountPoint}).Debug("Create allocator")

	part, ok := partsMap[mountPoint]
	if !ok {
		if partsMap == nil {
			partsMap = make(map[string]*partition)
		}

		part = &partition{mountPoint: mountPoint}
		partsMap[mountPoint] = part
	}

	part.allocatorCount++

	return &Allocator{
		path:    path,
		part:    part,
		remover: remover,
	}, nil
}

// Close closes space allocator.
func (allocator *Allocator) Close() error {
	partsMutex.Lock()
	defer partsMutex.Unlock()

	log.WithFields(log.Fields{"path": allocator.path}).Debug("Close allocator")

	allocator.part.allocatorCount--

	if allocator.part.allocatorCount == 0 {
		delete(partsMap, allocator.part.mountPoint)
	}

	return nil
}

// AllocateSpace allocates space in storage.
func (allocator *Allocator) AllocateSpace(size uint64) (*Space, error) {
	log.WithFields(log.Fields{"path": allocator.path, "size": size}).Debug("Allocate space")

	if err := allocator.part.allocateSpace(size); err != nil {
		return nil, err
	}

	return &Space{size: size, path: allocator.path, part: allocator.part}, nil
}

// Accept accepts previously allocated space.
func (space *Space) Accept() error {
	log.WithFields(log.Fields{"path": space.path, "size": space.size}).Debug("Space accepted")

	return space.part.allocateDone()
}

// Release releases previously allocated space.
func (space *Space) Release() error {
	log.WithFields(log.Fields{"path": space.path, "size": space.size}).Debug("Space released")

	space.part.freeSpace(space.size)

	return space.part.allocateDone()
}

// FreeSpace frees space in storage.
// This function should be called when storage item is removed by owner.
func (allocator *Allocator) FreeSpace(size uint64) {
	log.WithFields(log.Fields{"path": allocator.path, "size": size}).Debug("Free space")

	allocator.part.freeSpace(size)
}

// AddOutdatedItem adds outdated item.
// If there is no space to allocate, spaceallocator will try to free some space by calling ItemRemover function for
// outdated items. Item owner should remove this item. ItemRemover function is called based on item timestamp:
// oldest item should be removed first. After calling ItemRemover the item is automatically removed from outdated
// item list.
func (allocator *Allocator) AddOutdatedItem(id string, size uint64, timestamp time.Time) error {
	log.WithFields(log.Fields{
		"path": allocator.path, "id": id, "size": size, "timestamp": timestamp,
	}).Debug("Add outdated item")

	if allocator.remover == nil {
		return aoserrors.New("no item remover")
	}

	allocator.part.addOutdatedItem(outdatedItem{id: id, size: size, remove: allocator.remover, timestamp: timestamp})

	return nil
}

// RestoreOutdatedItem removes item from outdated item list.
func (allocator *Allocator) RestoreOutdatedItem(id string) {
	log.WithFields(log.Fields{"path": allocator.path, "id": id}).Debug("Restore outdated item")

	allocator.part.restoreOutdatedItem(id)
}

/***********************************************************************************************************************
 * byTimestamp
 **********************************************************************************************************************/

func (items byTimestamp) Len() int           { return len(items) }
func (items byTimestamp) Less(i, j int) bool { return items[i].timestamp.Before(items[j].timestamp) }
func (items byTimestamp) Swap(i, j int)      { items[i], items[j] = items[j], items[i] }

/***********************************************************************************************************************
 * Private
 **********************************************************************************************************************/

func (part *partition) allocateSpace(size uint64) error {
	part.Lock()
	defer part.Unlock()

	if part.allocationCount == 0 {
		availableSize, err := fs.GetAvailableSize(part.mountPoint)
		if err != nil {
			return aoserrors.Wrap(err)
		}

		part.availableSize = uint64(availableSize)

		log.WithFields(log.Fields{
			"mountPoint": part.mountPoint, "size": part.availableSize,
		}).Debug("Initial partition space")
	}

	if size > part.availableSize {
		freedSize, err := part.removeOutdatedItems(size - part.availableSize)
		if err != nil {
			return err
		}

		part.availableSize += freedSize
	}

	part.availableSize -= size
	part.allocationCount++

	log.WithFields(log.Fields{
		"mountPoint": part.mountPoint, "size": part.availableSize,
	}).Debug("Available partition space")

	return nil
}

func (part *partition) freeSpace(size uint64) {
	part.Lock()
	defer part.Unlock()

	if part.allocationCount > 0 {
		part.availableSize += size

		log.WithFields(log.Fields{
			"mountPoint": part.mountPoint, "size": part.availableSize,
		}).Debug("Available partition space")
	}
}

func (part *partition) allocateDone() error {
	part.Lock()
	defer part.Unlock()

	if part.allocationCount == 0 {
		return ErrNoAllocation
	}

	part.allocationCount--

	return nil
}

func (part *partition) addOutdatedItem(item outdatedItem) {
	part.Lock()
	defer part.Unlock()

	i := 0

	for ; i < len(part.outdatedItems); i++ {
		if part.outdatedItems[i].id == item.id {
			part.outdatedItems[i] = item

			break
		}
	}

	if i == len(part.outdatedItems) {
		part.outdatedItems = append(part.outdatedItems, item)
	}
}

func (part *partition) restoreOutdatedItem(id string) {
	part.Lock()
	defer part.Unlock()

	for i, item := range part.outdatedItems {
		if item.id == id {
			part.outdatedItems = append(part.outdatedItems[:i], part.outdatedItems[i+1:]...)

			break
		}
	}
}

func (part *partition) removeOutdatedItems(requiredSize uint64) (freedSize uint64, err error) {
	var totalSize uint64

	for _, item := range part.outdatedItems {
		totalSize += item.size
	}

	if requiredSize > totalSize {
		return 0, ErrNoSpace
	}

	log.WithFields(log.Fields{
		"mountPoint": part.mountPoint, "requiredSize": requiredSize,
	}).Debug("Remove outdated items")

	sort.Sort(byTimestamp(part.outdatedItems))

	i := 0

	for ; freedSize < requiredSize; i++ {
		item := part.outdatedItems[i]

		log.WithFields(log.Fields{
			"mountPoint": part.mountPoint, "id": item.id, "size": item.size,
		}).Debug("Remove outdated item")

		if err = item.remove(item.id); err != nil {
			return freedSize, err
		}

		freedSize += item.size
	}

	part.outdatedItems = part.outdatedItems[i:]

	return freedSize, nil
}
