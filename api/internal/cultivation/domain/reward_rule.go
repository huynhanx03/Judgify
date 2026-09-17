package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	DefaultRewardRuleKey       = "solve-reward"
	DefaultRewardRuleAlgorithm = "base-exp-v1"
	RewardBasisPoints          = int64(10_000)
)

var (
	ErrInvalidRewardRule  = errors.New("cultivation: invalid reward rule revision")
	ErrRewardRuleConflict = errors.New("cultivation: reward rule revision conflict")
)

type RewardRuleConfiguration struct {
	MultiplierBasisPoints int64 `json:"multiplier_basis_points"`
	FlatBonus             int64 `json:"flat_bonus"`
}

type RewardRuleRevision struct {
	ID               uuid.UUID
	RuleKey          string
	Revision         int64
	AlgorithmVersion string
	Configuration    RewardRuleConfiguration
	Checksum         [sha256.Size]byte
	RequestChecksum  [sha256.Size]byte
	EffectiveFrom    time.Time
}

type RewardRulePublicationPreview struct {
	RuleKey          string
	CurrentRevision  int64
	NextRevision     int64
	AlgorithmVersion string
	Configuration    RewardRuleConfiguration
	Checksum         [sha256.Size]byte
}

type RewardRulePublicationCommand struct {
	ID               uuid.UUID
	ExpectedRevision int64
	AlgorithmVersion string
	Configuration    RewardRuleConfiguration
	Reason           string
	PreviewChecksum  [sha256.Size]byte
}

type RewardRulePublicationResult struct {
	Rule     RewardRuleRevision
	Replayed bool
}

func DefaultRewardRuleRevision() RewardRuleRevision {
	value := RewardRuleRevision{
		ID:               uuid.NewSHA1(uuid.NameSpaceURL, []byte("judgify/reward-rule/solve-reward/base-exp-v1/1")),
		RuleKey:          DefaultRewardRuleKey,
		Revision:         1,
		AlgorithmVersion: DefaultRewardRuleAlgorithm,
		Configuration:    RewardRuleConfiguration{MultiplierBasisPoints: RewardBasisPoints},
		EffectiveFrom:    time.Unix(0, 0).UTC(),
	}
	value.RequestChecksum = sha256.Sum256([]byte("judgify/cultivation/reward-rule/default/v1"))
	value.Checksum, _ = value.CalculateChecksum()
	return value
}

func (rule RewardRuleRevision) Valid() bool {
	if rule.ID == uuid.Nil || rule.RuleKey != DefaultRewardRuleKey ||
		rule.Revision < 1 || rule.AlgorithmVersion != DefaultRewardRuleAlgorithm ||
		rule.EffectiveFrom.IsZero() ||
		rule.Configuration.MultiplierBasisPoints < 0 ||
		rule.Configuration.MultiplierBasisPoints > 100_000 ||
		rule.Configuration.FlatBonus < -1_000_000_000 ||
		rule.Configuration.FlatBonus > 1_000_000_000 {
		return false
	}
	checksum, err := rule.CalculateChecksum()
	return err == nil && checksum == rule.Checksum && checksum != ([sha256.Size]byte{}) &&
		rule.RequestChecksum != ([sha256.Size]byte{})
}

func (rule RewardRuleRevision) CalculateChecksum() ([sha256.Size]byte, error) {
	document := struct {
		Schema           string                  `json:"schema"`
		ID               uuid.UUID               `json:"id"`
		RuleKey          string                  `json:"rule_key"`
		Revision         int64                   `json:"revision"`
		AlgorithmVersion string                  `json:"algorithm_version"`
		Configuration    RewardRuleConfiguration `json:"configuration"`
		EffectiveFrom    time.Time               `json:"effective_from"`
	}{
		Schema: "judgify.cultivation.reward-rule.v1",
		ID:     rule.ID, RuleKey: rule.RuleKey, Revision: rule.Revision,
		AlgorithmVersion: rule.AlgorithmVersion,
		Configuration:    rule.Configuration,
		EffectiveFrom:    rule.EffectiveFrom.UTC(),
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(encoded), nil
}

func NewRewardRulePublicationPreview(
	current RewardRuleRevision,
	configuration RewardRuleConfiguration,
) (RewardRulePublicationPreview, error) {
	if !current.Valid() || configuration.MultiplierBasisPoints < 0 ||
		configuration.MultiplierBasisPoints > 100_000 ||
		configuration.FlatBonus < -1_000_000_000 ||
		configuration.FlatBonus > 1_000_000_000 {
		return RewardRulePublicationPreview{}, ErrInvalidRewardRule
	}
	preview := RewardRulePublicationPreview{
		RuleKey: current.RuleKey, CurrentRevision: current.Revision,
		NextRevision:     current.Revision + 1,
		AlgorithmVersion: DefaultRewardRuleAlgorithm,
		Configuration:    configuration,
	}
	preview.Checksum = RewardRulePublicationPreviewChecksum(preview)
	return preview, nil
}

func NormalizeRewardRulePublicationCommand(
	command RewardRulePublicationCommand,
) (RewardRulePublicationCommand, error) {
	command.AlgorithmVersion = strings.TrimSpace(command.AlgorithmVersion)
	command.Reason = strings.TrimSpace(command.Reason)
	if command.ID == uuid.Nil || command.ExpectedRevision < 1 ||
		command.AlgorithmVersion != DefaultRewardRuleAlgorithm ||
		command.PreviewChecksum == ([sha256.Size]byte{}) ||
		!validRewardRuleReason(command.Reason) {
		return RewardRulePublicationCommand{}, ErrInvalidRewardRule
	}
	if _, err := NewRewardRulePublicationPreview(DefaultRewardRuleRevision(), command.Configuration); err != nil {
		return RewardRulePublicationCommand{}, err
	}
	return command, nil
}

func RewardRulePublicationPreviewChecksum(preview RewardRulePublicationPreview) [sha256.Size]byte {
	document := struct {
		Schema           string                  `json:"schema"`
		RuleKey          string                  `json:"rule_key"`
		CurrentRevision  int64                   `json:"current_revision"`
		NextRevision     int64                   `json:"next_revision"`
		AlgorithmVersion string                  `json:"algorithm_version"`
		Configuration    RewardRuleConfiguration `json:"configuration"`
	}{
		Schema:  "judgify.cultivation.reward-rule-publication-preview.v1",
		RuleKey: preview.RuleKey, CurrentRevision: preview.CurrentRevision,
		NextRevision:     preview.NextRevision,
		AlgorithmVersion: preview.AlgorithmVersion,
		Configuration:    preview.Configuration,
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(encoded)
}

func RewardRulePublicationRequestChecksum(command RewardRulePublicationCommand) [sha256.Size]byte {
	document := struct {
		Schema           string                  `json:"schema"`
		ID               uuid.UUID               `json:"id"`
		ExpectedRevision int64                   `json:"expected_revision"`
		AlgorithmVersion string                  `json:"algorithm_version"`
		Configuration    RewardRuleConfiguration `json:"configuration"`
		Reason           string                  `json:"reason"`
		PreviewChecksum  string                  `json:"preview_checksum"`
	}{
		Schema: "judgify.cultivation.reward-rule-publication.v1",
		ID:     command.ID, ExpectedRevision: command.ExpectedRevision,
		AlgorithmVersion: command.AlgorithmVersion,
		Configuration:    command.Configuration, Reason: command.Reason,
		PreviewChecksum: checksumHex(command.PreviewChecksum),
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(encoded)
}

func validRewardRuleReason(value string) bool {
	if !utf8.ValidString(value) || len(value) < 3 || len(value) > 1024 {
		return false
	}
	for _, current := range value {
		if unicode.IsControl(current) && current != '\n' && current != '\t' {
			return false
		}
	}
	return true
}

// Apply returns the immutable reward for one base value. Division rounds
// half-away-from-zero, matching the documented fixed-point formula.
func (rule RewardRuleRevision) Apply(base int64) (int64, error) {
	if !rule.Valid() || base < 0 ||
		(rule.Configuration.MultiplierBasisPoints > 0 &&
			base > math.MaxInt64/rule.Configuration.MultiplierBasisPoints) {
		return 0, ErrInvalidRewardRule
	}
	product := base * rule.Configuration.MultiplierBasisPoints
	value := product / RewardBasisPoints
	remainder := product % RewardBasisPoints
	if remainder*2 >= RewardBasisPoints {
		value++
	}
	if rule.Configuration.FlatBonus > 0 && value > math.MaxInt64-rule.Configuration.FlatBonus {
		return 0, ErrInvalidRewardRule
	}
	if rule.Configuration.FlatBonus < 0 && value < math.MinInt64-rule.Configuration.FlatBonus {
		return 0, ErrInvalidRewardRule
	}
	value += rule.Configuration.FlatBonus
	if value < 0 {
		return 0, nil
	}
	return value, nil
}
