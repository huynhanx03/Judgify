"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card } from "@/components/ui/card";
import { register } from "@/services/auth.service";
import { findAllTraits, gachaRoll } from "@/services/cultivation.service";
import { ApiError } from "@/lib/api-client";
import { notify } from "@/lib/toast";
import { TEXT } from "@/constants/text";
import { TraitCard } from "@/modules/cultivation/trait-card";
import { TraitCodexModal } from "@/modules/cultivation/trait-codex-modal";
import type { TraitResponse } from "@/types/cultivation";
import { Loader2, Eye, EyeOff, Dices, BookOpen } from "lucide-react";

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

      {/* Phàm Trần — Basic Info */}
      <Card className="border-white/10 bg-white/[0.02] backdrop-blur-sm p-5 space-y-4">
        <h3 className="text-xs font-bold uppercase tracking-[0.2em] text-muted-foreground border-b border-white/10 pb-2">
          {TEXT.AUTH.STEP_INFO}
        </h3>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.USERNAME}</Label>
            <Input placeholder={TEXT.AUTH.USERNAME_PLACEHOLDER} value={form.username} onChange={(e) => updateField("username", e.target.value)} className="h-10 bg-muted/30 border-white/10" />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.PASSWORD}</Label>
            <div className="relative">
              <Input type={showPassword ? "text" : "password"} placeholder={TEXT.AUTH.PASSWORD_PLACEHOLDER} value={form.password} onChange={(e) => updateField("password", e.target.value)} className="h-10 bg-muted/30 border-white/10 pr-10" />
              <button type="button" onClick={() => setShowPassword(!showPassword)} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground">
                {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
          </div>
        </div>

        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <div className="space-y-1.5">
            <Label className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.LAST_NAME}</Label>
            <Input placeholder={TEXT.AUTH.LAST_NAME_PLACEHOLDER} value={form.last_name} onChange={(e) => updateField("last_name", e.target.value)} className="h-10 bg-muted/30 border-white/10" />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.FIRST_NAME}</Label>
            <Input placeholder={TEXT.AUTH.FIRST_NAME_PLACEHOLDER} value={form.first_name} onChange={(e) => updateField("first_name", e.target.value)} className="h-10 bg-muted/30 border-white/10" />
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.GENDER}</Label>
            <div className="flex gap-1 h-10">
              {[
                { v: 0, l: TEXT.AUTH.GENDER_MALE },
                { v: 1, l: TEXT.AUTH.GENDER_FEMALE },
                { v: 2, l: TEXT.AUTH.GENDER_OTHER },
              ].map((opt) => (
                <button key={opt.v} type="button" onClick={() => updateField("gender", opt.v)}
                  className={`flex-1 rounded-md text-xs font-medium transition-all ${form.gender === opt.v ? "bg-primary text-primary-foreground" : "bg-muted/30 text-muted-foreground hover:bg-muted"}`}
                >
                  {opt.l}
                </button>
              ))}
            </div>
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.BIRTHDAY}</Label>
            <Input type="date" value={form.birthday} onChange={(e) => updateField("birthday", e.target.value)} className="h-10 bg-muted/30 border-white/10" />
          </div>
        </div>
      </Card>

      {/* Thiên Mệnh — Trait Selection */}
      <Card className="border-white/10 bg-white/[0.02] backdrop-blur-sm p-5 space-y-4 mt-6">
        <div className="flex items-center justify-between border-b border-white/10 pb-2">
          <h3 className="text-xs font-bold uppercase tracking-[0.2em] text-muted-foreground">
            {TEXT.AUTH.STEP_TRAITS}
          </h3>
          <button onClick={openCodex} className="text-muted-foreground hover:text-primary transition-colors" title="Xem tất cả traits">
            <BookOpen className="w-4 h-4" />
          </button>
        </div>

        {traitsLoading ? (
          <div className="h-[200px] flex items-center justify-center">
            <Loader2 className="w-8 h-8 animate-spin text-primary" />
          </div>
        ) : !rolledRootBone && rolledTalents.length === 0 ? (
          <div className="h-[200px] flex flex-col items-center justify-center border-2 border-dashed border-white/10 rounded-xl space-y-3">
            <p className="text-sm text-muted-foreground">{TEXT.AUTH.TRAIT_SUBTITLE}</p>
            <Button onClick={rollTraits} disabled={isRolling} className="bg-amber-600 hover:bg-amber-500 text-white font-bold tracking-widest uppercase gap-2">
              <Dices className="w-4 h-4" />
              {isRolling ? TEXT.AUTH.TRAIT_ROLLING : TEXT.AUTH.TRAIT_ROLL_BUTTON}
            </Button>
          </div>
        ) : (
          <div className="space-y-4">
            {/* Root Bone — full width, auto-selected */}
            {rolledRootBone && (
              <div>
                <p className="text-xs font-bold uppercase tracking-wider text-amber-400 mb-2">
                  {TEXT.AUTH.TRAIT_ROOT_BONE}
                </p>
                <TraitCard trait={rolledRootBone} selected animationDelay={0} />
              </div>
            )}

            {/* Talents — 3 rows × 2 cols */}
            <div>
              <p className="text-xs font-bold uppercase tracking-wider text-purple-400 mb-2">
                {TEXT.AUTH.TRAIT_SELECT_TALENTS} ({selectedTalents.length}/3)
              </p>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-2">
                {rolledTalents.map((trait, i) => (
                  <TraitCard
                    key={trait.id}
                    trait={trait}
                    selected={selectedTalents.includes(trait.id)}
                    onClick={() => toggleTalent(trait.id)}
                    animationDelay={0.15 + i * 0.08}
                  />
                ))}
              </div>
            </div>

            {/* Reroll */}
            <div className="flex justify-center">
              <Button variant="outline" className="px-8 py-2 text-sm gap-2 border-purple-500/40 text-purple-300 hover:bg-purple-500/10 hover:border-purple-400/60 transition-all" onClick={rollTraits} disabled={isRolling}>
                <Dices className="w-4 h-4" />
                {isRolling ? TEXT.AUTH.TRAIT_ROLLING : TEXT.AUTH.TRAIT_REROLL}
              </Button>
            </div>
          </div>
        )}
      </Card>

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
