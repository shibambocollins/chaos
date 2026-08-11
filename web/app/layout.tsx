import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Chaos — Raft Cluster Visualizer",
  description: "Replaying deterministic Raft simulation traces from internal/sim.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
