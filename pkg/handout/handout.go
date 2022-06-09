package handout

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HandOut handout
type HandOut interface {
	Do(param *Param) error
}

// Param params
type Param struct {
	Code string `json:"code"`
}

// GINHandOut gin hand out func
func GINHandOut(h HandOut) gin.HandlerFunc {
	return func(c *gin.Context) {
		param := new(Param)
		err := c.ShouldBindJSON(param)
		if err != nil {
			param = nil
		}
		err = h.Do(param)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
}
