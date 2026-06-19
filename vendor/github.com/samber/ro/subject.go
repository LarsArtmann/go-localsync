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

package ro

// Subject is a sort of bridge or proxy, that acts both as an observer and
// as an Observable. Because it is an observer, it can subscribe to one
// or more Observables, and because it is an Observable, it can pass through
// the items it observes by reemitting them, and it can also emit new items.
type Subject[T any] interface {
	Observable[T]
	Observer[T]

	HasObserver() bool
	CountObservers() int

	IsClosed() bool
	HasThrown() bool
	IsCompleted() bool

	AsObservable() Observable[T]
	AsObserver() Observer[T]
}

// NewSubject is an alias to NewPublishSubject.
func NewSubject[T any]() Subject[T] {
	return NewPublishSubject[T]()
}
