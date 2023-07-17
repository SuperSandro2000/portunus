/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/majewsky/portunus/internal/core"
	"github.com/majewsky/portunus/internal/frontend"
	"github.com/majewsky/portunus/internal/ldap"
	_ "github.com/majewsky/xyrillian.css"
	"github.com/sapcc/go-bits/logg"
	"github.com/sapcc/go-bits/must"
	"github.com/sapcc/go-bits/osext"
)

func main() {
	logg.ShowDebug = os.Getenv("PORTUNUS_DEBUG") == "true"
	dropPrivileges()

	seed, err := core.ReadDatabaseSeedFromEnvironment()
	if err != nil {
		logg.Fatal("while reading PORTUNUS_SEED_PATH: " + err.Error())
	}

	ctx := context.TODO()
	nexus := core.NewNexus()

	fs := core.FileStore{
		Path:        filepath.Join(os.Getenv("PORTUNUS_SERVER_STATE_DIR"), "database.json"),
		Initializer: core.DatabaseInitializer(seed),
	}
	fsAPI := fs.RunAsync()

	ldapConn := must.Return(ldap.Connect(ldap.ConnectionOptions{
		DNSuffix:      osext.MustGetenv("PORTUNUS_LDAP_SUFFIX"),
		Password:      osext.MustGetenv("PORTUNUS_LDAP_PASSWORD"),
		TLSDomainName: os.Getenv("PORTUNUS_SLAPD_TLS_DOMAIN_NAME"),
	}))
	ldapAdapter := ldap.NewAdapter(nexus, ldapConn)
	go func() {
		must.Succeed(ldapAdapter.Run(ctx))
	}()

	engine := core.RunEngineAsync(fsAPI, nexus, seed)

	handler := frontend.HTTPHandler(engine, os.Getenv("PORTUNUS_SERVER_HTTP_SECURE") == "true")
	logg.Fatal(http.ListenAndServe(os.Getenv("PORTUNUS_SERVER_HTTP_LISTEN"), handler).Error())
}

func dropPrivileges() {
	gidParsed, err := strconv.ParseUint(os.Getenv("PORTUNUS_SERVER_GID"), 10, 32)
	if err != nil {
		logg.Fatal("cannot parse PORTUNUS_SERVER_GID: " + err.Error())
	}
	gid := int(gidParsed)
	err = syscall.Setresgid(gid, gid, gid)
	if err != nil {
		logg.Fatal("change GID failed: " + err.Error())
	}

	uidParsed, err := strconv.ParseUint(os.Getenv("PORTUNUS_SERVER_UID"), 10, 32)
	if err != nil {
		logg.Fatal("cannot parse PORTUNUS_SERVER_UID: " + err.Error())
	}
	uid := int(uidParsed)
	err = syscall.Setresuid(uid, uid, uid)
	if err != nil {
		logg.Fatal("change UID failed: " + err.Error())
	}
}
