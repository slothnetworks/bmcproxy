//go:build tools

// This file pins third-party modules that later packages depend on
// (internal/metrics -> prometheus, pkg/protocol/ipmi -> go-ipmi,
// internal/cache -> singleflight, internal/server -> swaggo) so `go mod tidy` keeps them in go.mod
// before those packages exist. It is never compiled into the binary.
package transport

import (
	_ "github.com/bougou/go-ipmi/pkg/client"
	_ "github.com/getkin/kin-openapi/openapi2conv"
	_ "github.com/prometheus/client_golang/prometheus"
	_ "github.com/prometheus/client_golang/prometheus/promhttp"
	_ "github.com/swaggo/http-swagger/v2"
	_ "github.com/swaggo/swag"
	_ "golang.org/x/sync/singleflight"
)
