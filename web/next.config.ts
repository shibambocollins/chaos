import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Static export: no server needed, just HTML/CSS/JS reading pre-generated
  // JSON trace files from public/traces at runtime. Matches the project's
  // decision to replay recorded internal/sim traces rather than run a live
  // backend.
  output: "export",
};

export default nextConfig;
