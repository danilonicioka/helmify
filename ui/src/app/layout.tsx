import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Helmify | Production-Grade Helm Chart Generator",
  description: "Automate the creation of TJPA-standard Helm charts from Kubernetes manifests with zero-delay probes and deterministic rollouts.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>
        {children}
      </body>
    </html>
  );
}
