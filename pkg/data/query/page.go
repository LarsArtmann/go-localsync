package query

// Page is a generic paginated result for any entity type T.
// Use it for repositories, APIs, and read models.
type Page[T any] struct {
	Items   []T
	Total   int64
	Limit   int
	Offset  int
	HasMore bool
}

// NewPage creates a Page from a slice of items and total count.
// It computes HasMore based on limit and offset.
func NewPage[T any](items []T, total int64, limit, offset int) Page[T] {
	return Page[T]{
		Items:   items,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: limit > 0 && int64(offset+len(items)) < total,
	}
}

// EmptyPage returns a zero-value page with initialized slice.
func EmptyPage[T any]() Page[T] {
	return Page[T]{
		Items:   []T{},
		Total:   0,
		Limit:   0,
		Offset:  0,
		HasMore: false,
	}
}

// MapPage applies a transformation to each item in the page, returning a new page.
func MapPage[T, U any](page Page[T], fn func(T) U) Page[U] {
	mapped := make([]U, len(page.Items))

	for i, item := range page.Items {
		mapped[i] = fn(item)
	}

	return Page[U]{
		Items:   mapped,
		Total:   page.Total,
		Limit:   page.Limit,
		Offset:  page.Offset,
		HasMore: page.HasMore,
	}
}
