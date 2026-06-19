// Copyright 2025 samber.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://github.com/samber/ro/blob/main/licenses/LICENSE.apache.md
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package xsync

import (
	"sync"
)

// RWMutex is a read-write mutex interface.
type RWMutex interface {
	TryLock() bool
	Lock()
	Unlock()
	TryRLock() bool
	RLock()
	RUnlock()
}

/************************
 *    Standard mutex    *
 ************************/

var _ RWMutex = (*RWMutexWithLock)(nil)

// NewRWMutexWithLock creates a new read-write mutex with a standard read-write mutex.
func NewRWMutexWithLock() *RWMutexWithLock {
	return &RWMutexWithLock{
		mu: sync.RWMutex{},
	}
}

// RWMutexWithLock is a read-write mutex with a standard read-write mutex.
type RWMutexWithLock struct {
	mu sync.RWMutex
}

// TryLock tries to lock the mutex.
func (m *RWMutexWithLock) TryLock() bool {
	return m.mu.TryLock()
}

// Lock locks the mutex.
func (m *RWMutexWithLock) Lock() {
	m.mu.Lock()
}

// Unlock unlocks the mutex.
func (m *RWMutexWithLock) Unlock() {
	m.mu.Unlock()
}

// TryRLock tries to lock the mutex for reading.
func (m *RWMutexWithLock) TryRLock() bool {
	return m.mu.TryRLock()
}

// RLock locks the mutex for reading.
func (m *RWMutexWithLock) RLock() {
	m.mu.RLock()
}

// RUnlock unlocks the mutex for reading.
func (m *RWMutexWithLock) RUnlock() {
	m.mu.RUnlock()
}

/************************
 *    Fast mutex        *
 ************************/

// @TODO

/************************
 *      Fake mutex      *
 ************************/

var _ RWMutex = (*RWMutexWithoutLock)(nil)

// NewRWMutexWithoutLock creates a new read-write mutex without a lock.
func NewRWMutexWithoutLock() *RWMutexWithoutLock {
	return &RWMutexWithoutLock{}
}

// RWMutexWithoutLock is a read-write mutex without a lock.
type RWMutexWithoutLock struct{}

// TryLock always returns true.
func (m *RWMutexWithoutLock) TryLock() bool {
	return true
}

// Lock does nothing.
func (m *RWMutexWithoutLock) Lock() {
}

// Unlock does nothing.
func (m *RWMutexWithoutLock) Unlock() {
}

// TryRLock always returns true.
func (m *RWMutexWithoutLock) TryRLock() bool {
	return true
}

// RLock does nothing.
func (m *RWMutexWithoutLock) RLock() {
}

// RUnlock does nothing.
func (m *RWMutexWithoutLock) RUnlock() {
}
