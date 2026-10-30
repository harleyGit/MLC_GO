package im

import (
	"errors"
	"github.com/go-sql-driver/mysql"
)

func hgCallWriteError(err error) error {
	var dbErr *mysql.MySQLError
	if errors.As(err, &dbErr) && dbErr.Number == 1062 {
		return HGErrConflict
	}
	return err
}
