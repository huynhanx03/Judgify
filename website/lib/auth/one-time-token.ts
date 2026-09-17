import { ONE_TIME_CREDENTIAL } from "@/constants/identity";

interface OneTimeTokenBrowser {
  location: Pick<Location, "hash" | "href">;
  history: Pick<History, "state" | "replaceState">;
}

interface ConsumeOneTimeTokenOptions {
  browser?: OneTimeTokenBrowser;
  fragmentKey?: string;
  maximumLength?: number;
}

/**
 * Moves an opaque one-time credential from the URL fragment into caller-owned
 * memory and immediately removes every browser-visible copy of that key.
 */
export function consumeOneTimeTokenFragment({
  browser,
  fragmentKey = ONE_TIME_CREDENTIAL.FRAGMENT_KEY,
  maximumLength = ONE_TIME_CREDENTIAL.MAX_LENGTH,
}: ConsumeOneTimeTokenOptions = {}): string | null {
  const activeBrowser = browser ?? window;
  const fragmentParameters = new URLSearchParams(
    activeBrowser.location.hash.startsWith("#")
      ? activeBrowser.location.hash.slice(1)
      : activeBrowser.location.hash,
  );
  const credential = fragmentParameters.get(fragmentKey) ?? "";

  const sanitizedURL = new URL(activeBrowser.location.href);
  sanitizedURL.hash = "";
  sanitizedURL.searchParams.delete(fragmentKey);
  activeBrowser.history.replaceState(
    activeBrowser.history.state,
    "",
    `${sanitizedURL.pathname}${sanitizedURL.search}`,
  );

  return credential.length > 0 && credential.length <= maximumLength
    ? credential
    : null;
}
