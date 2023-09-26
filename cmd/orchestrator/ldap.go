/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package main

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/majewsky/portunus/internal/crypt"
	"github.com/sapcc/go-bits/logg"
)

// Notes on this configuration template:
//   - Only Portunus' own technical user has any sort of write access.
//   - The cn=portunus-viewers virtual group corresponds to Portunus' `LDAP.CanRead` permission.
//   - Users can read their own object, so that applications not using a service
//     user can discover group memberships of a logged-in user.
//   - TLSProtocolMin 3.3 means "TLS 1.2 or higher". (TODO select cipher suites according to recommendations)
//
// TODO when TLS is configured, also listen on ldap:///, but require StartTLS through `security minssf=256`.
var configTemplate = `
include %PORTUNUS_SLAPD_SCHEMA_DIR%/core.schema
include %PORTUNUS_SLAPD_SCHEMA_DIR%/cosine.schema
include %PORTUNUS_SLAPD_SCHEMA_DIR%/inetorgperson.schema
include %PORTUNUS_SLAPD_SCHEMA_DIR%/nis.schema

include %PORTUNUS_SLAPD_STATE_DIR%/portunus.schema

access to dn.base="" by * read
access to dn.base="cn=Subschema" by * read

access to *
	by dn.base="cn=portunus,%PORTUNUS_LDAP_SUFFIX%" write
	by group.exact="cn=portunus-viewers,%PORTUNUS_LDAP_SUFFIX%" read
	by self read
	by anonymous auth

TLSCACertificateFile  "%PORTUNUS_SLAPD_STATE_DIR%/ca.pem"
TLSCertificateFile    "%PORTUNUS_SLAPD_STATE_DIR%/cert.pem"
TLSCertificateKeyFile "%PORTUNUS_SLAPD_STATE_DIR%/key.pem"
TLSProtocolMin 3.3

database   mdb
maxsize    1073741824
suffix     "%PORTUNUS_LDAP_SUFFIX%"
rootdn     "cn=portunus,%PORTUNUS_LDAP_SUFFIX%"
rootpw     "%PORTUNUS_LDAP_PASSWORD_HASH%"
directory  "%PORTUNUS_SLAPD_STATE_DIR%/data"

index objectClass eq
`

// We do not use the OLC machinery for the memberOf attribute because
// portunus-server itself can do it much more easily. But that means we have to
// define the memberOf attribute on the schema level.
//
// Also, in order to work in as many scenarios as possible, we do not use the
// standard attribute name `memberOf`, but `isMemberOf` instead. (Some OpenLDAPs
// define the `memberOf` attribute even if you don't enable the memberof
// overlay.)
var customSchema = `
	attributetype ( 9999.1.1 NAME 'isMemberOf'
		DESC 'back-reference to groups this user is a member of'
		SUP distinguishedName )

	attributetype ( 9999.1.2 NAME 'sshPublicKey'
		DESC 'SSH public key used by this user'
		SUP name )

	objectclass ( 9999.2.1 NAME 'portunusPerson'
		DESC 'addon to objectClass person that adds Portunus-specific attributes'
		SUP top AUXILIARY
		MAY ( isMemberOf $ sshPublicKey ) )

`

//^ The trailing empty line is important, otherwise slapd cannot correctly
//parse this file. ikr?

func renderSlapdConfig(environment map[string]string, hasher crypt.PasswordHasher) []byte {
	password := generateServiceUserPassword()
	logg.Debug("password for cn=portunus,%s is %s",
		environment["PORTUNUS_LDAP_SUFFIX"], password)
	environment["PORTUNUS_LDAP_PASSWORD"] = password
	environment["PORTUNUS_LDAP_PASSWORD_HASH"] = hasher.HashPassword(password)

	config := configTemplate
	if environment["PORTUNUS_SLAPD_TLS_CERTIFICATE"] == "" {
		config = regexp.MustCompile(`(?m)^TLS.*$`).ReplaceAllString(config, "")
	}

	config = regexp.MustCompile(`%\w+%`).
		ReplaceAllStringFunc(config, func(match string) string {
			match = strings.TrimPrefix(match, "%")
			match = strings.TrimSuffix(match, "%")
			return environment[match]
		})

	return []byte(config)
}

func generateServiceUserPassword() string {
	buf := make([]byte, 32)
	_, err := rand.Read(buf[:])
	if err != nil {
		logg.Fatal(err.Error())
	}
	return hex.EncodeToString(buf[:])
}

// Does not return. Call with `go`.
func runLDAPServer(environment map[string]string) {
	debugLogFlags := uint64(0)
	if logg.ShowDebug {
		//with PORTUNUS_DEBUG=true, turn on all debug logging except for package
		//traces (those might reveal user passwords in the logfile when bind
		//requests are logged)
		debugLogFlags = 0xFFFF &^ 0x12
	}

	bindURL := "ldap:///"
	if environment["PORTUNUS_SLAPD_TLS_CERTIFICATE"] != "" {
		bindURL = "ldaps:///"
	}

	logg.Info("starting LDAP server")
	//run slapd
	cmd := exec.Command(environment["PORTUNUS_SLAPD_BINARY"],
		"-u", environment["PORTUNUS_SLAPD_USER"],
		"-g", environment["PORTUNUS_SLAPD_GROUP"],
		"-h", bindURL,
		"-f", filepath.Join(environment["PORTUNUS_SLAPD_STATE_DIR"], "slapd.conf"),
		//even for debugLogFlags == 0, giving `-d` is still important because its
		//presence keeps slapd from daemonizing)
		"-d", strconv.FormatUint(debugLogFlags, 10),
	)
	cmd.Stdin = nil
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		logg.Error("error encountered while running slapd: " + err.Error())
		logg.Info("Since slapd logs to syslog only, check there for more information.")
		os.Exit(1)
	}
}
