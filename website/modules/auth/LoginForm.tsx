"use client";

import { useState } from "react";
import Link from "next/link";
import { notify } from "@/lib/toast";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuth } from "@/contexts/auth-context";
import { ApiError } from "@/lib/api-client";
import { TEXT } from "@/constants/text";
import { Loader2 } from "lucide-react";

function GoogleIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none">
      <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4" />
      <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
      <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18A10.96 10.96 0 0 0 1 12c0 1.77.42 3.45 1.18 4.93l3.66-2.84z" fill="#FBBC05" />
      <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
    </svg>
  );
}

export default function LoginForm() {
  const { login } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setIsLoading(true);

    try {
      await login({ username, password });
      notify.success("Đăng nhập thành công!");
    } catch (err) {
      if (err instanceof ApiError) {
        notify.error(err.message);
      } else {
        notify.error(TEXT.COMMON.ERROR);
      }
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-5">
      <div className="text-center space-y-1 mb-2">
        <h2 className="text-xl md:text-2xl font-black tracking-[0.15em] text-primary uppercase font-serif drop-shadow-[0_0_10px_rgba(245,158,11,0.3)]">
          {TEXT.AUTH.LOGIN_TITLE}
        </h2>
        <p className="text-[10px] text-muted-foreground uppercase tracking-widest font-medium">
          {TEXT.AUTH.LOGIN_SUBTITLE}
        </p>
      </div>

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
          className="h-11 bg-muted/30 focus-visible:ring-primary focus-visible:bg-black/20 transition-all border-white/10"
        />
      </div>

      <div className="space-y-2">
        <Label htmlFor="password" className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          {TEXT.AUTH.PASSWORD}
        </Label>
        <Input
          id="password"
          type="password"
          placeholder={TEXT.AUTH.PASSWORD_PLACEHOLDER}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
          className="h-11 bg-muted/30 focus-visible:ring-primary focus-visible:bg-black/20 transition-all border-white/10"
        />
        <div className="flex justify-end">
          <Link
            href="/forgot-password"
            className="text-xs font-medium text-primary hover:text-primary/80 transition-colors"
          >
            {TEXT.AUTH.FORGOT_PASSWORD}
          </Link>
        </div>
      </div>

      <Button
        type="submit"
        className="w-full h-12 bg-primary hover:bg-primary/90 text-primary-foreground font-bold text-md tracking-wider uppercase shadow-[0_0_20px_rgba(245,158,11,0.2)] hover:shadow-[0_0_25px_rgba(245,158,11,0.4)] transition-all"
        disabled={isLoading}
      >
        {isLoading ? <Loader2 className="mr-2 h-5 w-5 animate-spin" /> : null}
        {isLoading ? TEXT.COMMON.LOADING : TEXT.AUTH.LOGIN_BUTTON}
      </Button>

      {/* Divider */}
      <div className="relative flex items-center gap-4">
        <div className="flex-1 h-px bg-white/10" />
        <span className="text-xs text-muted-foreground uppercase tracking-wider">hoặc</span>
        <div className="flex-1 h-px bg-white/10" />
      </div>

      {/* Google Login */}
      <Button
        type="button"
        variant="outline"
        className="w-full h-11 border-white/10 bg-transparent hover:bg-white/5 text-foreground font-medium"
        onClick={() => {
          // TODO: Implement Google OAuth flow
          notify.info("Tính năng đang phát triển");
        }}
      >
        <GoogleIcon className="w-5 h-5 mr-2" />
        {TEXT.AUTH.LOGIN_GOOGLE}
      </Button>

      {/* Link to register */}
      <p className="text-center text-sm text-muted-foreground">
        {TEXT.AUTH.NO_ACCOUNT}{" "}
        <Link href="/register" className="font-semibold text-primary hover:text-primary/80 transition-colors">
          {TEXT.AUTH.TAB_REGISTER}
        </Link>
      </p>
    </form>
  );
}
