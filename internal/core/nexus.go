/*******************************************************************************
* Copyright 2023 Stefan Majewsky <majewsky@gmx.net>
* SPDX-License-Identifier: GPL-3.0-only
* Refer to the file "LICENSE" for details.
*******************************************************************************/

package core

import (
	"context"
	"reflect"
	"sync"

	"github.com/sapcc/go-bits/errext"
)

// Reducer is an action that modifies the contents of a Database. This type
// appears in the Nexus.Update() interface method.
type Reducer func(Database) (Database, error)

// Nexus stores the contents of the Database. All other parts of the
// application use a reference to the Nexus to read and update the Database.
type Nexus interface {
	// AddListener registers a listener with the nexus. Whenever the database
	// changes, the callback will be invoked to provide a copy of the database to
	// the listener. The listener will be removed from the nexus when `ctx`
	// expires.
	//
	// Note that the callback is invoked from whatever goroutine is causing the
	// DB to be updated. If a specific goroutine shall process the event, the
	// callback should send into a channel from which that goroutine is receiving.
	AddListener(ctx context.Context, callback func(Database))

	// Update changes the contents of the database. This interface follows the
	// State Reducer pattern: The reducer callback is invoked with the current
	// Database, and is expected to return the updated Database. The updated
	// Database is then validated and the database seed is enforced, if any.
	Update(reducer Reducer, opts *UpdateOptions) errext.ErrorSet
}

// UpdateOptions controls optional behavior in Nexus.Update().
type UpdateOptions struct {
	//If true, conflicts with the seed will be reported as validation errors.
	//If false (default), conflicts with the seed will be corrected silently.
	ConflictWithSeedIsError bool
}

// NewNexus instantiates the Nexus.
func NewNexus(d *DatabaseSeed) Nexus {
	return &nexusImpl{seed: d}
}

type nexusImpl struct {
	//The mutex guards access to all other fields in this struct.
	mutex     sync.Mutex
	seed      *DatabaseSeed
	db        Database
	listeners []listener
}

type listener struct {
	ctx      context.Context
	callback func(Database)
}

// AddListener implements the Nexus interface.
func (n *nexusImpl) AddListener(ctx context.Context, callback func(Database)) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	n.listeners = append(n.listeners, listener{ctx, callback})

	//if the DB has already been filled before AddListener(), tell the listener
	//about the current DB contents right away
	if !n.db.IsEmpty() && ctx.Err() == nil {
		callback(n.db.Cloned())
	}
}

// Update implements the Nexus interface.
func (n *nexusImpl) Update(reducer Reducer, optsPtr *UpdateOptions) (errs errext.ErrorSet) {
	var opts UpdateOptions
	if optsPtr != nil {
		opts = *optsPtr
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	//compute new DB by applying the reducer to a clone of the old DB
	newDB, err := reducer(n.db.Cloned())
	if err != nil {
		errs.Add(err)
		return
	}
	newDB.Normalize()

	//TODO: perform validation of new state, use ErrorSet to return detailed validation errors
	//enforce Seed
	if n.seed != nil {
		if opts.ConflictWithSeedIsError {
			errs = n.seed.CheckConflicts(newDB)
		} else {
			n.seed.ApplyTo(&newDB)
		}
	}

	//new DB looks good -> store it and inform our listeners *if* it actually
	//represents a change
	if reflect.DeepEqual(n.db, newDB) {
		//This check is important to prevent infinite loops like this:
		//DB update -> disk write -> fsnotify -> disk read -> DB update
		return nil
	}
	n.db = newDB
	for _, listener := range n.listeners {
		if listener.ctx.Err() == nil {
			listener.callback(n.db.Cloned())
		}
	}
	return nil
}
