import type { Metadata } from "next";
import { JetBrains_Mono } from "next/font/google";
import { config } from "@/lib/config";
import { Header } from "@/components/header";
import { Footer } from "@/components/footer";
import "./globals.css";

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: `${config.name} - ${config.tagline}`,
  description: config.description,
  openGraph: {
    title: `${config.name} - ${config.tagline}`,
    description: config.description,
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: `${config.name} - ${config.tagline}`,
    description: config.description,
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className={jetbrainsMono.variable}>
      <body className="font-mono antialiased">
        <Header />
        <main>{children}</main>
        <Footer />
      </body>
    </html>
  );
}
