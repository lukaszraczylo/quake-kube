package content

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Config struct {
	AssetsDir string
}

// Optimize router for maximum performance
func NewRouter(cfg *Config) (*echo.Echo, error) {
	// Pre-generate assets manifest so its dynamic generation won't hammer the server
	// This initial cache warming is critical for performance
	assets, err := getAssets(cfg.AssetsDir)
	if err != nil {
		return nil, err
	}
	
	// Pre-marshal the assets to JSON for faster response times
	assetsJSON, err := json.MarshalIndent(assets, "", "   ")
	if err != nil {
		return nil, err
	}
	
	// Create an optimized echo instance
	e := echo.New()
	
	// Set performance-focused configuration
	e.HideBanner = true
	e.HidePort = true
	e.DisableHTTP2 = false // Ensure HTTP/2 is enabled for better performance
	
	// Enable gzip compression for better network performance
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5, // Balanced compression level for speed vs size
	}))
	
	// Configure middleware for optimal performance
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		StackSize: 1 << 10, // 1KB, smaller stack for better performance
	}))
	
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "${method} ${uri} ${status} ${latency_human}\n",
	}))
	
	// Optimize CORS configuration
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
		// Cache preflight requests for 1 hour for better browser performance
		MaxAge: 3600,
	}))
	
	// Index page
	e.GET("/", func(c echo.Context) error {
		return c.HTML(http.StatusOK, `<!doctype html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <title>Map pack upload</title>
</head>
<body>

<a href="/maps">Show maps</a>

<h1>Upload map pack file</h1>

<form action="/maps" method="post" enctype="multipart/form-data">
	GameName: <input type="text" name="name" value="baseq3" /><br>
    Files: <input type="file" name="file"><br><br>
    <input type="submit" value="Submit">
</form>
</body>
</html>`)
	})
	
	// Use cached and pre-marshalled JSON for manifest
	e.GET("/assets/manifest.json", func(c echo.Context) error {
		c.Response().Header().Set("Content-Type", "application/json")
		c.Response().Header().Set("Cache-Control", "public, max-age=60") // Cache for 60 seconds
		return c.JSONBlob(http.StatusOK, assetsJSON)
	})
	
	// Optimized asset file serving with caching headers
	e.GET("/assets/*", func(c echo.Context) error {
		path := filepath.Join(cfg.AssetsDir, trimAssetName(c.Param("*")))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return c.String(http.StatusNotFound, "file not found")
		}
		
		// Add cache headers for static assets to improve client performance
		c.Response().Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
		return c.File(path)
	})

	// Optimized maps endpoint
	e.GET("/maps", func(c echo.Context) error {
		maps, err := getMaps(cfg.AssetsDir)
		if err != nil {
			return err
		}
		// Add cache control headers
		c.Response().Header().Set("Cache-Control", "public, max-age=60") // Cache for 60 seconds
		return c.JSONPretty(http.StatusOK, maps, "    ")
	})
	
	// Map upload endpoint
	e.POST("/maps", func(c echo.Context) error {
		name := c.FormValue("name")
		file, err := c.FormFile("file")
		if err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		defer src.Close()

		if hasExts(file.Filename, ".zip") {
			r, err := zip.NewReader(src, file.Size)
			if err != nil {
				return err
			}
			files := make([]string, 0, len(r.File)) // Pre-allocate capacity for better performance
			for _, f := range r.File {
				if !hasExts(f.Name, ".pk3") {
					continue
				}
				pak, err := f.Open()
				if err != nil {
					return err
				}
				defer pak.Close()

				dst, err := os.Create(filepath.Join(cfg.AssetsDir, name, filepath.Base(f.Name)))
				if err != nil {
					return err
				}
				defer dst.Close()

				// Use a buffer for more efficient copying
				buf := make([]byte, 32*1024)
				if _, err = io.CopyBuffer(dst, pak, buf); err != nil {
					return err
				}
				files = append(files, filepath.Base(f.Name))
			}
			if len(files) == 0 {
				return c.HTML(http.StatusOK, fmt.Sprintf("<p>File %s did not contain any map pack files.</p>", file.Filename))
			}
			for i := range files {
				files[i] = "<li>" + files[i] + "</li>"
			}
			return c.HTML(http.StatusOK, fmt.Sprintf("<p>Loaded the following map packs from file %s:</p><ul>%s</ul>", file.Filename, strings.Join(files, "")))
		}
		
		dst, err := os.Create(filepath.Join(cfg.AssetsDir, name, file.Filename))
		if err != nil {
			return err
		}
		defer dst.Close()

		// Use buffered copy for better performance
		buf := make([]byte, 32*1024)
		if _, err = io.CopyBuffer(dst, src, buf); err != nil {
			return err
		}
		return c.HTML(http.StatusOK, fmt.Sprintf("<p>File %s uploaded successfully.</p>", filepath.Join(name, file.Filename)))
	})
	
	return e, nil
}

// trimAssetName returns a path string that has been prefixed with a crc32
// checksum.
func trimAssetName(s string) string {
	d, f := filepath.Split(s)
	f = f[strings.Index(f, "-")+1:]
	return filepath.Join(d, f)
}
