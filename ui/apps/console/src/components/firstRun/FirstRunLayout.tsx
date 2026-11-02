import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { XMarkIcon } from "@heroicons/react/24/outline";
import SessionMenu from "@/components/layout/SessionMenu";
import ScreenIntro from "@/components/layout/ScreenIntro";
import BrandLogo from "@/components/layout/BrandLogo";
import BrandStory from "@/components/layout/BrandStory";
import WindowControls from "@/components/layout/WindowControls";
import { useIsDesktop } from "@/hooks/useIsDesktop";

interface FirstRunLayoutProps {
  eyebrow: string;
  title?: string;
  lead?: string;
  signedIn: boolean;
  inConsole: boolean;
  children: ReactNode;
}

export default function FirstRunLayout({ eyebrow, title = "Build your connected workspace", lead = "Create your account, connect a device, and open your first session. Let's get you set up.", signedIn, inConsole, children }: FirstRunLayoutProps) {
  const isDesktop = useIsDesktop();
  return <div className="cmh-public-shell">
    <header className="cmh-public-header" data-tauri-drag-region>
      <span aria-label="CloudMateHub"><BrandLogo /></span>
      <div className="flex items-center gap-3">{signedIn && <SessionMenu placement={isDesktop ? "tabStrip" : "tabStripAvatar"} />}<WindowControls /></div>
    </header>
    <main id="main-content" tabIndex={-1} className="cmh-entry-grid cmh-setup-grid outline-none">
      <BrandStory />
      <section className="cmh-setup-card page-enter">
        {inConsole && <Link to="/devices" aria-label="Skip for now" title="Skip for now" className="cmh-skip"><XMarkIcon className="w-4 h-4" /></Link>}
        <ScreenIntro eyebrow={eyebrow} title={title} lead={lead} />
        {children}
      </section>
    </main>
  </div>;
}
