import "./globals.css";

export const metadata = {
  title: "SkillRoom",
  description: "React skill evaluation platform with runner-backed execution.",
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
