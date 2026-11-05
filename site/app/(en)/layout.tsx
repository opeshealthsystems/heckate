import type { ReactNode } from "react";
import type { Viewport } from "next";
import "../globals.css";

export const viewport: Viewport = { width: "device-width", initialScale: 1 };

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
