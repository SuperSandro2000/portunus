/*******************************************************************************
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/crypto/ssh"
)

// FieldRef identifies a field within a User or Group. It appears in type ValidationError.
type FieldRef struct {
	ObjectType string //either "user" or "group"
	ObjectName string //the LoginName for users or the Name for groups
	FieldName  string //e.g. "surname" or "posix_gid", matches the input element name in the respective HTML forms
}

// ValidationError is a structured error type that describes an unacceptable
// field value in a User or Group. The generic type is either User or Group.
type ValidationError struct {
	FieldRef   FieldRef
	FieldError error //sentence without subject, e.g. "may not be missing"
}

// Wrap converts an error into a ValidationError.
func (r FieldRef) Wrap(err error) error {
	if err == nil {
		return nil
	}
	return ValidationError{r, err}
}

// WrapFirst converts the first non-nil error from the given list into a
// ValidationError. The intended use of this is to have a sequence of
// increasingly strict checks, and show only the error from the broadest check.
func (r FieldRef) WrapFirst(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return ValidationError{r, err}
		}
	}
	return nil
}

// Error implements the builtin/error interface.
func (e ValidationError) Error() string {
	r := e.FieldRef
	return fmt.Sprintf("field %q in %s %q %s",
		r.FieldName, r.ObjectType, r.ObjectName, e.FieldError.Error())
}

// this regexp copied from useradd(8) manpage
const posixAccountNamePattern = `[a-z_][a-z0-9_-]*\$?`

var (
	errIsDuplicate       = errors.New("is already in use")
	errIsDuplicateInSeed = errors.New("is defined multiple times")
	errIsMissing         = errors.New("is missing")
	errLeadingSpaces     = errors.New("may not start with a space character")
	errTrailingSpaces    = errors.New("may not end with a space character")

	errNotPosixAccountName = errors.New("is not an acceptable user/group name matching the pattern /" + posixAccountNamePattern + "/")
	posixAccountNameRx     = regexp.MustCompile(`^` + posixAccountNamePattern + `$`)
	errNotPosixUIDorGID    = errors.New("is not a number between 0 and 65535 inclusive")

	errNotAbsolutePath = errors.New("must be an absolute path, i.e. start with a /")
)

// MustNotBeEmpty is a h.ValidationRule.
func MustNotBeEmpty(val string) error {
	if strings.TrimSpace(val) == "" {
		return errIsMissing
	}
	return nil
}

// MustNotHaveSurroundingSpaces is a h.ValidationRule.
func MustNotHaveSurroundingSpaces(val string) error {
	if val != "" {
		if strings.TrimLeftFunc(val, unicode.IsSpace) != val {
			return errLeadingSpaces
		}
		if strings.TrimRightFunc(val, unicode.IsSpace) != val {
			return errTrailingSpaces
		}
	}
	return nil
}

// MustBePosixAccountName is a h.ValidationRule.
func MustBePosixAccountName(val string) error {
	if posixAccountNameRx.MatchString(val) {
		return nil
	}
	return errNotPosixAccountName
}

// MustBePosixUIDorGID is a h.ValidationRule.
func MustBePosixUIDorGID(val string) error {
	if val != "" {
		_, err := strconv.ParseUint(val, 10, 16)
		if err != nil {
			return errNotPosixUIDorGID
		}
	}
	return nil
}

// MustBeAbsolutePath is a h.ValidationRule.
func MustBeAbsolutePath(val string) error {
	if val != "" && !strings.HasPrefix(val, "/") {
		return errNotAbsolutePath
	}
	return nil
}

// SplitSSHPublicKeys preprocesses the content of a submitted <textarea> where a
// list of SSH public keys is expected. The result will have one public key per
// array entry.
func SplitSSHPublicKeys(val string) (result []string) {
	for _, line := range strings.Split(val, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// MustBeSSHPublicKeys is a h.ValidationRule.
func MustBeSSHPublicKeys(val string) error {
	for idx, line := range SplitSSHPublicKeys(val) {
		_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			return fmt.Errorf("must have a valid SSH public key on each line (parse error on line %d)", idx+1)
		}
	}
	return nil
}

// MustBeSSHPublicKey is a h.ValidationRule.
func MustBeSSHPublicKey(val string) error {
	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(val))
	if err != nil {
		return errors.New("must be a valid SSH public key")
	}
	return nil
}
