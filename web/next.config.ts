import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Static export: no server needed, just HTML/CSS/JS reading pre-generated
  // JSON trace files from public/traces at runtime. Matches the project's
  // decision to replay recorded internal/sim traces rather than run a live
  // backend.
  output: "export",

  // No floating dev-tools badge over the workspace. It sits bottom-left,
  // exactly on top of the simulation controls.
  devIndicators: false,
};

export default nextConfig;
