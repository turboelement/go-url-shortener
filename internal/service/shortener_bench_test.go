package service

import (
	"context"
	"testing"

	"go-url-shortener/internal/repository"
)

func BenchmarkGenerateShortID(b *testing.B) {
	repo := repository.NewURLRepository()
	svc := NewShortenerService(repo)
	defer svc.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = svc.GenerateShortID()
	}
}

func BenchmarkShorten(b *testing.B) {
	repo := repository.NewURLRepository()
	svc := NewShortenerService(repo)
	defer svc.Close()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		url := "https://example.com/very/long/url/path/for/testing/" + itoa(i)
		_, err := svc.Shorten(ctx, url)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShortenWithUser(b *testing.B) {
	repo := repository.NewURLRepository()
	svc := NewShortenerService(repo)
	defer svc.Close()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		url := "https://example.com/very/long/url/path/for/testing/" + itoa(i)
		_, err := svc.ShortenWithUser(ctx, url, "test-user-id")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBatchShorten_Small(b *testing.B) {
	benchmarkBatchShorten(b, 10)
}

func BenchmarkBatchShorten_Medium(b *testing.B) {
	benchmarkBatchShorten(b, 100)
}

func BenchmarkBatchShorten_Large(b *testing.B) {
	benchmarkBatchShorten(b, 1000)
}

func benchmarkBatchShorten(b *testing.B, batchSize int) {
	repo := repository.NewURLRepository()
	svc := NewShortenerService(repo)
	defer svc.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		items := make([]BatchItem, batchSize)
		for j := 0; j < batchSize; j++ {
			items[j] = BatchItem{
				CorrelationID: "corr-" + itoa(i*batchSize+j),
				OriginalURL:   "https://example.com/" + itoa(i*batchSize+j),
			}
		}

		_, err := svc.BatchShorten(ctx, items)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetOriginalURL(b *testing.B) {
	repo := repository.NewURLRepository()
	svc := NewShortenerService(repo)
	defer svc.Close()

	ctx := context.Background()

	// Pre-populate with some URLs
	shortIDs := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		url := "https://example.com/bench/get/" + itoa(i)
		shortID, err := svc.Shorten(ctx, url)
		if err != nil {
			b.Fatal(err)
		}
		shortIDs[i] = shortID
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := svc.GetOriginalURL(ctx, shortIDs[i%1000])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetUserURLs(b *testing.B) {
	repo := repository.NewURLRepository()
	svc := NewShortenerService(repo)
	defer svc.Close()

	ctx := context.Background()

	// Pre-populate with 1000 URLs for a specific user
	userID := "bench-user-id"
	for i := 0; i < 1000; i++ {
		url := "https://example.com/bench/user/" + itoa(i)
		_, err := svc.ShortenWithUser(ctx, url, userID)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := svc.GetUserURLs(ctx, userID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeleteUserURLsAsync(b *testing.B) {
	repo := repository.NewURLRepository()
	svc := NewShortenerService(repo)
	defer svc.Close()

	ctx := context.Background()

	// Pre-populate with URLs
	shortIDs := make([]string, 100)
	for i := 0; i < 100; i++ {
		url := "https://example.com/bench/delete/" + itoa(i)
		shortID, err := svc.ShortenWithUser(ctx, url, "bench-delete-user")
		if err != nil {
			b.Fatal(err)
		}
		shortIDs[i] = shortID
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		svc.DeleteUserURLsAsync("bench-delete-user", shortIDs)
	}
}

// Simple int to string conversion without fmt.Sprintf to avoid extra allocs in benchmarks
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
