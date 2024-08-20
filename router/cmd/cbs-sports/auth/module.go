package auth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/wundergraph/cosmo/router/core"
	"go.uber.org/zap"
)

func init() {
	core.RegisterModule(&AuthModule{})
}

const (
	authModuleId       = "auth"
	authModulePriority = 1
	contextKey         = "userLogin"
	userLoginHeader    = "x-user-login"
)

type AuthModule struct {
	Enabled bool `mapstructure:"enabled"`
	sapi    *sapi
	cache   *cache
	store   *store
}

func (m *AuthModule) OnOriginRequest(req *http.Request, ctx core.RequestContext) (*http.Request, *http.Response) {
	userLogin := ctx.GetString(contextKey)

	if userLogin != "" {
		req.Header.Set(userLoginHeader, userLogin)
	}

	return req, nil
}

func (m *AuthModule) Provision(ctx *core.ModuleContext) error {
	if !m.Enabled {
		ctx.Logger.Warn("AUTH custom module is disabled")
		return nil
	}

	sapi, err := newSapi(ctx.Logger)
	if err != nil {
		return fmt.Errorf("could not connect to SAPI: %v", err)
	}
	m.sapi = sapi

	cache, err := newCache(ctx.Logger)
	if err != nil {
		return fmt.Errorf("could not connect to Redis: %v", err)
	}
	m.cache = cache

	store, err := newStore(ctx.Logger)
	if err != nil {
		return fmt.Errorf("could not connect to database: %v", err)
	}
	m.store = store

	return nil
}

func (m *AuthModule) Middleware(ctx core.RequestContext, next http.Handler) {
	if !m.Enabled {
		next.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
		return
	}

	authorizer, err := findAuthorizer(ctx)
	if err != nil {
		ctx.Logger().Warn(fmt.Sprintf("error when trying to get request authorizer: %v", err))
		next.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
		return
	}
	if authorizer == nil {
		ctx.Logger().Debug("No authorizer found in request")
		next.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
		return
	}

	userLogin, err := authorizer.userLogin()
	if err != nil || userLogin == "" {
		ctx.Logger().Warn(fmt.Sprintf("error when decrypting request authorizer: %v", err))
		next.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
		return
	}

	isUserCached, err := m.cache.exists(ctx.Request().Context(), userLogin)
	if err != nil {
		ctx.Logger().Warn(fmt.Sprintf("error when checking cached user: %v", err))
		next.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
		return
	}

	if !isUserCached {
		sapiUserValue, err := m.sapi.userDetails(ctx.Request().Context(), userLogin)
		if err != nil {
			ctx.Logger().Warn(fmt.Sprintf("error when requesting sapi user details: %v", err))
			next.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
			return
		}

		dbUserValue, err := m.store.find(ctx.Request().Context(), userLogin)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				ctx.Logger().Warn(fmt.Sprintf("error while getting db user: %v", err))
				next.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
				return
			}

			dbUserValue, err = m.store.save(ctx.Request().Context(), saveDbUser{
				UserLogin:    sql.NullString{String: sapiUserValue.UserLogin, Valid: sapiUserValue.UserLogin != ""},
				CustId:       sql.NullInt64{Int64: sapiUserValue.CustId, Valid: sapiUserValue.CustId > 0},
				EncryptedPid: sql.NullString{String: sapiUserValue.EncryptedPid, Valid: sapiUserValue.EncryptedPid != ""},
			})
			if err != nil {
				ctx.Logger().Warn(fmt.Sprintf("error while saving db user: %v", err))
				next.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
				return
			}
		}

		var extra *json.RawMessage
		if dbUserValue.Extra.Valid {
			temp := json.RawMessage(dbUserValue.Extra.String)
			extra = &temp
		}
		err = m.cache.saveUser(ctx.Request().Context(), cachedUser{
			Id:                 dbUserValue.Id,
			FirstName:          sapiUserValue.FirstName,
			LastName:           sapiUserValue.LastName,
			EncryptedPid:       sapiUserValue.EncryptedPid,
			PreferredEntryName: fmt.Sprintf("%s %s", sapiUserValue.FirstName, sapiUserValue.LastName),
			Email:              sapiUserValue.Email,
			CustId:             sapiUserValue.CustId,
			UserLogin:          sapiUserValue.UserLogin,
			AvatarUrl:          dbUserValue.AvatarUrl.String,
			Extra:              extra,
		})
		if err != nil {
			ctx.Logger().Warn(fmt.Sprintf("error while saving cache user: %v", err))
			next.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
			return
		}
	}

	ctx.Logger().Debug("setting context key", zap.String("key", contextKey), zap.String("userLogin", userLogin))
	ctx.Set(contextKey, userLogin)
	next.ServeHTTP(ctx.ResponseWriter(), ctx.Request())
}

func (m *AuthModule) Module() core.ModuleInfo {
	return core.ModuleInfo{
		ID:       authModuleId,
		Priority: authModulePriority,
		New: func() core.Module {
			return &AuthModule{}
		},
	}
}

var (
	_ core.RouterMiddlewareHandler = (*AuthModule)(nil)
	_ core.EnginePreOriginHandler  = (*AuthModule)(nil)
	_ core.Provisioner             = (*AuthModule)(nil)
)
