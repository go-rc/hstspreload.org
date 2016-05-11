package database

import (
	"time"

	"golang.org/x/net/context"

	"github.com/chromium/hstspreload.appspot.com/database/gcd"
	"google.golang.org/cloud/datastore"
)

const (
	// A blank project ID forces the project ID to be read from
	// the DATASTORE_PROJECT_ID environment variable.
	batchSize = 450
	timeout   = 10 * time.Second

	domainStateKind = "DomainState"
)

// PutStates updates the given domain updates in batches.
// Writes and flushes updates to w.
func PutStates(db gcd.Backend, updates []DomainState, statusReport func(format string, args ...interface{})) error {
	if len(updates) == 0 {
		statusReport("No updates.\n")
		return nil
	}

	// Set up the datastore context.
	c, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, datastoreErr := db.NewClient(c)
	if datastoreErr != nil {
		return datastoreErr
	}

	putMulti := func(keys []*datastore.Key, values []DomainState) error {
		statusReport("Updating %d entries...", len(keys))

		if _, err := client.PutMulti(c, keys, values); err != nil {
			statusReport(" failed.\n")
			return err
		}

		statusReport(" done.\n")
		return nil
	}

	var keys []*datastore.Key
	var values []DomainState
	for _, state := range updates {
		key := datastore.NewKey(c, domainStateKind, string(state.Name), 0, nil)
		keys = append(keys, key)
		values = append(values, state)

		if len(keys) >= batchSize {
			if err := putMulti(keys, values); err != nil {
				return err
			}
			keys = keys[:0]
			values = values[:0]
		}
	}

	if err := putMulti(keys, values); err != nil {
		return err
	}
	return nil
}

// PutState is a convenience version of PutStates for a single domain.
func PutState(db gcd.Backend, update DomainState) error {
	ignoreStatus := func(format string, args ...interface{}) {}
	return PutStates(db, []DomainState{update}, ignoreStatus)
}

// StatesForQuery returns ahe states for the given datastore query.
func StatesForQuery(db gcd.Backend, query *datastore.Query) (states []DomainState, err error) {
	// Set up the datastore context.
	c, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, datastoreErr := db.NewClient(c)
	if datastoreErr != nil {
		return states, datastoreErr
	}

	keys, err := client.GetAll(c, query, &states)
	if err != nil {
		return states, err
	}

	for i, key := range keys {
		state := states[i]
		state.Name = key.Name()
		states[i] = state
	}

	return states, nil
}

// DomainsForQuery returns the domains that match the given datastore query.
func DomainsForQuery(db gcd.Backend, query *datastore.Query) (domains []string, err error) {
	// Set up the datastore context.
	c, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, datastoreErr := db.NewClient(c)
	if datastoreErr != nil {
		return domains, datastoreErr
	}

	keys, err := client.GetAll(c, query.KeysOnly(), nil)
	if err != nil {
		return domains, err
	}

	for _, key := range keys {
		domain := key.Name()
		domains = append(domains, domain)
	}

	return domains, nil
}

// StateForDomain get the state for the given domain.
// Note that the Name field of `state` will not be set.
func StateForDomain(db gcd.Backend, domain string) (state DomainState, err error) {
	// Set up the datastore context.
	c, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client, datastoreErr := db.NewClient(c)
	if datastoreErr != nil {
		return state, datastoreErr
	}

	key := datastore.NewKey(c, domainStateKind, string(domain), 0, nil)
	getErr := client.Get(c, key, &state)
	if getErr != nil {
		if getErr == datastore.ErrNoSuchEntity {
			return DomainState{Status: StatusUnknown}, nil
		}
		return state, getErr
	}

	return state, nil
}

// AllDomainStates gets the states of all domains in the database.
func AllDomainStates(db gcd.Backend) (states []DomainState, err error) {
	return StatesForQuery(db, datastore.NewQuery("DomainState"))
}

// DomainsWithStatus returns the domains with the given status in the database.
func DomainsWithStatus(db gcd.Backend, status PreloadStatus) (domains []string, err error) {
	return DomainsForQuery(db, datastore.NewQuery("DomainState").Filter("Status =", string(status)))
}
