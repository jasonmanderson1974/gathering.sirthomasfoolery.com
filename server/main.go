package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"sirtom/server/db"
	"sirtom/server/logger"
	"sirtom/server/routes"
	"sirtom/server/services/reminders"

	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "sirtom/server/docs"
)

// @title The Fellowship API
// @version 1.0
// @description This is the API for The Fellowship!

// @host localhost:3002/api

// contentHashedAsset matches the eight-hex-character content hash Vue CLI puts
// in every filename it emits — app.8bd8de3a.js, chunk-vendors.a014b665.css,
// apple_logo.e6bf682d.svg. Those are safe to cache forever, because the name
// changes whenever the bytes do (J4).
//
// The gate is the hash rather than the directory on purpose: img/ holds both
// hashed build output and files copied verbatim from public/ (ogImage.png,
// when2meetOgImage2.png), and those keep their default revalidate-every-time
// behaviour, as do favicon.ico and robots.txt at the root.
//
// index.html is never registered by the static walk at all — it goes through
// noRouteHandler, which sets no-cache — so a deploy still propagates instantly
// even though the bundles it points at are pinned for a year.
var contentHashedAsset = regexp.MustCompile(`\.[0-9a-f]{8}\.`)

func init() {
	// AddExtensionType only errors on an extension not starting with ".", which
	// none of these can be — discard deliberately rather than leave it unchecked.
	for ext, typ := range map[string]string{
		".css":   "text/css",
		".js":    "application/javascript",
		".svg":   "image/svg+xml",
		".woff":  "font/woff",
		".woff2": "font/woff2",
		".ttf":   "font/ttf",
		".json":  "application/json",
		".map":   "application/json",
	} {
		_ = mime.AddExtensionType(ext, typ)
	}
}

func main() {
	// Set release flag
	release := flag.Bool("release", false, "Whether this is the release version of the server")
	flag.Parse()
	// Setenv only errors on an invalid (empty or "="-bearing) name.
	if *release {
		_ = os.Setenv("GIN_MODE", "release")
		gin.SetMode(gin.ReleaseMode)
	} else {
		_ = os.Setenv("GIN_MODE", "debug")
	}

	// Init logfile
	logFile, err := os.OpenFile("logs.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	gin.DefaultWriter = io.MultiWriter(logFile, os.Stdout)

	// Init logger
	logger.Init(logFile)

	// Load .env variables
	loadDotEnv()

	// Say so, at boot, when sign-in codes will be logged instead of mailed.
	//
	// `sendOtp` has a dev-only branch that prints the OTP rather than emailing
	// it, so local sign-in is possible at all (TODO3 M3). It is gated on debug
	// mode AND absent SMTP credentials — but the whole reason this line exists
	// is that "am I in release mode?" turned out to be a signal that can be
	// silently wrong: the dev image ran `-release=true`, overriding
	// compose.dev.yaml's `GIN_MODE: debug`, and nobody knew until a branch
	// depended on it. A server that is going to log credentials should announce
	// that on line one of its log, not leave it to be inferred.
	if gin.Mode() != gin.ReleaseMode && os.Getenv("GMAIL_APP_PASSWORD") == "" {
		logger.StdErr.Println(
			"DEV MODE: no SMTP credentials — sign-in codes will be LOGGED, not emailed",
		)
	}

	// Init router
	router := gin.New()
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		var statusColor, methodColor, resetColor string
		if param.IsOutputColor() {
			statusColor = param.StatusCodeColor()
			methodColor = param.MethodColor()
			resetColor = param.ResetColor()
		}

		if param.Latency > time.Minute {
			param.Latency = param.Latency.Truncate(time.Second)
		}
		return fmt.Sprintf("%v |%s %3d %s| %13v | %15s |%s %-7s %s %#v\n%s",
			param.TimeStamp.Format("2006/01/02 15:04:05"),
			statusColor, param.StatusCode, resetColor,
			param.Latency,
			param.ClientIP,
			methodColor, param.Method, resetColor,
			param.Path,
			param.ErrorMessage,
		)
	}))
	router.Use(gin.Recovery())

	// Cors
	corsOrigins := parseCorsOrigins(os.Getenv("CORS_ORIGINS"))
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"https://gathering.sirthomasfoolery.com", "http://localhost:8080"}
	}
	router.Use(cors.New(cors.Config{
		AllowOrigins:     corsOrigins,
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE"},
		AllowHeaders:     []string{"Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Init database
	closeConnection := db.Init()
	defer closeConnection()

	// B7: encrypt any Google/Outlook OAuth token still stored in the clear.
	// Idempotent and cheap — once everything carries the current version marker
	// it finds nothing. Runs before the router starts serving, so it never
	// races a request. Best-effort: a failure here leaves the tokens readable
	// (they are passed through as legacy plaintext), so it must not stop the
	// boot — but it does mean step 4 cannot land until this reports clean.
	if scanned, migrated, err := db.EncryptPlaintextOAuthTokens(); err != nil {
		logger.StdErr.Println("oauth token encryption sweep failed (tokens remain in the clear):", err)
	} else if migrated > 0 {
		logger.StdOut.Printf("encrypted stored OAuth tokens for %d of %d users with calendar accounts", migrated, scanned)
	}

	// Start the in-process email scheduler: pre-gathering reminders and
	// remindee nudges, both over Gmail SMTP
	stopReminders := reminders.StartReminderScheduler()
	defer stopReminders()

	// Session
	store := cookie.NewStore([]byte(os.Getenv("SESSION_SECRET")))
	// Harden the session cookie. Secure only in release mode so local dev over
	// http://localhost still works; SameSite=Lax blocks cross-site POSTs (CSRF)
	// while allowing the OAuth top-level redirect back to the app.
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
		HttpOnly: true,
		Secure:   gin.Mode() == gin.ReleaseMode,
		SameSite: http.SameSiteLaxMode,
	})
	router.Use(sessions.Sessions("session", store))

	// Init routes
	apiRouter := router.Group("/api")
	routes.InitHealth(apiRouter)
	routes.InitAuth(apiRouter)
	routes.InitUser(apiRouter)
	routes.InitUsers(apiRouter)
	routes.InitEvents(apiRouter)
	routes.InitChronicle(apiRouter)
	routes.InitFolders(apiRouter)
	routes.InitAdmin(apiRouter)

	frontendDist := os.Getenv("FRONTEND_DIST")
	if frontendDist == "" {
		frontendDist = "./frontend/dist"
		if _, err := os.Stat(frontendDist); os.IsNotExist(err) {
			frontendDist = "../frontend/dist"
		}
	}

	err = filepath.WalkDir(frontendDist, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() != "index.html" {
			// Get the path relative to frontendDist
			relPath, err := filepath.Rel(frontendDist, path)
			if err != nil || relPath == "" || relPath == "." {
				return nil
			}
			urlPath := fmt.Sprintf("/%s", relPath)
			if contentHashedAsset.MatchString(d.Name()) {
				filePath := path
				serve := func(c *gin.Context) {
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
					c.File(filePath)
				}
				// StaticFile registers both verbs; match it.
				router.GET(urlPath, serve)
				router.HEAD(urlPath, serve)
			} else {
				router.StaticFile(urlPath, path)
			}
		}
		return nil
	})
	if err != nil {
		logger.StdErr.Printf("Warning: failed to walk frontend dist: %s", err)
	}

	indexPath := filepath.Join(frontendDist, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		router.LoadHTMLFiles(indexPath)
	} else {
		logger.StdErr.Printf("Warning: index.html not found at %s", indexPath)
	}
	router.NoRoute(noRouteHandler())

	// Init swagger documentation
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	// Run server. Run only returns on failure — most often the port already
	// being held — and discarding that error made the process exit silently
	// with status 0, so a failed start looked like a clean shutdown.
	addr := ":3002"
	if os.Getenv("NODE_ENV") == "staging" {
		addr = ":3003"
	}
	// BIND_ADDR narrows which interfaces are served. The default is unchanged —
	// every interface — because that is what local development and the old
	// Docker deployment both relied on: the container bound broadly and Docker
	// published it on the host's loopback, so the outside world never saw it.
	//
	// Running the binary directly loses that second half. Without this, a host
	// on an isolated VLAN still serves the app to everything else on that VLAN,
	// straight past the tunnel. Production sets BIND_ADDR=127.0.0.1:3002.
	if bindAddr := os.Getenv("BIND_ADDR"); bindAddr != "" {
		addr = bindAddr
	}
	if err := router.Run(addr); err != nil {
		logger.StdErr.Fatalln("server failed to start:", err)
	}
}

// Load .env variables
func loadDotEnv() {
	err := godotenv.Load(".env")
	if err != nil {
		// .env file is optional - env vars can be passed directly (e.g., via Docker)
		logger.StdOut.Println("No .env file found, using environment variables")
	}

	// Validate secrets
	validateSessionSecret()
	validateEncryptionKey()
}

// parseCorsOrigins splits CORS_ORIGINS on commas, trimming surrounding
// whitespace and dropping empty entries.
//
// The trim is the point: a browser's Origin header carries no spaces, so the
// natural-looking `CORS_ORIGINS="a.example.com, b.example.com"` produced a
// literal " b.example.com" entry that could never match. The second origin was
// silently blocked, and CORS rejections surface only in the browser console —
// nothing server-side says why.
func parseCorsOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

// validateSessionSecret ensures SESSION_SECRET is set and meets security requirements
func validateSessionSecret() {
	secret := os.Getenv("SESSION_SECRET")

	if secret == "" {
		logger.StdErr.Panicln("SESSION_SECRET environment variable is required but not set")
	}

	// Minimum 32 characters for adequate security (256 bits)
	if len(secret) < 32 {
		logger.StdErr.Panicln("SESSION_SECRET must be at least 32 characters long")
	}
}

// validateEncryptionKey ensures ENCRYPTION_KEY is a usable AES key.
//
// It is passed to aes.NewCipher as raw bytes, which accepts ONLY 16, 24 or 32
// of them. Nothing checked that before, so a wrong-length key failed per
// request, deep inside a calendar call, long after boot — and the failure that
// makes this worth a startup check is a quiet one: DEPLOYMENT.md recommended
// generating it with `openssl rand -base64 32`, which produces a 44-character
// string that AES rejects outright. Fail the boot instead.
func validateEncryptionKey() {
	key := os.Getenv("ENCRYPTION_KEY")

	if key == "" {
		logger.StdErr.Panicln("ENCRYPTION_KEY environment variable is required but not set")
	}

	switch len(key) {
	case 16, 24, 32:
		// AES-128, AES-192, AES-256 respectively.
	default:
		logger.StdErr.Panicf(
			"ENCRYPTION_KEY must be exactly 16, 24 or 32 bytes (got %d). "+
				"Generate one with `openssl rand -hex 16` (32 chars). "+
				"Note `openssl rand -base64 32` yields 44 characters, which AES rejects.\n",
			len(key),
		)
	}
}

func noRouteHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// E3: no per-route meta tags. This used to look the event up by id and
		// put its NAME in the title and og:title — served to anyone, with no
		// session, which leaked gathering names to crawlers and to anyone who
		// guessed a short id. Behind a login, per-event OG previews are useless
		// anyway (nobody unauthenticated can open the link), so the shell just
		// serves its static default title.
		params := gin.H{}

		// The app shell must never be served stale. A browser holding a cached
		// index.html keeps referencing the previous build's hashed JS/CSS chunks,
		// which a later deploy has since removed — leaving returning visitors on a
		// broken/partial app (this is what silently broke the OTP-login flow after
		// a run of deploys). Force revalidation of the shell on every load; the
		// hashed assets themselves stay immutably cacheable (they're served by the
		// StaticFile routes above, not here, so this doesn't touch them).
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")

		c.HTML(http.StatusOK, "index.html", params)
	}
}
