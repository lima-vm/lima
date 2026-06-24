// SPDX-FileCopyrightText: Copyright The Lima Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/lima-vm/lima/v2/pkg/hostagent/api"
	"github.com/lima-vm/lima/v2/pkg/limatype"
)

type fakeAgent struct {
	added   *api.MountRequest
	removed string
	list    []api.Mount
}

func (f *fakeAgent) Info(context.Context) (*api.Info, error) { return &api.Info{}, nil }

func (f *fakeAgent) MountAdd(_ context.Context, hostPath, guestPath string, mountType limatype.MountType, writable bool) (*api.Mount, error) {
	f.added = &api.MountRequest{HostPath: hostPath, MountPoint: guestPath, Type: string(mountType), Writable: writable}
	return &api.Mount{ID: guestPath, HostPath: hostPath, MountPoint: guestPath, Type: string(mountType), Writable: writable}, nil
}

func (f *fakeAgent) MountRemove(_ context.Context, guestPath string) error {
	f.removed = guestPath
	return nil
}

func (f *fakeAgent) MountList() []api.Mount { return f.list }

func newTestServer(a Agent) *httptest.Server {
	mux := http.NewServeMux()
	AddRoutes(mux, &Backend{Agent: a})
	return httptest.NewServer(mux)
}

func TestMountsPost(t *testing.T) {
	fake := &fakeAgent{}
	srv := newTestServer(fake)
	defer srv.Close()

	body := `{"hostPath":"/h/code","mountPoint":"/mnt/code","type":"virtiofs","writable":true}`
	resp, err := http.Post(srv.URL+"/v1/mounts", "application/json", strings.NewReader(body))
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	assert.Assert(t, fake.added != nil)
	assert.Equal(t, fake.added.MountPoint, "/mnt/code")
	assert.Equal(t, fake.added.Type, "virtiofs")
	assert.Equal(t, fake.added.Writable, true)

	var m api.Mount
	assert.NilError(t, json.NewDecoder(resp.Body).Decode(&m))
	assert.Equal(t, m.MountPoint, "/mnt/code")
}

func TestMountsGet(t *testing.T) {
	fake := &fakeAgent{list: []api.Mount{{ID: "/mnt/a", MountPoint: "/mnt/a", Type: "9p"}}}
	srv := newTestServer(fake)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/mounts")
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	var list []api.Mount
	assert.NilError(t, json.NewDecoder(resp.Body).Decode(&list))
	assert.Equal(t, len(list), 1)
	assert.Equal(t, list[0].MountPoint, "/mnt/a")
}

func TestMountsDelete(t *testing.T) {
	fake := &fakeAgent{}
	srv := newTestServer(fake)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/v1/mounts", strings.NewReader(`{"mountPoint":"/mnt/code"}`))
	assert.NilError(t, err)
	resp, err := http.DefaultClient.Do(req)
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusNoContent)
	assert.Equal(t, fake.removed, "/mnt/code")
}
