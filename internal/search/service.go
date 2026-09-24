package search

import (
	"context"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

type SearchResult struct {
	Query     string      `json:"query"`
	TotalHits int         `json:"total_hits"`
	Hits      []SearchHit `json:"hits"`
}

func (s *Service) SearchProducts(ctx context.Context, q string) (*SearchResult, error) {
	q = strings.TrimSpace(q)
	hits, err := s.repo.SearchProducts(ctx, q)
	if err != nil {
		return nil, err
	}
	return &SearchResult{
		Query:     q,
		TotalHits: len(hits),
		Hits:      hits,
	}, nil
}

type SuggestionResult struct {
	Query       string   `json:"query"`
	Suggestions []string `json:"suggestions"`
}

func (s *Service) GetSuggestions(ctx context.Context, q string) (*SuggestionResult, error) {
	q = strings.TrimSpace(q)
	suggs, err := s.repo.GetSuggestions(ctx, q)
	if err != nil {
		return nil, err
	}
	return &SuggestionResult{
		Query:       q,
		Suggestions: suggs,
	}, nil
}
