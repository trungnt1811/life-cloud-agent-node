package router

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginswagger "github.com/swaggo/gin-swagger"

	"github.com/lifenetwork-ai/life-cloud-agent-node/conf"
	"github.com/lifenetwork-ai/life-cloud-agent-node/constants"
	_ "github.com/lifenetwork-ai/life-cloud-agent-node/docs"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/adapters/federated/client"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/handlers"
	middleware "github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/middleware"
	routev1 "github.com/lifenetwork-ai/life-cloud-agent-node/internal/delivery/http/route"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/di"
	"github.com/lifenetwork-ai/life-cloud-agent-node/internal/platform/logger"
)

const defaultCORSAllowedHeaders = "Content-Type,Content-Length,Accept-Encoding,X-CSRF-Token,Authorization,accept,origin,Cache-Control,X-Requested-With,X-Request-ID,X-Correlation-ID"

type Options struct {
	CORSAllowedOrigins []string
	CORSAllowedHeaders string
	RequestTimeout     time.Duration
	EnableSwagger      bool
	SwaggerAuthUser    string
	SwaggerAuthPass    string
	// AdminAuthUser/Pass gate /admin/query-fields. Deliberately separate
	// from SwaggerAuthUser/Pass - see AdminAuthOptions's doc comment.
	AdminAuthUser string
	AdminAuthPass string
	Env           string
	// NodeID/AgentVersion/Advisories back GET /admin/status (decision 0003).
	// Advisories is nil when the federated client worker is not running
	// (e.g. CONTROL_CENTER_ADDRESS unset); the endpoint then always reports
	// no advisory received.
	NodeID       string
	AgentVersion string
	Advisories   *client.AdvisoryStore
}

func SetupWithDependencies(useCases *di.UseCases, log logger.Logger, options ...Options) *gin.Engine {
	resolved := resolveOptions(options...)
	r := initialize(resolved, log)
	setupApplicationRoutes(r, useCases, log, resolved)
	registerSwaggerRoute(r, resolved, log)
	return r
}

func OptionsFromConfiguration(config *conf.Configuration) Options {
	return Options{
		CORSAllowedOrigins: ConfiguredCORSAllowedOrigins(config),
		CORSAllowedHeaders: ConfiguredCORSAllowedHeaders(config),
		RequestTimeout:     ConfiguredRequestTimeout(config),
		EnableSwagger:      config == nil || !isProductionLikeEnv(config.Env),
		SwaggerAuthUser:    strings.TrimSpace(valueOrEmpty(config, func(c *conf.Configuration) string { return c.SwaggerBasicAuthUser })),
		SwaggerAuthPass:    strings.TrimSpace(valueOrEmpty(config, func(c *conf.Configuration) string { return c.SwaggerBasicAuthPass })),
		AdminAuthUser:      strings.TrimSpace(valueOrEmpty(config, func(c *conf.Configuration) string { return c.AdminBasicAuthUser })),
		AdminAuthPass:      strings.TrimSpace(valueOrEmpty(config, func(c *conf.Configuration) string { return c.AdminBasicAuthPass })),
		Env:                valueOrEmpty(config, func(c *conf.Configuration) string { return c.Env }),
		NodeID:             strings.TrimSpace(valueOrEmpty(config, func(c *conf.Configuration) string { return c.NodeID })),
		AgentVersion:       strings.TrimSpace(valueOrEmpty(config, func(c *conf.Configuration) string { return c.AgentVersion })),
	}
}

func ConfiguredCORSAllowedOrigins(config *conf.Configuration) []string {
	if config == nil || strings.TrimSpace(config.CORS.AllowedOrigins) == "" {
		return []string{"*"}
	}

	origins := strings.Split(config.CORS.AllowedOrigins, ",")
	result := make([]string, 0, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			result = append(result, origin)
		}
	}
	if len(result) == 0 {
		return []string{"*"}
	}
	return result
}

func ConfiguredCORSAllowedHeaders(config *conf.Configuration) string {
	if config == nil || strings.TrimSpace(config.CORS.AllowedHeaders) == "" {
		return defaultCORSAllowedHeaders
	}
	return config.CORS.AllowedHeaders
}

func ConfiguredRequestTimeout(config *conf.Configuration) time.Duration {
	if config != nil && config.AppRequestTimeoutMs > 0 {
		return time.Duration(config.AppRequestTimeoutMs) * time.Millisecond
	}
	return time.Duration(constants.DefaultHTTPRequestMs) * time.Millisecond
}

func initialize(options Options, log logger.Logger) *gin.Engine {
	r := gin.New()
	if log == nil {
		log = logger.GetLogger()
	}

	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware(options.CORSAllowedOrigins, options.CORSAllowedHeaders))
	r.Use(middleware.RequestTracingMiddleware())
	r.Use(middleware.RequestLoggerWithLogger(log))
	r.Use(middleware.RequestTimeoutMiddleware(options.RequestTimeout))
	r.Use(middleware.RequestDataGuardMiddleware())
	r.Use(middleware.DefaultPagination())
	return r
}

func setupApplicationRoutes(r *gin.Engine, useCases *di.UseCases, log logger.Logger, options Options) {
	if useCases == nil {
		useCases = &di.UseCases{}
	}
	exampleHandler := handlers.NewExampleHandler(useCases.ExampleUseCase, log)
	enabledQueryFieldHandler := handlers.NewEnabledQueryFieldHandler(useCases.EnabledQueryFieldUseCase, log)
	statusHandler := handlers.NewStatusHandler(options.NodeID, options.AgentVersion, options.Advisories)
	adminAuth := routev1.AdminAuthOptions{
		Username: options.AdminAuthUser,
		Password: options.AdminAuthPass,
	}
	routev1.SetupHealthRoutes(r)
	routev1.SetupExampleRoutes(r, exampleHandler)
	routev1.SetupEnabledQueryFieldRoutes(r, enabledQueryFieldHandler, adminAuth, log)
	routev1.SetupStatusRoutes(r, statusHandler, adminAuth, log)
}

func registerSwaggerRoute(r *gin.Engine, options Options, log logger.Logger) {
	if r == nil || !options.EnableSwagger {
		return
	}
	if isProductionLikeEnv(options.Env) {
		if log != nil {
			log.Warn("Swagger route disabled in production-like environment")
		}
		return
	}

	swaggerHandler := ginswagger.WrapHandler(swaggerfiles.Handler)
	switch {
	case options.SwaggerAuthUser == "" && options.SwaggerAuthPass == "":
		r.GET("/swagger/*any", swaggerHandler)
	case options.SwaggerAuthUser != "" && options.SwaggerAuthPass != "":
		r.GET("/swagger/*any", middleware.SwaggerBasicAuth(options.SwaggerAuthUser, options.SwaggerAuthPass), swaggerHandler)
	default:
		if log != nil {
			log.Warn("Swagger basic auth is misconfigured; route will not be registered")
		}
	}
}

func resolveOptions(options ...Options) Options {
	resolved := Options{
		CORSAllowedOrigins: []string{"*"},
		CORSAllowedHeaders: defaultCORSAllowedHeaders,
		RequestTimeout:     time.Duration(constants.DefaultHTTPRequestMs) * time.Millisecond,
		EnableSwagger:      true,
	}
	if len(options) == 0 {
		return resolved
	}

	resolved = options[0]
	if len(resolved.CORSAllowedOrigins) == 0 {
		resolved.CORSAllowedOrigins = []string{"*"}
	}
	if strings.TrimSpace(resolved.CORSAllowedHeaders) == "" {
		resolved.CORSAllowedHeaders = defaultCORSAllowedHeaders
	}
	if resolved.RequestTimeout <= 0 {
		resolved.RequestTimeout = time.Duration(constants.DefaultHTTPRequestMs) * time.Millisecond
	}
	return resolved
}

func isProductionLikeEnv(env string) bool {
	switch strings.ToLower(strings.TrimSpace(env)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

func valueOrEmpty(config *conf.Configuration, selector func(*conf.Configuration) string) string {
	if config == nil || selector == nil {
		return ""
	}
	return selector(config)
}
