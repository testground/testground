// TODO Package state will contain all (ORM) entities and services to track the
// execution of test plans against upstream commits and corresponding Git refs
// (branches, tags).
//
// It will not store metrics, artifacts, etc. Those will be stored in
// Elasticsearch, and the Engine will offer APIs to query both the state
// (sourced from this package) and the results, metrics and outputs (sourced
// from Elasticsearch), in a convenient and coherent way.
//
// The most suitable database for storing this data is probably SQLite -- the
// data is of relational kind, and at this stage, volume and load, it's not
// worth adopting a standalone engine like PostgreSQL.
//
// TODO Use jinzhu/gorm for relational ORM, and golang-migrate/migrate for
// schema migrations.
//
// If we want to go the KV store route, consider badger. Beware we'll need to
// replicate data in different shapes to create multitude of indices (e.g. so we
// can query all test runs pertaining to branch X of repo Y).
package state
