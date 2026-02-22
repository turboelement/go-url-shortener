package repository

type URLRepositoryInterface interface {
	Save(shortID, originalURL string)
	Get(shortID string) (string, bool)
}
