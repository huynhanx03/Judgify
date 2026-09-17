package db

import (
	"context"
	"fmt"

	dbent "github.com/huynhanx03/judgify/internal/ent"
	"github.com/huynhanx03/judgify/pkg/judging"
)

// PublicRuntimeCatalog projects only browser-safe identities from the exact
// capabilities activated for one release. It never exposes execution tuples.
type PublicRuntimeCatalog struct {
	client    *dbent.EntClient
	releaseID string
}

func NewPublicRuntimeCatalog(
	client *dbent.EntClient,
	releaseID string,
) (*PublicRuntimeCatalog, error) {
	if client == nil || !intakeCapabilityIdentifier.MatchString(releaseID) {
		return nil, judging.ErrInvalidRuntimeCatalog
	}
	return &PublicRuntimeCatalog{
		client:    client,
		releaseID: releaseID,
	}, nil
}

func (catalog *PublicRuntimeCatalog) Read(
	ctx context.Context,
) (judging.RuntimeCatalog, error) {
	if catalog == nil || catalog.client == nil || ctx == nil {
		return judging.RuntimeCatalog{}, judging.ErrInvalidRuntimeCatalog
	}
	if err := ctx.Err(); err != nil {
		return judging.RuntimeCatalog{}, err
	}
	rows, err := catalog.client.DB(ctx).ActivatedJudgeCapability.QueryContext(
		ctx,
		publicRuntimeCatalogSQL,
		catalog.releaseID,
		judging.MaximumCatalogRuntimes+1,
	)
	if err != nil {
		return judging.RuntimeCatalog{}, fmt.Errorf(
			"read active public runtime catalog: %w",
			err,
		)
	}
	defer func() { _ = rows.Close() }()
	identities := make([]judging.RuntimeIdentity, 0, 8)
	for rows.Next() {
		var identity judging.RuntimeIdentity
		if err := rows.Scan(
			&identity.Key,
			&identity.Language,
			&identity.DisplayName,
		); err != nil {
			return judging.RuntimeCatalog{}, fmt.Errorf(
				"scan active public runtime catalog: %w",
				err,
			)
		}
		identities = append(identities, identity)
		if len(identities) > judging.MaximumCatalogRuntimes {
			return judging.RuntimeCatalog{}, judging.ErrInvalidRuntimeCatalog
		}
	}
	if err := rows.Err(); err != nil {
		return judging.RuntimeCatalog{}, fmt.Errorf(
			"read active public runtime catalog: %w",
			err,
		)
	}
	result, err := judging.NewRuntimeCatalog(identities)
	if err != nil {
		return judging.RuntimeCatalog{}, fmt.Errorf(
			"validate active public runtime catalog: %w",
			err,
		)
	}
	return result, nil
}

const publicRuntimeCatalogSQL = `
SELECT DISTINCT profile.runtime_key,
       profile.language,
       profile.display_name
FROM activated_judge_capabilities AS activation
JOIN runtime_profiles AS profile
  ON profile.runtime_key = activation.runtime_key
 AND profile.profile_version = activation.runtime_profile_version
 AND profile.compiler_version = activation.compiler_version
 AND profile.runtime_bootstrap_version = activation.runtime_bootstrap_version
 AND profile.compile_command_id = activation.compile_command_id
 AND profile.run_command_id = activation.run_command_id
 AND profile.sandbox_image_digest = activation.image_digest
 AND profile.manifest_version = activation.manifest_version
 AND profile.manifest_checksum = activation.manifest_checksum
WHERE activation.release_id = $1
  AND activation.state = 'active'
  AND activation.retired_at IS NULL
  AND profile.active
ORDER BY profile.runtime_key, profile.language, profile.display_name
LIMIT $2`
