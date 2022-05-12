// Package testutils
//
// Useful documentation for understanding what's happening
// Also see vendor/github.com/go-git/go-git/v5/plumbing/transport/server/server.go
//
// https://github.com/git/git/blob/master/Documentation/technical/http-protocol.txt
// https://github.com/git/git/blob/master/Documentation/technical/pack-protocol.txt
// https://github.com/git/git/blob/master/Documentation/technical/protocol-common.txt
package testutils

import (
	"context"
	"fmt"
	http2 "github.com/go-git/go-git/v5/plumbing/transport/http"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/capability"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/stretchr/testify/require"
)

const (
	timeout = 5 * time.Minute
)

type FakeGit struct {
	t       *testing.T
	gitRepo *git.Repository
	auth    *http2.BasicAuth

	session   transport.UploadPackSession
	rpSession transport.ReceivePackSession
}

func NewFakeGitHttp(t *testing.T, gitRepo *git.Repository, auth *http2.BasicAuth) *FakeGit {
	g := &FakeGit{
		t:       t,
		gitRepo: gitRepo,
		auth:    auth,
	}

	g.setupPackSessions()
	return g
}

func (g *FakeGit) SetupRoutes(r *gin.Engine, owner, repo string) {
	r.GET(g.endpointURL(owner, repo, "/info/refs"), g.getInfoRefs)
	r.POST(g.endpointURL(owner, repo, "/git-upload-pack"), g.getUploadPack)
	r.POST(g.endpointURL(owner, repo, "/git-receive-pack"), g.getReceivePack)
	// unused handlers at the moment
	// we only require the above for a fresh clone
	// we leave the following handlers to make debugging easier if the go-git client starts querying new data
	r.GET(g.endpointURL(owner, repo, "/HEAD"), g.getHead)
	r.GET(g.endpointURL(owner, repo, "/objects/info/alternates"), g.getTextFile)
	r.GET(g.endpointURL(owner, repo, "/objects/info/http-alternates"), g.getTextFile)
	r.GET(g.endpointURL(owner, repo, "/objects/info/packs"), g.getInfoPacks)
	r.GET(g.endpointURL(owner, repo, "/objects/:dir/:file"), g.getLooseObject)
	r.GET(g.endpointURL(owner, repo, "/objects/pack/:pack"), g.getPackFile)
}

func (g *FakeGit) setupPackSessions() {
	// We leverage the go-git server implementation for handling an upload pack session
	srv := server.NewServer(g)
	sess, err := srv.NewUploadPackSession(nil, nil)
	require.NoError(g.t, err)
	g.session = sess

	rpSess, err := srv.NewReceivePackSession(nil, nil)
	require.NoError(g.t, err)
	g.rpSession = rpSess
}

func (g *FakeGit) endpointURL(owner, repo, path string) string {
	if owner == "" || repo == "" {
		return path
	}

	return fmt.Sprintf("/%s/%s.git/%s", owner, repo, strings.TrimPrefix(path, "/"))
}

// {"GET", "/info/refs$", get_info_refs}
func (g *FakeGit) getInfoRefs(c *gin.Context) {
	if err := g.validateCredentials(c.Request); err != nil {
		c.Writer.WriteHeader(http.StatusForbidden)
		return
	}

	name := c.Query("service")
	// see Smart Server Response section
	if name != transport.UploadPackServiceName && name != transport.ReceivePackServiceName {
		c.Writer.WriteHeader(http.StatusForbidden)
		return
	}
	c.Header("Content-Type", fmt.Sprintf("application/x-%s-advertisement", transport.UploadPackServiceName))
	c.Header("Cache-Control", "no-cache")
	c.Writer.WriteHeader(http.StatusOK)

	// can we not use vendor/github.com/go-git/go-git/v5/plumbing/transport/server/server.go somehow?
	ar := packp.NewAdvRefs()
	iter, err := g.gitRepo.References()
	require.NoError(g.t, err)
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}

		ar.References[ref.Name().String()] = ref.Hash()
		return nil
	})
	require.NoError(g.t, err)

	ref, err := g.gitRepo.Reference(plumbing.HEAD, true)
	require.NoError(g.t, err)
	require.Equal(g.t, ref.Type(), plumbing.HashReference)
	h := ref.Hash()
	ar.Head = &h

	err = ar.Encode(c.Writer)
	require.NoError(g.t, err)
}

//{"POST", "/git-upload-pack$", service_rpc},
func (g *FakeGit) getUploadPack(c *gin.Context) {
	if err := g.validateCredentials(c.Request); err != nil {
		c.Writer.WriteHeader(http.StatusForbidden)
	}

	if c.GetHeader("Content-Type") != fmt.Sprintf("application/x-%s-request", transport.UploadPackServiceName) {
		c.Writer.WriteHeader(http.StatusBadRequest)
		return
	}

	c.Header("Content-Type", fmt.Sprintf("application/x-%s-result", transport.UploadPackServiceName))
	c.Header("Cache-Control", "no-cache")
	c.Writer.WriteHeader(http.StatusOK)

	packReq := &packp.UploadPackRequest{}
	err := packReq.Decode(c.Request.Body)
	require.NoError(g.t, err)

	// other-wise validate will fail
	if packReq.Capabilities == nil {
		packReq.Capabilities = capability.NewList()
	}
	if packReq.Depth == nil {
		packReq.Depth = packp.DepthCommits(0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := g.session.UploadPack(ctx, packReq)
	require.NoError(g.t, err)

	err = resp.Encode(c.Writer)
	require.NoError(g.t, err)
}

//{"POST", "/git-receive-pack$", service_rpc}
func (g *FakeGit) getReceivePack(c *gin.Context) {
	if err := g.validateCredentials(c.Request); err != nil {
		c.Writer.WriteHeader(http.StatusForbidden)
	}

	if c.GetHeader("Content-Type") != fmt.Sprintf("application/x-%s-request", transport.ReceivePackServiceName) {
		header := c.GetHeader("Content-Type")
		_ = header
		c.Writer.WriteHeader(http.StatusBadRequest)
		return
	}

	c.Header("Content-Type", fmt.Sprintf("application/x-%s-advertisement", transport.ReceivePackServiceName))
	c.Header("Cache-Control", "no-cache")
	c.Writer.WriteHeader(http.StatusOK)

	req := packp.NewReferenceUpdateRequest()
	err := req.Capabilities.Add(capability.ReportStatus)
	require.NoError(g.t, err)

	err = req.Decode(c.Request.Body)
	require.NoError(g.t, err)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := g.rpSession.ReceivePack(ctx, req)
	require.NoError(g.t, err)

	err = resp.Encode(c.Writer)
	require.NoError(g.t, err)
}

func (g *FakeGit) Load(*transport.Endpoint) (storer.Storer, error) {
	if g.gitRepo == nil {
		return nil, transport.ErrRepositoryNotFound
	}

	return g.gitRepo.Storer, nil
}

// {"GET", "/HEAD$", get_head}
func (g *FakeGit) getHead(*gin.Context) {
	g.t.Fatalf("getHead not implemented")
}

// {"GET", "/objects/info/http-alternates$", get_text_file}
// {"GET", "/objects/info/alternates$", get_text_file}
func (g *FakeGit) getTextFile(*gin.Context) {
	g.t.Fatalf("getTextFile not implemented")

}

// {"GET", "/objects/info/packs$", get_info_packs}
func (g *FakeGit) getInfoPacks(*gin.Context) {
	g.t.Fatalf("getInfoPacks not implemented")
}

//{"GET", "/objects/[0-9a-f]{2}/[0-9a-f]{38}$", get_loose_object},
func (g *FakeGit) getLooseObject(*gin.Context) {
	g.t.Fatalf("getLooseObject not implemented")
}

//{"GET", "/objects/pack/pack-[0-9a-f]{40}\\.pack$", get_pack_file},
//{"GET", "/objects/pack/pack-[0-9a-f]{64}\\.pack$", get_pack_file},
func (g *FakeGit) getPackFile(*gin.Context) {
	g.t.Fatalf("getPackFile not implemented")
}

func (g *FakeGit) validateCredentials(r *http.Request) error {
	username, password, ok := r.BasicAuth()
	if !ok {
		return fmt.Errorf("basic auth not set")
	}

	if g.auth.Username != username {
		return fmt.Errorf("invalid credentials")
	}

	if g.auth.Password != password {
		return fmt.Errorf("invalid credentials")
	}

	return nil
}
