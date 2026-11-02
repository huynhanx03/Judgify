package usecase

import (
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
)

import entity "github.com/huynhanx03/judgify/internal/cultivation/domain"

var errTraitOfferEntropy = errors.New("cultivation trait offer: entropy unavailable")

// sampleWeightedWithoutReplacement draws distinct candidates using unbiased
// crypto/rand-compatible bounded integers. The caller serializes access to a
// stateful injected reader; this function never touches process-global RNG.
func sampleWeightedWithoutReplacement(
	reader io.Reader,
	candidates []entity.CatalogTrait,
	count int,
) ([]entity.CatalogTrait, error) {
	if reader == nil || count < 1 || count > len(candidates) {
		return nil, entity.ErrTraitOfferInvalid
	}
	remaining := append([]entity.CatalogTrait(nil), candidates...)
	selected := make([]entity.CatalogTrait, 0, count)
	for len(selected) < count {
		var total int64
		for _, candidate := range remaining {
			if candidate.Weight < 1 || int64(candidate.Weight) > math.MaxInt64-total {
				return nil, entity.ErrTraitOfferInvalid
			}
			total += int64(candidate.Weight)
		}
		if total < 1 {
			return nil, entity.ErrTraitOfferInvalid
		}
		draw, err := randomIntBelow(reader, total)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errTraitOfferEntropy, err)
		}
		var cumulative int64
		chosen := -1
		for index, candidate := range remaining {
			cumulative += int64(candidate.Weight)
			if draw < cumulative {
				chosen = index
				break
			}
		}
		if chosen < 0 {
			return nil, entity.ErrTraitOfferInvalid
		}
		selected = append(selected, remaining[chosen])
		remaining = append(remaining[:chosen], remaining[chosen+1:]...)
	}
	return selected, nil
}

func randomIntBelow(reader io.Reader, maximum int64) (int64, error) {
	if reader == nil || maximum < 1 {
		return 0, entity.ErrTraitOfferInvalid
	}
	value, err := cryptorand.Int(reader, big.NewInt(maximum))
	if err != nil {
		return 0, err
	}
	return value.Int64(), nil
}
