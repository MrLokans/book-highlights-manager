// Package http — Store interface documentation.
//
// Controller-specific interfaces follow the Interface Segregation Principle,
// defined in their respective files:
//
//   - TagStore (tags.go): tag CRUD, book/highlight tag associations, search
//   - DeleteStore (delete.go): soft and permanent delete for books/highlights
//   - FavouritesStore (favourites.go): favourite toggle and retrieval
//   - VocabularyStore (vocabulary.go): word CRUD, definitions, enrichment
//   - DatabasePinger (health.go): database connectivity check
//
// BookReader and BookExporter are defined in internal/exporters/generic.go.
package http
