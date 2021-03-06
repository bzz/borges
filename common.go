package borges

import (
	stderrors "errors"
	"io"

	"github.com/satori/go.uuid"
	"gopkg.in/src-d/core-retrieval.v0/model"
	"gopkg.in/src-d/go-errors.v0"
	"gopkg.in/src-d/go-kallax.v1"
)

var (
	// ErrAlreadyStopped signals that an operation cannot be done because
	// the entity is already sopped.
	ErrAlreadyStopped = errors.NewKind("already stopped: %s")

	ErrWaitForJobs = errors.NewKind("no more jobs at the moment")

	ErrReferencedObjectTypeNotSupported error = stderrors.New("referenced object type not supported")
)

// Job represents a borges job to fetch and archive a repository.
type Job struct {
	RepositoryID uuid.UUID
}

// JobIter is an iterator of Job.
type JobIter interface {
	io.Closer
	// Next returns the next job. It returns io.EOF if there are no more
	// jobs. If there are no more jobs at the moment, but there can be
	// in the future, it returns an error of kind ErrWaitForJobs.
	Next() (*Job, error)
}

// RepositoryID tries to find a repository by the endpoint into the database.
// If no repository is found, it creates a new one and returns the ID.
func RepositoryID(endpoints []string, isFork *bool, storer *model.RepositoryStore) (uuid.UUID, error) {
	q := make([]interface{}, len(endpoints))
	for _, ep := range endpoints {
		q = append(q, ep)
	}

	rs, err := storer.Find(
		model.NewRepositoryQuery().
			Where(kallax.And(kallax.ArrayOverlap(
				model.Schema.Repository.Endpoints, q...,
			))),
	)
	if err != nil {
		return uuid.Nil, err
	}

	repositories, err := rs.All()
	if err != nil {
		return uuid.Nil, err
	}

	l := len(repositories)
	switch {
	case l == 0:
		r := model.NewRepository()
		r.Endpoints = endpoints
		r.IsFork = isFork
		if _, err := storer.Save(r); err != nil {
			return uuid.Nil, err
		}

		return uuid.UUID(r.ID), nil
	case l > 1:
		// TODO log error printing the ids and the endpoint
	}

	r := repositories[0]

	// check if the existing repository has all the aliases
	allEndpoints, update := getUniqueEndpoints(r.Endpoints, endpoints)

	if update {
		sf := []kallax.SchemaField{model.Schema.Repository.Endpoints}

		r.Endpoints = allEndpoints
		if _, err := storer.Update(r, sf...); err != nil {
			return uuid.Nil, err
		}
	}

	return uuid.UUID(repositories[0].ID), nil
}

func getUniqueEndpoints(re, ne []string) ([]string, bool) {
	actualSet := make(map[string]bool)
	outputSet := make(map[string]bool)

	for _, e := range re {
		actualSet[e] = true
		outputSet[e] = true
	}

	eEq := 0
	for _, e := range ne {
		if _, ok := actualSet[e]; ok {
			eEq++
		}

		outputSet[e] = true
	}

	if eEq == len(outputSet) {
		return nil, false
	}

	var result []string
	for e := range outputSet {
		result = append(result, e)
	}

	return result, true
}

// Referencer can retrieve reference models (*model.Reference).
type Referencer interface {
	// References retrieves a slice of *model.Reference or an error.
	References() ([]*model.Reference, error)
}
