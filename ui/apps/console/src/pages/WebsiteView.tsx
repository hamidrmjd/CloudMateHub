import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { Callout } from "@cloudmatehub/design-system/primitives";
import { directoryRequest } from "@/api/directory";
import { useAuthStore } from "@/stores/authStore";

const policy = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; form-action 'none'; base-uri 'none'">`;
const style = "<style>body{font:16px system-ui;margin:1rem;overflow-wrap:anywhere}pre{white-space:pre-wrap}</style>";

export default function WebsiteView() {
  const { websiteId = "" } = useParams();
  const userId = useAuthStore((state) => state.userId);
  const preview = useQuery({
    queryKey: ["assigned-website-preview", userId, websiteId],
    queryFn: () => directoryRequest<{ name: string; html: string }>(`/assigned-websites/${encodeURIComponent(websiteId)}/preview`),
    retry: false,
    gcTime: 0,
    staleTime: 0,
    refetchOnWindowFocus: false,
  });
  const title = preview.data?.name ?? "Website";

  return (
    <main className="flex h-dvh flex-col bg-background text-text-primary">
      <header className="flex flex-wrap items-center gap-x-5 gap-y-1 border-b border-border px-4 py-3">
        <Link to="/websites" className="underline">Websites</Link>
        <h1 className="font-semibold">{title}</h1>
        <span className="text-sm text-text-secondary">Read-only · Downloads disabled</span>
      </header>
      {preview.isPending && <p role="status" className="p-4">Loading website…</p>}
      {preview.error && <div className="p-4"><Callout variant="error">{preview.error.message}</Callout></div>}
      {preview.isSuccess && <iframe title={title} sandbox="" referrerPolicy="no-referrer"
        className="min-h-0 w-full flex-1 bg-white"
        srcDoc={`<!doctype html>${policy}${style}${preview.data.html}`} />}
    </main>
  );
}
