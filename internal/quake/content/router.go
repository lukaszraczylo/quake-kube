package content

import (
	"archive/zip"
	"crypto/subtle"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	maxUploadBytes = 2 << 30
	maxPackBytes   = 1 << 30
)

type Config struct {
	AssetsDir string
	// UploadToken, when set, is required as a bearer token on POST /maps.
	UploadToken string
}

// Optimize router for maximum performance
func NewRouter(cfg *Config) (*echo.Echo, error) {
	st := newStore(cfg.AssetsDir)
	if _, err := st.manifest(); err != nil {
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
		// pk3 files are already zip-compressed.
		Skipper: func(c echo.Context) bool { return strings.HasPrefix(c.Request().URL.Path, "/assets/") },
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
		assetsJSON, err := st.manifest()
		if err != nil {
			return err
		}
		c.Response().Header().Set("Cache-Control", "public, max-age=60") // Cache for 60 seconds
		return c.JSONBlob(http.StatusOK, assetsJSON)
	})

	// Optimized asset file serving with caching headers
	e.GET("/assets/*", func(c echo.Context) error {
		path, err := safeJoin(cfg.AssetsDir, trimAssetName(c.Param("*")))
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid path")
		}
		if fi, err := os.Stat(path); err != nil || fi.IsDir() {
			return c.String(http.StatusNotFound, "file not found")
		}

		// Add cache headers for static assets to improve client performance
		c.Response().Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
		return c.File(path)
	})

	// Optimized maps endpoint
	e.GET("/maps", func(c echo.Context) error {
		maps, err := st.mapList()
		if err != nil {
			return err
		}
		// Add cache control headers
		c.Response().Header().Set("Cache-Control", "public, max-age=60") // Cache for 60 seconds
		return c.JSONPretty(http.StatusOK, maps, "    ")
	})

	// Map upload endpoint
	e.POST("/maps", func(c echo.Context) error {
		if !uploadAllowed(cfg.UploadToken, c.Request().Header.Get(echo.HeaderAuthorization)) {
			return echo.NewHTTPError(http.StatusUnauthorized, "upload token required")
		}
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxUploadBytes)
		name := c.FormValue("name")
		if name == "" {
			name = "."
		}
		dir, err := safeJoin(cfg.AssetsDir, name)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid name")
		}
		file, err := c.FormFile("file")
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		defer src.Close()
		defer st.invalidate()

		if hasExts(file.Filename, ".zip") {
			r, err := zip.NewReader(src, file.Size)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, err.Error())
			}
			files := make([]string, 0, len(r.File))
			for _, f := range r.File {
				if !hasExts(f.Name, ".pk3") {
					continue
				}
				base := filepath.Base(f.Name)
				if err := savePack(f, filepath.Join(dir, base)); err != nil {
					return err
				}
				files = append(files, "<li>"+html.EscapeString(base)+"</li>")
			}
			if len(files) == 0 {
				return c.HTML(http.StatusOK, fmt.Sprintf("<p>File %s did not contain any map pack files.</p>", html.EscapeString(file.Filename)))
			}
			return c.HTML(http.StatusOK, fmt.Sprintf("<p>Loaded the following map packs from file %s:</p><ul>%s</ul>", html.EscapeString(file.Filename), strings.Join(files, "")))
		}

		if err := writeFileAtomic(filepath.Join(dir, filepath.Base(file.Filename)), src); err != nil {
			return err
		}
		return c.HTML(http.StatusOK, fmt.Sprintf("<p>File %s uploaded successfully.</p>", html.EscapeString(filepath.Join(name, file.Filename))))
	})

	return e, nil
}

// uploadAllowed accepts everything when no token is configured.
func uploadAllowed(token, header string) bool {
	if token == "" {
		return true
	}
	got, ok := strings.CutPrefix(header, "Bearer ")
	return ok && subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

// savePack extracts one zip entry, capped at maxPackBytes against zip bombs.
func savePack(f *zip.File, dst string) error {
	pak, err := f.Open()
	if err != nil {
		return err
	}
	defer closeQuietly(pak)
	return writeFileAtomic(dst, io.LimitReader(pak, maxPackBytes))
}

// writeFileAtomic writes r to a temp file and renames it into place, so a
// failed upload never leaves a truncated pk3 that gets checksummed and served.
func writeFileAtomic(path string, r io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // served assets must be world-readable
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after a successful rename
	if _, err := io.Copy(tmp, r); err != nil {
		closeQuietly(tmp)
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil { //nolint:gosec // served assets must be world-readable
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// safeJoin joins an untrusted relative path under root, rejecting escapes.
func safeJoin(root, rel string) (string, error) {
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("path %q escapes assets dir", rel)
	}
	return filepath.Join(root, rel), nil
}

// trimAssetName returns a path string that has been prefixed with a crc32
// checksum.
func trimAssetName(s string) string {
	d, f := filepath.Split(s)
	f = f[strings.Index(f, "-")+1:]
	return filepath.Join(d, f)
}
