/*******************************************************************************
*
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
*
* This program is free software: you can redistribute it and/or modify it under
* the terms of the GNU General Public License as published by the Free Software
* Foundation, either version 3 of the License, or (at your option) any later
* version.
*
* This program is distributed in the hope that it will be useful, but WITHOUT ANY
* WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
* A PARTICULAR PURPOSE. See the GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License along with
* this program. If not, see <http://www.gnu.org/licenses/>.
*
*******************************************************************************/

package main

import (
	"net/http"
	"os"
	"strconv"
	"syscall"

	"github.com/majewsky/portunus/internal/core"
	"github.com/majewsky/portunus/internal/frontend"
	"github.com/sapcc/go-bits/logg"
)

func main() {
	logg.ShowDebug = true //TODO make configurable
	dropPrivileges()
	ldapWorker := newLDAPWorker()

	go ldapWorker.processEvents(mockEventsChan())

	//TODO make HTTP listen address configurable
	logg.Fatal(http.ListenAndServe(":8080", frontend.HTTPHandler()).Error())
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

func mockEventsChan() <-chan core.Event {
	channel := make(chan core.Event, 1)
	channel <- core.Event{
		Added: []core.Entity{
			core.User{
				LoginName:    "john",
				GivenName:    "John",
				FamilyName:   "Doe",
				PasswordHash: core.HashPasswordForLDAP("12345"),
			},
			core.User{
				LoginName:    "jane",
				GivenName:    "Jane",
				FamilyName:   "Doe",
				PasswordHash: core.HashPasswordForLDAP("password"),
			},
			core.Group{
				Name:             "admins",
				Description:      "system administrators",
				MemberLoginNames: []string{"jane"},
				Permissions: core.Permissions{
					LDAP: core.LDAPAccessFullRead,
				},
			},
			core.Group{
				Name:             "users",
				Description:      "contains everyone",
				MemberLoginNames: []string{"jane", "john"},
				Permissions: core.Permissions{
					LDAP: core.LDAPAccessNone,
				},
			},
		},
	}
	return channel
}
