package mqtt

import "context"

// staticPlugLookup is an in-memory plugLookup for tests.
type staticPlugLookup struct {
	namespace string
	slug      string
}

// NewStaticPlugLookup returns a plugLookup that always returns the given namespace and slug.
// Intended for tests.
func NewStaticPlugLookup(namespace, slug string) plugLookup {
	return &staticPlugLookup{namespace: namespace, slug: slug}
}

func (s *staticPlugLookup) NamespaceAndSlug(_ context.Context, _ string) (string, string, error) {
	return s.namespace, s.slug, nil
}
