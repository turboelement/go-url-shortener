package repository

type URLRepository struct {
	store map[string]string
}

func NewURLRepository() *URLRepository {
	return &URLRepository{
		store: make(map[string]string),
	}
}

func (r *URLRepository) Save(shortID, originalURL string) {
	r.store[shortID] = originalURL
}

func (r *URLRepository) Get(shortID string) (string, bool) {
	url, ok := r.store[shortID]
	return url, ok
}
