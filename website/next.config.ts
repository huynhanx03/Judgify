import type { NextConfig } from "next";

const DEFAULT_DEVELOPMENT_API_ORIGIN = "http://127.0.0.1:8000";

const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  {
    key: "Strict-Transport-Security",
    value: "max-age=63072000; includeSubDomains; preload",
  },
  {
    key: "Permissions-Policy",
    value: "camera=(), geolocation=(), microphone=()",
  },
];

function developmentApiOrigin(): string {
  const configured =
    process.env.JUDGIFY_DEV_API_ORIGIN?.trim() ||
    DEFAULT_DEVELOPMENT_API_ORIGIN;
  const origin = new URL(configured);
  const loopback =
    origin.hostname === "localhost" ||
    origin.hostname === "127.0.0.1" ||
    origin.hostname === "[::1]";

  if (
    origin.protocol !== "http:" ||
    !loopback ||
    origin.username ||
    origin.password ||
    (origin.pathname !== "/" && origin.pathname !== "") ||
    origin.search ||
    origin.hash
  ) {
    throw new Error(
      "JUDGIFY_DEV_API_ORIGIN must be an HTTP loopback origin without credentials or a path",
    );
  }

  return origin.origin;
}

const nextConfig: NextConfig = {
  // Memoizes safe React work at compile time, reducing client re-render cost
  // without adding a second client-state/cache abstraction.
  reactCompiler: true,
  output: "standalone",
  poweredByHeader: false,
  compress: true,
  async headers() {
    return [
      {
        source: "/:path*",
        headers: securityHeaders,
      },
    ];
  },
  async rewrites() {
    if (process.env.NODE_ENV !== "development") return [];

    const apiOrigin = developmentApiOrigin();
    return [
      {
        source: "/api/:path*",
        destination: `${apiOrigin}/:path*`,
      },
      {
        source: "/ws",
        destination: `${apiOrigin}/ws`,
      },
    ];
  },
};

export default nextConfig;
