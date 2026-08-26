import type { Metadata } from "next";
import { TEXT } from "@/constants/text";
import {
  parseOAuthCallbackQuery,
  type OAuthCallbackQuery,
} from "@/lib/auth/oauth-callback";
import { OAuthCompletionClient } from "@/modules/auth/oauth-completion-client";

export const metadata: Metadata = {
  title: TEXT.META.OAUTH_CALLBACK_TITLE,
};

export default async function OAuthCompletionPage({
  searchParams,
}: {
  searchParams: Promise<OAuthCallbackQuery>;
}) {
  return (
    <OAuthCompletionClient
      result={parseOAuthCallbackQuery(await searchParams)}
    />
  );
}
