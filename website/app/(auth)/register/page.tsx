"use client";

/**
 * Register page with multi-field form.
 * Premium glassmorphism design matching login page.
 */

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { register } from "@/services/auth.service";
import { TEXT } from "@/constants/text";
import { Loader2, CheckCircle2, Eye, EyeOff } from "lucide-react";

const GENDER_OPTIONS = [
  { value: 0, label: TEXT.AUTH.GENDER_MALE },
  { value: 1, label: TEXT.AUTH.GENDER_FEMALE },
  { value: 2, label: TEXT.AUTH.GENDER_OTHER },
];

export default function RegisterPage() {
  const router = useRouter();
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);

  const [form, setForm] = useState({
    username: "",
    password: "",
    confirmPassword: "",
    first_name: "",
    last_name: "",
    gender: 0,
    birthday: "",
  });

  function updateField(field: string, value: string | number) {
    setForm((prev) => ({ ...prev, [field]: value }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    if (form.password !== form.confirmPassword) {
      setError(TEXT.AUTH.PASSWORD_MISMATCH);
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
      });
      setSuccess(true);
      setTimeout(() => router.push("/login"), 1500);
    } catch {
      setError(TEXT.COMMON.ERROR);
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <Card className="glass-card shadow-2xl border-white/10 dark:border-white/5 bg-background/60 dark:bg-zinc-950/60 transition-all">
      <CardHeader className="text-center space-y-4 pb-4">
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-amber-700 text-primary-foreground text-2xl font-bold heading-gaming shadow-[0_0_20px_var(--color-primary)]">
          J
        </div>
        <div className="space-y-1">
          <CardTitle className="text-2xl font-bold tracking-tight">
            {TEXT.AUTH.REGISTER_TITLE}
          </CardTitle>
          <CardDescription className="text-muted-foreground">
            {TEXT.AUTH.REGISTER_SUBTITLE}
          </CardDescription>
        </div>
      </CardHeader>

      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive border border-destructive/20 animate-in fade-in">
              {error}
            </div>
          )}

          {success && (
            <div className="rounded-lg bg-emerald-500/10 px-4 py-3 text-sm text-emerald-500 border border-emerald-500/20 animate-in fade-in flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4" />
              {TEXT.AUTH.REGISTER_SUCCESS}
            </div>
          )}

          {/* Username */}
          <div className="space-y-2">
            <Label htmlFor="username" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {TEXT.AUTH.USERNAME}
            </Label>
            <Input
              id="username"
              type="text"
              placeholder={TEXT.AUTH.USERNAME_PLACEHOLDER}
              value={form.username}
              onChange={(e) => updateField("username", e.target.value)}
              required
              disabled={success}
              className="h-11 bg-muted/50 focus-visible:ring-primary focus-visible:bg-background transition-all"
            />
          </div>

          {/* Name row */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label htmlFor="first_name" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {TEXT.AUTH.FIRST_NAME}
              </Label>
              <Input
                id="first_name"
                type="text"
                placeholder={TEXT.AUTH.FIRST_NAME_PLACEHOLDER}
                value={form.first_name}
                onChange={(e) => updateField("first_name", e.target.value)}
                required
                disabled={success}
                className="h-11 bg-muted/50 focus-visible:ring-primary focus-visible:bg-background transition-all"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="last_name" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {TEXT.AUTH.LAST_NAME}
              </Label>
              <Input
                id="last_name"
                type="text"
                placeholder={TEXT.AUTH.LAST_NAME_PLACEHOLDER}
                value={form.last_name}
                onChange={(e) => updateField("last_name", e.target.value)}
                required
                disabled={success}
                className="h-11 bg-muted/50 focus-visible:ring-primary focus-visible:bg-background transition-all"
              />
            </div>
          </div>

          {/* Gender & Birthday row */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-2">
              <Label className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {TEXT.AUTH.GENDER}
              </Label>
              <div className="flex gap-1">
                {GENDER_OPTIONS.map((opt) => (
                  <button
                    key={opt.value}
                    type="button"
                    disabled={success}
                    onClick={() => updateField("gender", opt.value)}
                    className={`flex-1 h-11 rounded-lg text-sm font-medium transition-all cursor-pointer ${
                      form.gender === opt.value
                        ? "bg-primary text-primary-foreground shadow-[0_4px_14px_0_rgba(245,158,11,0.3)]"
                        : "bg-muted/50 text-muted-foreground hover:bg-muted hover:text-foreground"
                    }`}
                  >
                    {opt.label}
                  </button>
                ))}
              </div>
            </div>
            <div className="space-y-2">
              <Label htmlFor="birthday" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {TEXT.AUTH.BIRTHDAY}
              </Label>
              <Input
                id="birthday"
                type="date"
                value={form.birthday}
                onChange={(e) => updateField("birthday", e.target.value)}
                required
                disabled={success}
                className="h-11 bg-muted/50 focus-visible:ring-primary focus-visible:bg-background transition-all"
              />
            </div>
          </div>

          {/* Password */}
          <div className="space-y-2">
            <Label htmlFor="password" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {TEXT.AUTH.PASSWORD}
            </Label>
            <div className="relative">
              <Input
                id="password"
                type={showPassword ? "text" : "password"}
                placeholder={TEXT.AUTH.PASSWORD_PLACEHOLDER}
                value={form.password}
                onChange={(e) => updateField("password", e.target.value)}
                required
                disabled={success}
                className="h-11 bg-muted/50 focus-visible:ring-primary focus-visible:bg-background transition-all pr-10"
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
              >
                {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
          </div>

          {/* Confirm Password */}
          <div className="space-y-2">
            <Label htmlFor="confirmPassword" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {TEXT.AUTH.CONFIRM_PASSWORD}
            </Label>
            <div className="relative">
              <Input
                id="confirmPassword"
                type={showConfirm ? "text" : "password"}
                placeholder={TEXT.AUTH.CONFIRM_PASSWORD_PLACEHOLDER}
                value={form.confirmPassword}
                onChange={(e) => updateField("confirmPassword", e.target.value)}
                required
                disabled={success}
                className="h-11 bg-muted/50 focus-visible:ring-primary focus-visible:bg-background transition-all pr-10"
              />
              <button
                type="button"
                onClick={() => setShowConfirm(!showConfirm)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors cursor-pointer"
              >
                {showConfirm ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </button>
            </div>
          </div>

          <Button
            type="submit"
            className="w-full h-11 bg-primary hover:bg-primary/90 text-primary-foreground font-semibold text-md shadow-[0_4px_14px_0_rgba(245,158,11,0.39)] transition-all cursor-pointer"
            disabled={isLoading || success}
          >
            {isLoading ? <Loader2 className="mr-2 h-5 w-5 animate-spin" /> : null}
            {isLoading ? TEXT.COMMON.LOADING : TEXT.AUTH.REGISTER}
          </Button>

          <div className="relative mt-4">
            <div className="absolute inset-0 flex items-center">
              <span className="w-full border-t border-border/50" />
            </div>
            <div className="relative flex justify-center text-xs uppercase">
              <span className="bg-background/80 px-2 text-muted-foreground backdrop-blur-sm">
                Hoặc
              </span>
            </div>
          </div>

          <div className="text-center text-sm text-muted-foreground mt-4">
            {TEXT.AUTH.HAS_ACCOUNT}{" "}
            <Link
              href="/login"
              className="font-semibold text-foreground hover:text-primary transition-colors hover:underline"
            >
              {TEXT.AUTH.LOGIN}
            </Link>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
