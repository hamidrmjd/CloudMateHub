import {
  Cog6ToothIcon,
  CommandLineIcon,
  CreditCardIcon,
} from "@heroicons/react/24/outline";
import { Navigate, useLocation } from "react-router-dom";
import SectionedLayout from "@/components/settings/SectionedLayout";
import { useNavSectionTitle } from "@/components/layout/navSections";
import { isCloud } from "@/env";
import { useAuthStore } from "@/stores/authStore";

/**
 * The namespace settings area, one section per URL under /settings. A link to the billing anchor
 * of the single page that came before lands on the Billing section.
 */
export default function SettingsLayout() {
  const isAdmin = useAuthStore((s) => s.isAdmin);
  const sectionTitle = useNavSectionTitle("/settings");
  const { pathname, hash } = useLocation();
  const sections = [
    { to: "general", label: "General", icon: Cog6ToothIcon },
    { to: "ssh", label: "SSH", icon: CommandLineIcon },
    ...(isAdmin ? [{ to: "https", label: "HTTPS & Certificates", icon: Cog6ToothIcon }, { to: "storage", label: "Storage & Recordings", icon: Cog6ToothIcon }, { to: "directories", label: "Directory connections", icon: Cog6ToothIcon }] : []),
    ...(isCloud()
      ? [{ to: "billing", label: "Billing", icon: CreditCardIcon }]
      : []),
  ];

  if (hash === "#billing" && pathname === "/settings" && isCloud()) {
    return <Navigate to="billing" replace />;
  }

  return (
    <SectionedLayout
      base="/settings"
      icon={<Cog6ToothIcon className="w-6 h-6" />}
      overline={sectionTitle}
      title="Settings"
      description="How this namespace is named, reached over SSH and billed"
      sections={sections}
    />
  );
}
