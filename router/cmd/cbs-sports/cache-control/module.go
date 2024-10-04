package module

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-http-utils/headers"
	"github.com/wundergraph/cosmo/router/core"
	"go.uber.org/zap"
)

func init() {
	core.RegisterModule(&CacheControlModule{})
}

const (
	ModuleID            = "cacheControl"
	CacheControlContext = "CACHE_CONTROL_CONTEXT"
)

type CacheControlValue struct {
	maxAge     int
	visibility string
}

type CacheControlModule struct {
	Enabled bool `mapstructure:"enabled"`
	Logger  *zap.Logger
}

func (ccl CacheControlValue) String() string {
	return fmt.Sprintf("max-age=%d, %s", ccl.maxAge, ccl.visibility)
}

func (m *CacheControlModule) Provision(ctx *core.ModuleContext) error {
	if !m.Enabled {
		ctx.Logger.Warn("CacheControlModule is disabled")
		return nil
	}

	m.Logger = ctx.Logger
	ctx.Logger.Info("CacheControlModule initialized")

	return nil
}

func (m *CacheControlModule) OnOriginResponse(response *http.Response, ctx core.RequestContext) *http.Response {
	if !m.Enabled {
		return nil
	}

	var prevCacheControlValue = ctx.GetString(CacheControlContext)

	currentCacheControlValue := getCacheControlValue(response.Header)
	selectedCacheControlValue := currentCacheControlValue

	if prevCacheControlValue != "" {
		selectedCacheControlValue = getMostRestrictiveHeader(prevCacheControlValue, currentCacheControlValue)
	}

	ctx.Set(CacheControlContext, selectedCacheControlValue)
	m.Logger.Debug(fmt.Sprintf("CacheControlModule: cache-control value %s current: %s | selected: %s",
		ctx.ActiveSubgraph(response.Request).Name, currentCacheControlValue, selectedCacheControlValue))
	if selectedCacheControlValue != "" {
		ctx.ResponseWriter().Header().Set(headers.CacheControl, selectedCacheControlValue)
	}
	return nil
}

func getCacheControlValue(header http.Header) string {
	return header.Get(headers.CacheControl)
}

func getMostRestrictiveHeader(prevCacheControlValue string, currentCacheControlValue string) string {
	mostRestrictive := CacheControlValue{maxAge: 0, visibility: "private"}

	prevCacheValue := parseCacheValue(prevCacheControlValue)

	if strings.Contains(currentCacheControlValue, "no-store") || strings.Contains(currentCacheControlValue, "no-cache") {
		return prevCacheControlValue
	}

	currentCacheValue := parseCacheValue(currentCacheControlValue)

	if prevCacheValue.maxAge < currentCacheValue.maxAge {
		mostRestrictive.maxAge = prevCacheValue.maxAge
	} else {
		mostRestrictive.maxAge = currentCacheValue.maxAge
	}

	if prevCacheValue.visibility == "private" || currentCacheValue.visibility == "private" {
		mostRestrictive.visibility = "private"
	} else if prevCacheValue.visibility == "public" || currentCacheValue.visibility == "public" {
		mostRestrictive.visibility = "public"
	}

	return mostRestrictive.String()
}

func parseCacheValue(header string) CacheControlValue {
	parts := strings.Split(header, ",")
	var maxAge int
	var visibility string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			fmt.Sscanf(part, "max-age=%d", &maxAge)
		}
		if strings.HasPrefix(part, "private") {
			visibility = "private"
		} else if strings.HasPrefix(part, "public") {
			visibility = "public"
		}
	}

	return CacheControlValue{maxAge: maxAge, visibility: visibility}
}

func (m *CacheControlModule) Module() core.ModuleInfo {
	return core.ModuleInfo{
		ID: ModuleID,
		New: func() core.Module {
			return &CacheControlModule{}
		},
	}
}

var (
	_ core.Provisioner             = (*CacheControlModule)(nil)
	_ core.EnginePostOriginHandler = (*CacheControlModule)(nil)
)
