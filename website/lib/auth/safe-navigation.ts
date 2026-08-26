const CONTROL_CHARACTER_PATTERN = /[\u0000-\u001f\u007f]/;

/**
 * Accept only an application-relative destination. This is the single
 * boundary for `next`/post-auth navigation and prevents protocol-relative,
 * absolute, credential-bearing, and script URLs from reaching Next router.
 */
export function safeAppDestination(
  value: string | null | undefined,
  fallback: string,
): string {
  if (
    !value ||
    !value.startsWith("/") ||
    value.startsWith("//") ||
    value.includes("\\") ||
    CONTROL_CHARACTER_PATTERN.test(value)
  ) {
    return fallback;
  }
  try {
    const base = new URL("https://judgify.invalid/");
    const parsed = new URL(value, base);
    if (parsed.origin !== base.origin) return fallback;
    return `${parsed.pathname}${parsed.search}${parsed.hash}`;
  } catch {
    return fallback;
  }
}
