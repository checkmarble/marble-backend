package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/checkmarble/marble-backend/usecases"
)

func NewServer(
	router *gin.Engine,
	port string,
	marbleAppHost string,
	uc usecases.Usecases,
	auth Authentication,
	tokenHandler TokenHandler,
	localTest ...bool,
) *http.Server {
	addRoutes(router, auth, tokenHandler, uc, marbleAppHost)

	var host string
	if len(localTest) > 0 && localTest[0] {
		host = "localhost"
	} else {
		host = "0.0.0.0"
	}

	return &http.Server{
		Addr:         fmt.Sprintf("%s:%s", host, port),
		WriteTimeout: time.Second * 60,
		ReadTimeout:  time.Second * 60,
		IdleTimeout:  time.Second * 60,
		Handler:      h2c.NewHandler(router, &http2.Server{}),
	}
}
