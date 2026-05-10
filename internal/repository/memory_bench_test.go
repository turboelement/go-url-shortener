package repository

import (
	"context"
	"testing"
)

func BenchmarkMemorySave(b *testing.B) {
	repo := NewURLRepository()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		shortID := "short-" + itoa(i)
		url := "https://example.com/" + itoa(i)
		_, err := repo.Save(ctx, shortID, url)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemorySaveWithUser(b *testing.B) {
	repo := NewURLRepository()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		shortID := "short-" + itoa(i)
		url := "https://example.com/" + itoa(i)
		_, err := repo.SaveWithUser(ctx, shortID, url, "bench-user")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryGet_Hit(b *testing.B) {
	repo := NewURLRepository()
	ctx := context.Background()

	// Pre-populate
	shortID := "target-id"
	_, err := repo.Save(ctx, shortID, "https://example.com/target")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := repo.Get(ctx, shortID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryGet_Miss(b *testing.B) {
	repo := NewURLRepository()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = repo.Get(ctx, "non-existent-id")
	}
}

func BenchmarkMemoryGetUserURLs_Small(b *testing.B) {
	benchmarkGetUserURLs(b, 10, "test-user")
}

func BenchmarkMemoryGetUserURLs_Medium(b *testing.B) {
	benchmarkGetUserURLs(b, 100, "test-user")
}

func BenchmarkMemoryGetUserURLs_Large(b *testing.B) {
	benchmarkGetUserURLs(b, 1000, "test-user")
}

func benchmarkGetUserURLs(b *testing.B, numURLs int, userID string) {
	repo := NewURLRepository()
	ctx := context.Background()

	for i := 0; i < numURLs; i++ {
		shortID := "short-" + itoa(i)
		url := "https://example.com/" + itoa(i)
		_, err := repo.SaveWithUser(ctx, shortID, url, userID)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := repo.GetUserURLs(ctx, userID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryBatchSave_Small(b *testing.B) {
	benchmarkBatchSave(b, 10)
}

func BenchmarkMemoryBatchSave_Medium(b *testing.B) {
	benchmarkBatchSave(b, 100)
}

func BenchmarkMemoryBatchSave_Large(b *testing.B) {
	benchmarkBatchSave(b, 1000)
}

func benchmarkBatchSave(b *testing.B, batchSize int) {
	repo := NewURLRepository()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		items := make([]BatchEntry, batchSize)
		for j := 0; j < batchSize; j++ {
			items[j] = BatchEntry{
				ShortID:     "batch-" + itoa(i) + "-" + itoa(j),
				OriginalURL: "https://example.com/batch/" + itoa(i) + "/" + itoa(j),
			}
		}

		err := repo.BatchSave(ctx, "test-user", items)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMemoryDeleteUserURLs(b *testing.B) {
	repo := NewURLRepository()
	ctx := context.Background()

	// Pre-populate
	shortIDs := make([]string, 100)
	for i := 0; i < 100; i++ {
		shortID := "del-" + itoa(i)
		url := "https://example.com/del/" + itoa(i)
		_, err := repo.SaveWithUser(ctx, shortID, url, "test-user")
		if err != nil {
			b.Fatal(err)
		}
		shortIDs[i] = shortID
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := repo.DeleteUserURLs(ctx, "test-user", shortIDs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		digit := n % 10
		s = string('0'+rune(digit)) + s
		n /= 10
	}
	return s
}
