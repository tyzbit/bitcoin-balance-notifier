package main

import (
	"html/template"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func InitFrontend(r *gin.Engine) {
	templateSubPath, err := fs.Sub(web, "web/templates/pages")
	if err != nil {
		log.Panic(err)
	}
	t, err := template.ParseFS(templateSubPath, "*.html")
	if err != nil {
		log.Panic(err)
	}
	r.SetHTMLTemplate(t)
	staticSubPath, err := fs.Sub(web, "web/static")
	if err != nil {
		log.Panic(err)
	}
	r.StaticFS("/static", http.FS(staticSubPath))
	r.GET("/", watcher.Home)
}

func InitBackend(r *gin.Engine) {
	r.POST("/balance", watcher.GetBalance)
	r.POST("/watch", watcher.AddWatch)
	r.GET("/balances", watcher.GetBalances)
	r.GET("/watches", watcher.GetWatches)
	r.DELETE("/identifier", watcher.DeleteIdentifier)
}