package db_test

import (
	"testing"
	"time"

	"sirtom/server/db"
)

func TestGetDailyUserLogByDate(t *testing.T) {
	db.GetDailyUserLogByDate(time.Now(), 7)
}
