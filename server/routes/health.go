// Health check for the deployment: the one endpoint the deploy script, the
// systemd unit and the cutover checklist all gate on.
//
// It is deliberately unauthenticated — it has to answer before anyone is signed
// in, and it reveals nothing a visitor couldn't infer from the site being up.
package routes

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"sirtom/server/db"
	"sirtom/server/responses"
)

// Version is the release this binary was built from. Set at build time with
//
//	-ldflags "-X sirtom/server/routes.Version=<sha>"
//
// and left as "dev" otherwise (a local `go run`, or a build that forgot). The
// deploy script reads it back to prove the release it just shipped is the one
// actually serving, rather than trusting that a restart took.
var Version = "dev"

// healthTimeout bounds the Mongo ping. Short on purpose: a health check that
// can hang is worse than one that fails, because the caller — a deploy waiting
// to decide whether to roll back — learns nothing while it waits.
const healthTimeout = 3 * time.Second

func InitHealth(router *gin.RouterGroup) {
	router.GET("/health", getHealth)
}

// @Summary Reports whether the server can serve traffic
// @Description Unauthenticated liveness/readiness probe. Returns 200 when the server can reach MongoDB, 503 when it cannot. Used by the deploy script to gate a release and roll back automatically.
// @Tags health
// @Produce json
// @Success 200 {object} responses.Health
// @Failure 503 {object} responses.Health
// @Router /health [get]
func getHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), healthTimeout)
	defer cancel()

	if err := db.Ping(ctx); err != nil {
		// 503, not 500: the server is fine, its dependency isn't, and that
		// distinction is what tells a deploy to roll back rather than retry.
		c.JSON(http.StatusServiceUnavailable, responses.Health{
			Status:  "unavailable",
			Mongo:   "down",
			Version: Version,
		})
		return
	}

	c.JSON(http.StatusOK, responses.Health{
		Status:  "ok",
		Mongo:   "ok",
		Version: Version,
	})
}
