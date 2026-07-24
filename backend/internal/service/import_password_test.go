package service

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"go-cpabe/backend/internal/domain"
)

// TestHashInitialPasswordsWithBoundsConcurrency 验证万级密码预处理使用受限工作池，并把摘要写回对应行而不串行执行。
func TestHashInitialPasswordsWithBoundsConcurrency(t *testing.T) {
	const jobCount = 18
	results := make([]domain.ImportRowResult, jobCount)
	jobs := make([]importPasswordHashJob, jobCount)
	for index := range jobCount {
		results[index] = domain.ImportRowResult{Fields: map[string]string{}}
		jobs[index] = importPasswordHashJob{ResultIndex: index, Password: fmt.Sprintf("password-%d", index)}
	}
	var active atomic.Int32
	var maximum atomic.Int32
	hasher := func(password string) (string, error) {
		current := active.Add(1)
		for {
			previous := maximum.Load()
			if current <= previous || maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
		active.Add(-1)
		return "hash:" + password, nil
	}
	if err := hashInitialPasswordsWith(context.Background(), results, jobs, 3, hasher); err != nil {
		t.Fatalf("hash passwords: %v", err)
	}
	if maximum.Load() < 2 || maximum.Load() > 3 {
		t.Fatalf("unexpected maximum concurrency: %d", maximum.Load())
	}
	for index := range jobCount {
		want := fmt.Sprintf("hash:password-%d", index)
		if got := results[index].Fields["initial_password_hash"]; got != want {
			t.Fatalf("row %d hash = %q, want %q", index, got, want)
		}
	}
}
