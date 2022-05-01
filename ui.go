package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (w Watcher) Home(c *gin.Context) {
	c.HTML(http.StatusOK, "home", gin.H{})
}
