import { cn } from "@cloudmatehub/design-system/cn";
import BrandLogo from "./BrandLogo";

export default function LogoMark({ full, className }: { full: boolean; className?: string }) {
  return <span aria-hidden="true" className={cn("block overflow-hidden transition-[width] duration-200", full ? "w-[190px]" : "w-[34px]", className)}><BrandLogo compact={!full} /></span>;
}
