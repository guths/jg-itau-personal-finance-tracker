package provider

import "jgfinance/internal/pdftext"

// Registry holds the known providers, asked in registration order.
type Registry struct {
	providers []Provider
}

// NewRegistry returns a registry asking providers in the given order.
func NewRegistry(providers ...Provider) *Registry {
	return &Registry{providers: providers}
}

// Detect returns the first provider that recognizes doc.
func (r *Registry) Detect(doc *pdftext.Document) (Provider, DocKind, error) {
	for _, p := range r.providers {
		if kind, ok := p.Detect(doc); ok {
			return p, kind, nil
		}
	}
	return nil, "", ErrUnknownDocument
}
