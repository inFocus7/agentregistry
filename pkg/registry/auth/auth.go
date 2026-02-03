package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

type Resource struct {
	Name string
	Type PermissionArtifactType
}

type User struct {
	Permissions []Permission
}

// Authn
type Principal struct {
	User User
}

type Session interface {
	Principal() Principal
}

type AuthnProvider interface {
	Authenticate(ctx context.Context, reqHeaders func(name string) string, query url.Values) (Session, error)
}

// context utils

type sessionKeyType struct{}

var (
	sessionKey = sessionKeyType{}
)

func AuthSessionFrom(ctx context.Context) (Session, bool) {
	v, ok := ctx.Value(sessionKey).(Session)
	return v, ok && v != nil
}

func AuthSessionTo(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

// todo: the middleware config is redefined here and router. should be consolidated.
// Middleware configuration options
type middlewareConfig struct {
	skipPaths map[string]bool
}

type MiddlewareOption func(*middlewareConfig)

func WithSkipPaths(paths ...string) MiddlewareOption {
	return func(c *middlewareConfig) {
		for _, path := range paths {
			c.skipPaths[path] = true
		}
	}
}

func AuthnMiddleware(authn AuthnProvider, options ...MiddlewareOption) func(ctx huma.Context, next func(huma.Context)) {
	fmt.Println("DEBUG: Calling AuthnMiddleware", authn)
	config := &middlewareConfig{
		skipPaths: make(map[string]bool),
	}
	for _, option := range options {
		option(config)
	}
	return func(ctx huma.Context, next func(huma.Context)) {
		fmt.Println("DEBUG: Inside AuthnMiddleware", authn)
		path := ctx.URL().Path

		// Skip authentication for specified paths
		// extract the last part of the path to match against skipPaths
		pathParts := strings.Split(path, "/")
		pathToMatch := "/" + pathParts[len(pathParts)-1]
		if config.skipPaths[pathToMatch] || config.skipPaths[path] {
			next(ctx)
			return
		}

		if authn == nil {
			// No auth provider configured, skip authentication
			next(ctx)
			return
		}
		url := ctx.URL()
		// is it right to pass ctx.Header? or pass a wrapper like func(name string) string {return ctx.Header(name)}?
		fmt.Println("DEBUG: Authenticating")
		session, err := authn.Authenticate(ctx.Context(), ctx.Header, url.Query())
		if err != nil {
			fmt.Println("DEBUG: Authentication error", err)
			ctx.SetStatus(http.StatusUnauthorized)
			_, _ = ctx.BodyWriter().Write([]byte("Unauthorized"))
			return
		}
		if session != nil {
			fmt.Println("DEBUG: Authentication successful, storing session", session)
			ctx = huma.WithContext(ctx, AuthSessionTo(ctx.Context(), session))
		}
		next(ctx)
	}
}
