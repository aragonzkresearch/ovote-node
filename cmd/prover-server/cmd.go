package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/groupoidlabs/ovote-node/types"
	flag "github.com/spf13/pflag"
	"go.vocdoni.io/dvote/db"
	"go.vocdoni.io/dvote/db/pebbledb"
	"go.vocdoni.io/dvote/log"
)

var port, dir string

type api struct {
	r *gin.Engine
	sync.Mutex

	lastID int
	db     db.Database
}

func main() {
	flag.StringVarP(&port, "port", "p", "9000", "network port for the HTTP API")
	flag.StringVarP(&dir, "dir", "d", "~/.proverserver", "db & files directory")
	flag.Parse()

	opts := db.Options{Path: dir}
	database, err := pebbledb.New(opts)
	if err != nil {
		log.Fatal(err)
	}

	a := api{}
	a.db = database
	a.r = gin.Default()
	a.lastID = 0

	a.r.GET("/status", a.getStatus)
	a.r.POST("/proof", a.genProof)
	a.r.GET("/proof/:id", a.getProof)
	a.r.GET("/proof/:id/public", a.getPublicInputs)

	err = a.r.Run(":" + port)
	if err != nil {
		log.Fatal(err)
	}
}

type errorMsg struct {
	Message string `json:"message"`
}

func returnErr(c *gin.Context, err error) {
	log.Warnw("HTTP API Bad request error", "err", err)
	c.JSON(http.StatusBadRequest, errorMsg{
		Message: err.Error(),
	})
}

func (a *api) getStatus(c *gin.Context) {
	if !a.isBusy() {
		c.JSON(http.StatusLocked, gin.H{
			"status": "prover busy",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func (a *api) genProof(c *gin.Context) {
	a.lastID++

	if !a.isBusy() {
		c.JSON(http.StatusLocked, gin.H{
			"status": "prover busy",
		})
	}

	// get zkinputs.json and store it in disk
	var zki types.ZKInputs
	if err := c.ShouldBindJSON(&zki); err != nil {
		returnErr(c, err)
		return
	}
	file, err := json.MarshalIndent(zki, "", " ")
	if err != nil {
		returnErr(c, err)
		return
	}
	err = ioutil.WriteFile("zkinputs"+strconv.Itoa(a.lastID)+".json",
		file, 0600)
	if err != nil {
		returnErr(c, err)
		return
	}

	go a.genWitnessAndProof(strconv.Itoa(a.lastID))

	// return the id, so the client knows which id to use to
	// retrieve the proof later
	c.JSON(http.StatusOK, gin.H{
		"id": a.lastID,
	})
}

func (a *api) getProof(c *gin.Context) {
	idStr := c.Param("id")
	c.File("proof" + idStr + ".json")
}

func (a *api) getPublicInputs(c *gin.Context) {
	idStr := c.Param("id")
	c.File("public" + idStr + ".json")
}
