package llm

import (
	"fmt"
	"math"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func qsAt(qs []float64, i int) float64 {
	if len(qs) > i && !math.IsNaN(qs[i]) {
		return qs[i]
	}
	return 0
}

func wrapLLMError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("llm %s: %w", operation, err)
}
