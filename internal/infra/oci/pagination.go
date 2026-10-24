package oci

// listAllPages drains a paginated OCI List* call. fetch is called with the
// page token (nil for the first page) and returns that page's items plus its
// OpcNextPage; the loop stops when the next token is nil or empty. Reading
// only page 1 silently drops everything past it.
func listAllPages[T any](fetch func(page *string) ([]T, *string, error)) ([]T, error) {
	var all []T
	var page *string
	for {
		items, next, err := fetch(page)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if next == nil || *next == "" {
			return all, nil
		}
		page = next
	}
}
