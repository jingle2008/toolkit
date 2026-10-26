/*
Package models holds toolkit's data model: the rows toolkit loads from
the repo, OCI and Kubernetes (base models, dedicated AI clusters, GPU
pools and nodes, tenants, limits, properties, ...) plus small, stable
helpers derived from their fields.

It is public so other tools and scripts can read toolkit's data. App
behavior shaped around the CLI, TUI or MCP server lives under internal/
instead.

# Stability

The API is unstable until v1. While toolkit is on v0.x, breaking changes
to this package may land in minor releases; each one is noted in
CHANGELOG.md.
*/
package models
