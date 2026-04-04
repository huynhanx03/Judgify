"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { register } from "@/services/auth.service";
import { findAllTraits, gachaRoll } from "@/services/cultivation.service";
import { ApiError } from "@/lib/api-client";
import { notify } from "@/lib/toast";
import { TEXT } from "@/constants/text";
import { PersonalInfoSection } from "@/modules/auth/sections/personal-info-section";
import { TraitSelectionSection } from "@/modules/auth/sections/trait-selection-section";
import { TraitCodexModal } from "@/modules/cultivation/trait-codex-modal";
import type { TraitResponse } from "@/types/cultivation";

export default function RegisterFlow() {
  const router = useRouter();
  const [isLoading, setIsLoading] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

  // Basic Info
  const [form, setForm] = useState({
    username: "", password: "", first_name: "", last_name: "", gender: 0, birthday: "",
  });

  // Traits
  const [allTraits, setAllTraits] = useState<TraitResponse[]>([]);
  const [traitsLoading, setTraitsLoading] = useState(false);
  const [rolledRootBone, setRolledRootBone] = useState<TraitResponse | null>(null);
  const [rolledTalents, setRolledTalents] = useState<TraitResponse[]>([]);
  const [selectedTalents, setSelectedTalents] = useState<number[]>([]);
  const [isRolling, setIsRolling] = useState(false);
  const [showCodex, setShowCodex] = useState(false);

  function updateField(field: string, value: string | number) {
    setForm((prev) => ({ ...prev, [field]: value }));
  }

  // Fetch all traits when opening codex
  const openCodex = useCallback(async () => {
    setShowCodex(true);
    if (allTraits.length > 0) return;
    setTraitsLoading(true);
    try {
      const data = await findAllTraits();
      setAllTraits(data);
    } catch {
      notify.error(TEXT.AUTH.TRAIT_FETCH_ERROR);
    } finally {
      setTraitsLoading(false);
    }
  }, [allTraits.length]);

  // Roll traits — 1 root bone (auto-selected), 6 talents
  const rollTraits = useCallback(async () => {
    if (isRolling) return;
    setIsRolling(true);
    setSelectedTalents([]);

    try {
      const result = await gachaRoll();
      // Only 1 root bone — auto-select
      setRolledRootBone(result.root_bones[0] ?? null);
      setRolledTalents(result.talents);
    } catch {
      notify.error(TEXT.AUTH.TRAIT_FETCH_ERROR);
    } finally {
      setIsRolling(false);
    }
  }, [isRolling]);

  // Auto-roll on mount
  const [hasAutoRolled, setHasAutoRolled] = useState(false);
  useEffect(() => {
    if (!hasAutoRolled) {
      setHasAutoRolled(true);
      rollTraits();
    }
  }, [hasAutoRolled, rollTraits]);

  const toggleTalent = (id: number) => {
    if (selectedTalents.includes(id)) {
      setSelectedTalents(selectedTalents.filter((t) => t !== id));
    } else {
      if (selectedTalents.length >= 3) {
        notify.warning(TEXT.AUTH.TRAIT_MAX_TALENTS);
        return;
      }
      setSelectedTalents([...selectedTalents, id]);
    }
  };

  const handleSubmit = async () => {
    if (!form.username || !form.password || !form.first_name || !form.last_name || !form.birthday) {
      notify.warning(TEXT.AUTH.REGISTER_MISSING_INFO);
      return;
    }
    if (!rolledRootBone) {
      notify.warning(TEXT.AUTH.TRAIT_ROOT_BONE_REQUIRED);
      return;
    }
    if (selectedTalents.length !== 3) {
      notify.warning(TEXT.AUTH.TRAIT_TALENTS_REQUIRED);
      return;
    }

    setIsLoading(true);
    try {
      await register({
        username: form.username,
        password: form.password,
        first_name: form.first_name,
        last_name: form.last_name,
        gender: form.gender,
        birthday: form.birthday,
        root_bone_id: rolledRootBone.id,
        talent_ids: selectedTalents,
      });
      notify.success(TEXT.AUTH.REGISTER_SUCCESS);
      router.push("/login");
    } catch (err) {
      if (err instanceof ApiError) {
        notify.error(err.message);
      } else {
        notify.error(TEXT.COMMON.ERROR);
      }
    } finally {
      setIsLoading(false);
    }
  };

  const canSubmit = form.username && form.password && form.first_name && form.last_name && form.birthday && rolledRootBone && selectedTalents.length === 3;

  return (
    <div className="w-full relative">
      {/* Header */}
      <div className="text-center space-y-1 mb-6">
        <h2 className="text-2xl md:text-3xl font-black tracking-[0.15em] text-primary uppercase font-serif drop-shadow-[0_0_10px_rgba(245,158,11,0.3)]">
          {TEXT.AUTH.REGISTER_TITLE}
        </h2>
        <p className="text-[10px] text-muted-foreground uppercase tracking-widest font-medium">
          {TEXT.AUTH.REGISTER_SUBTITLE}
        </p>
      </div>

      <PersonalInfoSection
        form={form}
        updateField={updateField}
        showPassword={showPassword}
        onTogglePassword={() => setShowPassword((p) => !p)}
      />

      <TraitSelectionSection
        rolledRootBone={rolledRootBone}
        rolledTalents={rolledTalents}
        selectedTalents={selectedTalents}
        isRolling={isRolling}
        traitsLoading={traitsLoading}
        onRoll={rollTraits}
        onToggleTalent={toggleTalent}
        onOpenCodex={openCodex}
      />

      {/* Submit + Link to login */}
      <div className="mt-6 space-y-4">
        <Button
          onClick={handleSubmit}
          className="w-full h-12 bg-primary hover:bg-primary/90 text-primary-foreground font-bold text-md tracking-[0.2em] uppercase shadow-[0_0_20px_rgba(245,158,11,0.2)] hover:shadow-[0_0_25px_rgba(245,158,11,0.4)] transition-all"
          disabled={isLoading || !canSubmit}
        >
          {isLoading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
          {isLoading ? TEXT.COMMON.LOADING : TEXT.AUTH.REGISTER_SUBMIT}
        </Button>

        <p className="text-center text-sm text-muted-foreground">
          {TEXT.AUTH.HAS_ACCOUNT}{" "}
          <Link href="/login" className="font-semibold text-primary hover:text-primary/80 transition-colors">
            {TEXT.AUTH.TAB_LOGIN}
          </Link>
        </p>
      </div>

      {/* Codex Modal */}
      <TraitCodexModal open={showCodex} onClose={() => setShowCodex(false)} traits={allTraits} loading={traitsLoading} />
    </div>
  );
}
