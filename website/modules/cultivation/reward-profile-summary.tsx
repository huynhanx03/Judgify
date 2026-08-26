import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { TEXT } from "@/constants/text";
import { formatDateTime } from "@/lib/format";
import { TraitCard } from "@/modules/cultivation/trait-card";
import type {
  RewardProfileResponse,
  RewardProfileTrait,
} from "@/types/cultivation";

function traitCardData(trait: RewardProfileTrait) {
  return {
    id: trait.trait_id,
    type: trait.trait_type,
    name: trait.trait_name,
    description: trait.description,
    rarity: {
      code: trait.rarity_code,
      name: trait.rarity_name,
    },
    effect_revision: trait.effect_revision,
  };
}

export function RewardProfileSummary({
  profile,
}: {
  profile: RewardProfileResponse;
}) {
  const source =
    profile.source === "onboarding"
      ? TEXT.PROFILE.REWARD_PROFILE.SOURCE_ONBOARDING
      : TEXT.PROFILE.REWARD_PROFILE.SOURCE_ADMINISTRATIVE;

  return (
    <div className="space-y-5">
      <dl className="grid gap-3 rounded-xl border border-border/70 bg-muted/20 p-4 sm:grid-cols-3">
        <div className="space-y-1">
          <dt className="text-xs font-medium text-muted-foreground">
            {TEXT.PROFILE.REWARD_PROFILE.SOURCE}
          </dt>
          <dd><Badge variant="secondary">{source}</Badge></dd>
        </div>
        <div className="space-y-1">
          <dt className="text-xs font-medium text-muted-foreground">
            {TEXT.PROFILE.REWARD_PROFILE.EFFECTIVE_AT}
          </dt>
          <dd className="text-sm font-medium">
            <time dateTime={profile.effective_at}>
              {formatDateTime(profile.effective_at)}
            </time>
          </dd>
        </div>
        <div className="min-w-0 space-y-1">
          <dt className="text-xs font-medium text-muted-foreground">
            {TEXT.PROFILE.REWARD_PROFILE.CHECKSUM}
          </dt>
          <dd
            className="truncate font-mono text-xs text-foreground"
            title={profile.assignment_checksum}
          >
            {profile.assignment_checksum}
          </dd>
        </div>
      </dl>

      <RewardProfileTraitGrid traits={profile.traits} idPrefix={profile.id} />
    </div>
  );
}

export function RewardProfileTraitGrid({
  traits,
  idPrefix,
}: {
  traits: RewardProfileTrait[];
  idPrefix: string;
}) {
  const root = traits.find((trait) => trait.slot === "root_bone");
  const talents = traits
    .filter((trait) => trait.slot === "talent")
    .sort((left, right) => left.slot_order - right.slot_order);
  return (
    <div className="space-y-5">
      {root ? (
        <section aria-labelledby={`reward-profile-root-${idPrefix}`}>
          <h3
            id={`reward-profile-root-${idPrefix}`}
            className="mb-3 text-xs font-bold tracking-wider text-muted-foreground uppercase"
          >
            {TEXT.AUTH.TRAIT_ROOT_BONE}
          </h3>
          <TraitCard trait={traitCardData(root)} variant="compact" />
        </section>
      ) : null}
      <Separator />
      <section aria-labelledby={`reward-profile-talents-${idPrefix}`}>
        <h3
          id={`reward-profile-talents-${idPrefix}`}
          className="mb-3 text-xs font-bold tracking-wider text-muted-foreground uppercase"
        >
          {TEXT.PROFILE.TALENTS_LABEL}
        </h3>
        <div className="grid gap-2 md:grid-cols-3">
          {talents.map((trait) => (
            <TraitCard
              key={trait.trait_id}
              trait={traitCardData(trait)}
              variant="compact"
            />
          ))}
        </div>
      </section>
    </div>
  );
}
