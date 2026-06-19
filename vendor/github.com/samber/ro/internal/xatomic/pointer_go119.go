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

//go:build go1.19

package xatomic

import "sync/atomic"

// Pointer is a wrapper around the standard atomic.Pointer[T] for Go 1.19+.
// This provides a consistent API across Go versions.
type Pointer[T any] struct {
	p atomic.Pointer[T]
}

// NewPointer returns a new Pointer[T] initialized with the given value.
func NewPointer[T any](v *T) *Pointer[T] {
	var p Pointer[T]
	p.Store(v)
	return &p
}

// Load returns the value stored in the pointer atomically.
func (x *Pointer[T]) Load() *T {
	return x.p.Load()
}

// Store stores the value in the pointer atomically.
func (x *Pointer[T]) Store(val *T) {
	x.p.Store(val)
}

// Swap swaps the value in the pointer with the new value and returns the old value atomically.
func (x *Pointer[T]) Swap(val *T) (old *T) {
	return x.p.Swap(val)
}

// CompareAndSwap performs a compare-and-swap operation on the pointer atomically.
// It stores new in the pointer if the current value is equal to old.
// It returns true if the swap was performed, false otherwise.
func (x *Pointer[T]) CompareAndSwap(old, nEw *T) (swapped bool) {
	return x.p.CompareAndSwap(old, nEw)
}
