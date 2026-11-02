import { Outlet } from "react-router-dom";
import { cn } from "@cloudmatehub/design-system/cn";
import { isCommunity } from "@/env";
import AuthFooterLinks from "@/components/common/AuthFooterLinks";
import { columnWidths, type ColumnWidth } from "./columnWidths";
import BrandLogo from "./BrandLogo";
import BrandStory from "./BrandStory";

export default function AuthLayout({ width = "md" }: { width?: ColumnWidth }) {
  return <div className="cmh-public-shell">
    <header className="cmh-public-header"><span aria-label="CloudMateHub"><BrandLogo /></span><span className="cmh-edition">YOUR CLOUD OPERATIONS WORKSPACE</span></header>
    <main className="cmh-entry-grid">
      <BrandStory />
      <div className={cn("cmh-form-column", columnWidths[width])}>
        <div data-testid="auth-card" className="cmh-form-card"><Outlet /></div>
        {isCommunity() && <AuthFooterLinks />}
      </div>
    </main>
  </div>;
}
