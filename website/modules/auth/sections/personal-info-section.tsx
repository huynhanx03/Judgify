"use client";

/**
 * Registration step 1 — personal info card.
 * Receives form state + handlers from RegisterFlow.
 */

import { Eye, EyeOff } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { TEXT } from "@/constants/text";

interface PersonalInfoSectionProps {
  form: {
    username: string;
    password: string;
    first_name: string;
    last_name: string;
    gender: number;
    birthday: string;
  };
  updateField: (field: string, value: string | number) => void;
  showPassword: boolean;
  onTogglePassword: () => void;
}

export function PersonalInfoSection({ form, updateField, showPassword, onTogglePassword }: PersonalInfoSectionProps) {
  return (
    <Card className="border-white/10 bg-white/[0.02] backdrop-blur-sm p-5 space-y-4">
      <h3 className="text-xs font-bold uppercase tracking-[0.2em] text-muted-foreground border-b border-white/10 pb-2">
        {TEXT.AUTH.STEP_INFO}
      </h3>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
        <div className="space-y-1.5">
          <Label className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.USERNAME}</Label>
          <Input placeholder={TEXT.AUTH.USERNAME_PLACEHOLDER} value={form.username}
            onChange={(e) => updateField("username", e.target.value)} className="h-10 bg-muted/30 border-white/10" />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.PASSWORD}</Label>
          <div className="relative">
            <Input type={showPassword ? "text" : "password"} placeholder={TEXT.AUTH.PASSWORD_PLACEHOLDER}
              value={form.password} onChange={(e) => updateField("password", e.target.value)}
              className="h-10 bg-muted/30 border-white/10 pr-10" />
            <button type="button" onClick={onTogglePassword}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground">
              {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <div className="space-y-1.5">
          <Label className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.LAST_NAME}</Label>
          <Input placeholder={TEXT.AUTH.LAST_NAME_PLACEHOLDER} value={form.last_name}
            onChange={(e) => updateField("last_name", e.target.value)} className="h-10 bg-muted/30 border-white/10" />
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.FIRST_NAME}</Label>
          <Input placeholder={TEXT.AUTH.FIRST_NAME_PLACEHOLDER} value={form.first_name}
            onChange={(e) => updateField("first_name", e.target.value)} className="h-10 bg-muted/30 border-white/10" />
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
                className={`flex-1 rounded-md text-xs font-medium transition-all ${form.gender === opt.v ? "bg-primary text-primary-foreground" : "bg-muted/30 text-muted-foreground hover:bg-muted"}`}>
                {opt.l}
              </button>
            ))}
          </div>
        </div>
        <div className="space-y-1.5">
          <Label className="text-xs font-semibold uppercase text-muted-foreground">{TEXT.AUTH.BIRTHDAY}</Label>
          <Input type="date" value={form.birthday}
            onChange={(e) => updateField("birthday", e.target.value)} className="h-10 bg-muted/30 border-white/10" />
        </div>
      </div>
    </Card>
  );
}
