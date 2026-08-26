"use client";

import { useState } from "react";
import Link from "next/link";
import { notify } from "@/lib/toast";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuth } from "@/contexts/auth-context";
import { ApiError } from "@/lib/api/error";
import { TEXT } from "@/constants/text";
import { Loader2 } from "lucide-react";
import { authService } from "@/services/auth.service";
import { APP_ROUTES } from "@/constants/routes";
import { OAUTH_PROVIDERS } from "@/constants/identity";
import type { OAuthProvider } from "@/types/auth";
import { OAuthProviderIcon } from "@/modules/auth/oauth-provider-icon";

export default function LoginForm() {
  const { login } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [oauthLoading, setOAuthLoading] = useState<OAuthProvider | null>(null);

  const isBusy = isLoading || oauthLoading !== null;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setIsLoading(true);

    try {
      await login({ username, password });
      notify.success(TEXT.AUTH.LOGIN_SUCCESS);
    } catch (err) {
			notify.error(
				err instanceof ApiError && (err.status === 401 || err.status === 404)
					? TEXT.AUTH.LOGIN_INVALID
					: TEXT.AUTH.LOGIN_ERROR,
			);
    } finally {
      setIsLoading(false);
    }
  }

  async function handleOAuthLogin(provider: OAuthProvider) {
		setOAuthLoading(provider);
    try {
			const response = await authService.startOAuth(provider);
      window.location.assign(response.auth_url);
    } catch {
			notify.error(
				TEXT.AUTH.OAUTH_START_ERROR(TEXT.AUTH.OAUTH_PROVIDER_NAME[provider]),
			);
			setOAuthLoading(null);
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-5" aria-busy={isBusy}>
      <div className="text-center space-y-1 mb-2">
        <h1 className="text-xl md:text-2xl font-black tracking-[0.15em] text-primary uppercase font-serif text-shadow-brand">
          {TEXT.AUTH.LOGIN_TITLE}
        </h1>
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
          disabled={isBusy}
          autoComplete="username"
          className="h-11 border-border bg-background/40 transition-colors focus-visible:bg-background/80 focus-visible:ring-primary"
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
          disabled={isBusy}
          autoComplete="current-password"
          className="h-11 border-border bg-background/40 transition-colors focus-visible:bg-background/80 focus-visible:ring-primary"
        />
        <div className="flex justify-end">
          <Link
            href={APP_ROUTES.FORGOT_PASSWORD}
            className="text-xs font-medium text-primary hover:text-primary/80 transition-colors"
          >
            {TEXT.AUTH.FORGOT_PASSWORD}
          </Link>
        </div>
      </div>

      <Button
        type="submit"
        className="button-brand-elevation h-12 w-full bg-primary font-bold tracking-wider text-primary-foreground uppercase transition-colors hover:bg-primary/90"
        disabled={isBusy}
      >
        {isLoading ? <Loader2 className="mr-2 h-5 w-5 animate-spin motion-reduce:animate-none" aria-hidden="true" /> : null}
        {isLoading ? TEXT.COMMON.LOADING : TEXT.AUTH.LOGIN_BUTTON}
      </Button>

      {/* Divider */}
      <div className="relative flex items-center gap-4">
        <div className="h-px flex-1 bg-border" />
        <span className="text-xs text-muted-foreground uppercase tracking-wider">
          {TEXT.AUTH.DIVIDER_OR}
        </span>
        <div className="h-px flex-1 bg-border" />
      </div>

		<div className="grid gap-2 sm:grid-cols-2">
			{OAUTH_PROVIDERS.map((provider) => {
				const name = TEXT.AUTH.OAUTH_PROVIDER_NAME[provider];
				const loading = oauthLoading === provider;
				return (
					<Button
						key={provider}
						type="button"
						variant="outline"
						className="h-11 w-full cursor-pointer border-border bg-background/40 font-medium text-foreground transition-colors hover:bg-muted"
						onClick={() => void handleOAuthLogin(provider)}
						disabled={isBusy}
					>
						{loading ? (
							<Loader2 className="mr-2 h-5 w-5 animate-spin motion-reduce:animate-none" aria-hidden="true" />
						) : (
							<OAuthProviderIcon provider={provider} className="mr-2 h-5 w-5" />
						)}
						{loading
							? TEXT.AUTH.OAUTH_CONNECTING(name)
							: TEXT.AUTH.OAUTH_LOGIN_WITH(name)}
					</Button>
				);
			})}
		</div>

      {/* Link to register */}
      <p className="text-center text-sm text-muted-foreground">
        {TEXT.AUTH.NO_ACCOUNT}{" "}
        <Link href={APP_ROUTES.REGISTER} className="font-semibold text-primary hover:text-primary/80 transition-colors">
          {TEXT.AUTH.TAB_REGISTER}
        </Link>
      </p>
    </form>
  );
}
