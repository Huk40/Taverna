package database

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

func IsReferenceConflict(err error) bool {
	var pgerr *pgconn.PgError
	return errors.As(err, &pgerr) && pgerr.Code == "23503"
}
