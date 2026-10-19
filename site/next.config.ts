import type { NextConfig } from "next";

// Static export: the public site is plain files on a separate host with no path to ERP data.
const config: NextConfig = {
  output: "export",
  trailingSlash: true,
  reactStrictMode: true,
  poweredByHeader: false,
  images: { unoptimized: true },
};

export default config;
