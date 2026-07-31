package CoinRepositoryPackage

import (
	CoinModelPackage "MLC_GO/internal/modules/coin/model"
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Run with: MLC_COIN_INTEGRATION=1 MLC_COIN_MYSQL_DSN='user:pass@tcp(host:3306)/db?parseTime=true&loc=UTC' go test -race ./internal/modules/coin/repository -run TestHGRealMySQLConcurrentDebit
func TestHGRealMySQLConcurrentDebit(t *testing.T) {
	if os.Getenv("MLC_COIN_INTEGRATION") != "1" {
		t.Skip("set MLC_COIN_INTEGRATION=1 to run coin MySQL concurrency test")
	}
	dsn := os.Getenv("MLC_COIN_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("MLC_COIN_MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repository := NewHGRepository(db, "mlc.domain.events")
	userID := fmt.Sprintf("coin-stress-%d", time.Now().UnixNano())
	seed := CoinModelPackage.HGCommand{Operation: CoinModelPackage.HGOperationGrant, UserID: userID, RequestID: "seed", Amount: 200, Reason: "integration"}
	if _, err := repository.Mutate(ctx, seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	for _, sameVideo := range []bool{true, false} {
		var wg sync.WaitGroup
		var successes atomic.Int64
		errs := make(chan error, 100)
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				video := "shared-video"
				limit := uint64(2)
				if !sameVideo {
					video = fmt.Sprintf("video-%d", index)
					limit = 0
				}
				_, debitErr := repository.Mutate(ctx, CoinModelPackage.HGCommand{Operation: CoinModelPackage.HGOperationDebit, UserID: userID, RequestID: fmt.Sprintf("debit-%t-%d", sameVideo, index), Amount: 1, BusinessType: "video_coin", BusinessKey: video, BusinessLimit: limit})
				if debitErr != nil && !(sameVideo && debitErr == ErrHGBusinessLimit) {
					errs <- debitErr
				} else if debitErr == nil {
					successes.Add(1)
				}
			}(i)
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Fatalf("sameVideo=%t debit: %v", sameVideo, err)
		}
		want := int64(100)
		if sameVideo {
			want = 2
		}
		if successes.Load() != want {
			t.Fatalf("sameVideo=%t successes=%d want=%d", sameVideo, successes.Load(), want)
		}
	}
	var balance uint64
	if err := db.QueryRowContext(ctx, "SELECT balance FROM user_coin_wallets WHERE user_id = ?", userID).Scan(&balance); err != nil {
		t.Fatalf("read final balance: %v", err)
	}
	if balance != 98 {
		t.Fatalf("final balance = %d, want 98", balance)
	}
}
