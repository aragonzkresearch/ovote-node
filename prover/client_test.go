package prover

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aragonzkresearch/ovote-node/types"
	qt "github.com/frankban/quicktest"
	"github.com/gin-gonic/gin"
)

func TestGenProof(t *testing.T) {
	c := qt.New(t)

	r := gin.Default()
	r.POST("/proof", mockGenProof)

	ts := httptest.NewServer(r)
	defer ts.Close()

	p := NewClient(ts.URL)
	zki := types.NewZKInputs(2, 2)
	pID, err := p.GenProof(1, zki)
	c.Assert(err, qt.IsNil)
	c.Assert(pID, qt.Equals, uint64(42))

	// now with handler that returns error
	r = gin.Default()
	r.POST("/proof", mockGetErr)
	ts = httptest.NewServer(r)
	defer ts.Close()

	p = NewClient(ts.URL)
	_, err = p.GenProof(1, zki)
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Equals, "expected error msg")
}

func TestGetProof(t *testing.T) {
	c := qt.New(t)

	// prepare the test files of proof & publicInputs
	err := ioutil.WriteFile("proofTest.json", []byte("proof-content"), 0600)
	c.Assert(err, qt.IsNil)
	err = ioutil.WriteFile("publicinputsTest.json", []byte("publicinputs-content"), 0600)
	c.Assert(err, qt.IsNil)
	defer os.Remove("proofTest.json")        //nolint:errcheck
	defer os.Remove("publicinputsTest.json") //nolint:errcheck

	r := gin.Default()
	r.GET("/proof/:proofID", mockGetProof)
	r.GET("/proof/:proofID/public", mockGetPublicInputs)

	ts := httptest.NewServer(r)
	defer ts.Close()

	p := NewClient(ts.URL)
	obtainedProof, obtainedPublicInputs, err := p.GetProof(1)
	c.Assert(err, qt.IsNil)
	c.Assert(obtainedProof, qt.DeepEquals, []byte("proof-content"))
	c.Assert(obtainedPublicInputs, qt.DeepEquals, []byte("publicinputs-content"))

	// now with handler that returns error
	r = gin.Default()
	r.GET("/proof/:proofID", mockGetErr)
	ts = httptest.NewServer(r)
	defer ts.Close()

	p = NewClient(ts.URL)
	_, _, err = p.GetProof(1)
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Equals, "expected error msg")
}

func mockGenProof(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"id": 42,
	})
}

func mockGetErr(c *gin.Context) {
	c.JSON(http.StatusBadRequest, errorMsg{
		Message: "expected error msg",
	})
}

func mockGetProof(c *gin.Context) {
	c.File("proofTest.json")
}

func mockGetPublicInputs(c *gin.Context) {
	c.File("publicinputsTest.json")
}
