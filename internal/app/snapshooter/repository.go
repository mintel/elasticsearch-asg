package snapshooter

import (
	"context"
	goerr "errors"
	"time"

	elastic "github.com/olivere/elastic/v7" // Elasticsearch client.
	"github.com/pkg/errors"                 // Wrap errors with stacktrace.
)

var (
	// ErrWrongType is returned by RepositoryService.Ensure when the
	// repository already exists but is of the wrong type.
	ErrWrongType = goerr.New("repository exists but is the wrong type")
)

// Repository represents an Elasticsearch snapshot repository.
//
// See also: https://www.elastic.co/guide/en/elasticsearch/reference/7.0/modules-snapshots.html#_repositories
type Repository struct {
	// The name of the repository.
	Name string

	// The type of the repository, such as "fs" or "s3".
	Type string

	// Settings for the repository.
	// Specific settings depend on Type.
	Settings map[string]string
}

// RepositoryService is an Elasticsearch client
// specific to a single snapshot repository.
type RepositoryService interface {
	// Ensure the snapshots repository exists with
	// given Name and Type. Settings are are not
	// checked. Returns ErrWrongType if a repository
	// with the correct Name but wrong Type exists.
	Ensure(context.Context) error

	// Create a snapshot named using SnapshotFormat
	// and the return the time it was created.
	CreateSnapshot(context.Context) (time.Time, error)

	// List the snapshots in the repository by time.
	// Snapshots that don't match SnapshotFormat
	// are ignored.
	ListSnapshots(context.Context) ([]time.Time, error)

	// Deletes a snapshot.
	DeleteSnapshot(context.Context, time.Time) error
}

// NewRepositoryService returns a new RepositoryService.
func NewRepositoryService(c *elastic.Client, r *Repository, dryRun bool) RepositoryService {
	if dryRun {
		return &nopRepositoryService{
			c: c,
			r: r,
		}
	} else {
		return &repositoryService{
			c: c,
			r: r,
		}
	}
}

// repositoryService is the standard implementation of
// RepositoryService.
type repositoryService struct {
	c *elastic.Client
	r *Repository
}

func (s *repositoryService) Ensure(ctx context.Context) error {
	resp, err := s.c.SnapshotGetRepository(s.r.Name).Do(ctx)
	if err != nil && !elastic.IsNotFound(err) {
		// Unexpected error while checking if snapshot repository exists.
		return errors.Wrap(err, "error ensuring Elasticsearch snapshot repository")
	} else if existingRepo, ok := resp[s.r.Name]; elastic.IsNotFound(err) || !ok {
		scr := s.c.SnapshotCreateRepository(s.r.Name).Type(s.r.Type)
		for k, v := range s.r.Settings {
			scr = scr.Setting(k, v)
		}
		if _, err = scr.Do(ctx); err != nil {
			return errors.Wrap(err, "error creating Elasticsearch snapshot repository")
		}
	} else if ok && existingRepo.Type != s.r.Type {
		// Snapshot repository exists, but is of the wrong type e.g. fs != s3.
		err = ErrWrongType
	}
	return err
}

func (s *repositoryService) CreateSnapshot(ctx context.Context) (time.Time, error) {
	t := time.Now().UTC().Truncate(time.Second)
	n := t.Format(SnapshotFormat)
	_, err := s.c.SnapshotCreate(s.r.Name, n).WaitForCompletion(true).Do(ctx)
	if err != nil {
		err = errors.Wrap(err, "error creating snapshot")
		return time.Time{}, err
	}
	return t, nil
}

func (s *repositoryService) ListSnapshots(ctx context.Context) ([]time.Time, error) {
	r, err := s.c.SnapshotGet(s.r.Name).Do(ctx)
	if err != nil {
		err = errors.Wrap(err, "error listing snapshots")
		return nil, err
	}
	times := make([]time.Time, 0, len(r.Snapshots))
	for _, snap := range r.Snapshots {
		t, err := time.Parse(SnapshotFormat, snap.Snapshot)
		if err != nil {
			continue
		}
		times = append(times, t)
	}
	return times, nil
}

func (s *repositoryService) DeleteSnapshot(ctx context.Context, t time.Time) error {
	n := t.Format(SnapshotFormat)
	_, err := s.c.SnapshotDelete(s.r.Name, n).Do(ctx)
	err = errors.Wrap(err, "error deleting snapshot")
	return err
}

// nopRepositoryService is a RepositoryService that
// does nothing for Ensure, CreateSnapshot, and DeleteSnapshot.
// Use for dry runs.
type nopRepositoryService struct {
	c *elastic.Client
	r *Repository
}

func (s *nopRepositoryService) Ensure(ctx context.Context) error {
	return nil
}

func (s *nopRepositoryService) CreateSnapshot(ctx context.Context) (time.Time, error) {
	t := time.Now().UTC().Truncate(time.Second)
	return t, nil
}

func (s *nopRepositoryService) ListSnapshots(ctx context.Context) ([]time.Time, error) {
	rs := &repositoryService{
		c: s.c,
		r: s.r,
	}
	return rs.ListSnapshots(ctx)
}

func (s *nopRepositoryService) DeleteSnapshot(ctx context.Context, t time.Time) error {
	return nil
}
