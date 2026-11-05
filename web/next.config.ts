import type { NextConfig } from "next";

// The browser may talk only to this site, the API and the identity provider; nothing else can be
// loaded or called, which limits what an injected script could do with a session token.
const api = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
const oidc = process.env.NEXT_PUBLIC_OIDC_ISSUER ?? "";
const wsApi = api.replace(/^http/, "ws");
const connect = ["'self'", api, wsApi, oidc].filter(Boolean).join(" ");
const dev = process.env.NODE_ENV !== "production";

export const csp = [
  "default-src 'self'",
  // Next.js ships small inline bootstrap scripts; a nonce-based policy is tracked in docs/SECURITY.md.
  `script-src 'self' 'unsafe-inline'${dev ? " 'unsafe-eval'" : ""}`,
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "font-src 'self'",
  `connect-src ${connect}`,
  "object-src 'none'",
  "base-uri 'self'",
  "form-action 'self'",
  "frame-ancestors 'none'",
  ...(api.startsWith("https") ? ["upgrade-insecure-requests"] : []),
].join("; ");

const config: NextConfig = {
  output: "standalone",
  reactStrictMode: true,
  poweredByHeader: false,
  async headers() {
    return [
      {
        source: "/(.*)",
        headers: [
          { key: "Content-Security-Policy", value: csp },
          { key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains" },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
          { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(), payment=()" },
          { key: "Cross-Origin-Opener-Policy", value: "same-origin" },
          { key: "X-Robots-Tag", value: "noindex, nofollow" },
        ],
      },
    ];
  },
};

export default config;
