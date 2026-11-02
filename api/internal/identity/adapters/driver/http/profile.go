package http

import "github.com/gin-gonic/gin"

// ProfileHandler is implemented by the application profile query boundary.
// Identity owns only the stable route; it does not import or orchestrate
// Cultivation and Problem projections.
type ProfileHandler interface {
	GetProfile(c *gin.Context)
}
