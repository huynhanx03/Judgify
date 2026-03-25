"use client";

/**
 * Login page with username/password form.
 * Premium glassmorphism design.
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
import { login } from "@/services/auth.service";
import { TEXT } from "@/constants/text";
import { Loader2 } from "lucide-react";

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setIsLoading(true);
    setError("");

    try {
      await login({ username, password });
      router.push("/arena");
    } catch {
      setError(TEXT.COMMON.ERROR);
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <Card className="glass-card shadow-2xl border-white/10 dark:border-white/5 bg-background/60 dark:bg-zinc-950/60 transition-all">
      <CardHeader className="text-center space-y-4 pb-6">
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-amber-700 text-primary-foreground text-2xl font-bold heading-gaming shadow-[0_0_20px_var(--color-primary)]">
          J
        </div>
        <div className="space-y-1">
          <CardTitle className="text-2xl font-bold tracking-tight">
            {TEXT.AUTH.WELCOME_BACK}
          </CardTitle>
          <CardDescription className="text-muted-foreground">
            {TEXT.AUTH.WELCOME_SUBTITLE}
          </CardDescription>
        </div>
      </CardHeader>

      <CardContent>
        <form onSubmit={handleSubmit} className="space-y-5">
          {error && (
            <div className="rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive border border-destructive/20 animate-in fade-in">
              {error}
            </div>
          )}
          
          <div className="space-y-2">
            <Label htmlFor="username" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {TEXT.AUTH.USERNAME}
            </Label>
            <Input
              id="username"
              type="text"
              placeholder={TEXT.AUTH.USERNAME_PLACEHOLDER}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
              className="h-11 bg-muted/50 focus-visible:ring-primary focus-visible:bg-background transition-all"
            />
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <Label htmlFor="password" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                {TEXT.AUTH.PASSWORD}
              </Label>
              <Link
                href="/forgot-password"
                className="text-xs font-medium text-primary hover:text-primary/80 transition-colors"
              >
                {TEXT.AUTH.FORGOT_PASSWORD}
              </Link>
            </div>
            <Input
              id="password"
              type="password"
              placeholder={TEXT.AUTH.PASSWORD_PLACEHOLDER}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              className="h-11 bg-muted/50 focus-visible:ring-primary focus-visible:bg-background transition-all"
            />
          </div>

          <Button 
            type="submit" 
            className="w-full h-11 bg-primary hover:bg-primary/90 text-primary-foreground font-semibold text-md shadow-[0_4px_14px_0_rgba(245,158,11,0.39)] transition-all" 
            disabled={isLoading}
          >
            {isLoading ? (
              <Loader2 className="mr-2 h-5 w-5 animate-spin" />
            ) : null}
            {isLoading ? TEXT.COMMON.LOADING : TEXT.AUTH.LOGIN}
          </Button>
          
          <div className="relative mt-6">
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
            {TEXT.AUTH.NO_ACCOUNT}{" "}
            <Link
              href="/register"
              className="font-semibold text-foreground hover:text-primary transition-colors hover:underline"
            >
              {TEXT.AUTH.REGISTER}
            </Link>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
