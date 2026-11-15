import type { Metadata } from "next";
import "./globals.css";
export const metadata: Metadata = { title: "Anime Intelligence", description: "Your anime library and taste intelligence" };
export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) { return <html lang="en"><body>{children}</body></html>; }
