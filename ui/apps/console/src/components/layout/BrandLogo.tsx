import { cn } from "@cloudmatehub/design-system/cn";

export default function BrandLogo({ compact = false, className }: { compact?: boolean; className?: string }) {
  return (
    <span aria-hidden="true" className={cn("cmh-logo", className)}>
      <svg viewBox="0 0 48 48" fill="none" className="cmh-logo-icon">
        <rect width="48" height="48" rx="14" fill="currentColor" />
        <path d="M15 31h19a7 7 0 0 0 0-14 11 11 0 0 0-21-1 8 8 0 0 0 2 15Z" stroke="#042e2b" strokeWidth="2.5" />
        <path d="m18 21 4 4-4 4m8 0h5" stroke="#042e2b" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
      {!compact && <span className="cmh-wordmark">CloudMate<span>Hub</span></span>}
    </span>
  );
}
