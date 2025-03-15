package client

import (
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	quakenet "github.com/lukaszraczylo/quake-kube/internal/quake/net"
)

type Config struct {
	ContentServerURL string
	ServerAddr       string

	Files http.FileSystem
}

func NewRouter(cfg *Config) (*echo.Echo, error) {
	// Create optimized Echo instance
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	
	// Configure middleware for optimal performance
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		StackSize: 1 << 10, // 1KB, optimized stack size
	}))
	
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "${method} ${uri} ${status} ${latency_human}\n",
	}))
	
	// Add gzip compression for better network efficiency
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5, // Balance between compression and CPU usage
	}))
	
	// Optimize CORS configuration with caching
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
		MaxAge: 3600, // Cache preflight requests for 1 hour
	}))

	// Load and parse template only once at startup
	f, err := cfg.Files.Open("index.html")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read with proper buffer sizing
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	
	// Create optimized template with caching
	templates, err := template.New("index").Parse(string(data))
	if err != nil {
		return nil, err
	}
	e.Renderer = &TemplateRenderer{templates}

	// default route
	e.GET("/", func(c echo.Context) error {
		m, err := quakenet.GetInfo(cfg.ServerAddr)
		if err != nil {
			return err
		}
		needsPass := false
		if v, ok := m["g_needpass"]; ok {
			if v == "1" {
				needsPass = true
			}
		}
		return c.Render(http.StatusOK, "index", map[string]interface{}{
			"ServerAddr": cfg.ServerAddr,
			"NeedsPass":  needsPass,
		})
	})

	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	e.GET("/info", func(c echo.Context) error {
		m, err := quakenet.GetInfo(cfg.ServerAddr)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, m)
	})

	e.GET("/status", func(c echo.Context) error {
		m, err := quakenet.GetStatus(cfg.ServerAddr)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, m)
	})

	// static files
	e.GET("/*", echo.WrapHandler(http.FileServer(cfg.Files)))

	// Quake3 assets requests must be proxied to the content server. The host
	// header is manipulated to ensure that services like CloudFlare will not
	// reject requests based upon incorrect host header.
	csurl, err := url.Parse(cfg.ContentServerURL)
	if err != nil {
		return nil, err
	}
	g := e.Group("/assets")
	g.Use(middleware.ProxyWithConfig(middleware.ProxyConfig{
		Balancer: middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
			{URL: csurl},
		}),
		Transport: &HostHeaderTransport{RoundTripper: http.DefaultTransport, Host: csurl.Host},
	}))
	return e, nil
}

type HostHeaderTransport struct {
	http.RoundTripper
	Host string
}

func (t *HostHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Host = t.Host
	return t.RoundTripper.RoundTrip(req)
}

type TemplateRenderer struct {
	*template.Template
}

func (t *TemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.ExecuteTemplate(w, name, data)
}
