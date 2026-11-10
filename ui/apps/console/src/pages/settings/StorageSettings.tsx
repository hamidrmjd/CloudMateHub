import { useEffect, useState } from "react";
import { Navigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Button, Callout, Card } from "@cloudmatehub/design-system/primitives";
import { useAuthStore } from "@/stores/authStore";

type StorageConfig = { id?: string; kind: "local" | "nfs" | "s3"; path?: string; endpoint?: string; region?: string; bucket?: string; prefix?: string; access_key?: string; secret_key?: string; session_token?: string; credentials_set?: boolean };
type Status = { config: StorageConfig; recording_required: boolean; retention_days?: number; last_error: string; last_success: string; pending: number; capture_buffer: string };
async function request<T>(path: string, method = "GET", body?: unknown): Promise<T> {
  const response = await fetch(`/api/storage${path}`, { method, headers: { Authorization: `Bearer ${useAuthStore.getState().token ?? ""}`, "Content-Type": "application/json" }, body: body ? JSON.stringify(body) : undefined, cache: "no-store" });
  const data = await response.json() as T & { error?: string };
  if (!response.ok) throw new Error(data.error ?? "Storage is unavailable");
  return data;
}
const inputClass = "block mt-1 w-full rounded border border-border bg-background px-3 py-2 text-sm";
export default function StorageSettings() {
  const admin = useAuthStore((s) => s.isAdmin);
  const user = useAuthStore((s) => s.userId);
  const status = useQuery({ queryKey: ["storage-settings", user], queryFn: () => request<Status>(""), enabled: admin, retry: false, refetchInterval: 15000 });
  const [config, setConfig] = useState<StorageConfig>({ kind: "local", path: "/recordings/files" });
  const [loaded, setLoaded] = useState(false);
  const [retention, setRetention] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  useEffect(() => { if (status.data && !loaded) { setConfig(status.data.config); setLoaded(true); } }, [status.data, loaded]);
  if (!admin) return <Navigate to="/settings/general" replace />;
  const field = (key: keyof StorageConfig, value: string) => setConfig((old) => ({ ...old, [key]: value }));
  const run = async (save: boolean) => {
    setBusy(true); setError(""); setMessage("");
    try { await request(save ? "" : "/test", save ? "PUT" : "POST", config); setMessage(save ? "Storage verified and saved. New completed recordings use this destination; earlier recordings stay accessible." : "Storage verified: write, read and cleanup succeeded."); if (save) { setConfig((c) => ({ ...c, access_key: "", secret_key: "", session_token: "" })); const refreshed = await status.refetch(); if (refreshed.data) setConfig(refreshed.data.config); } }
    catch (e) { setError(e instanceof Error ? e.message : "Storage check failed"); }
    finally { setBusy(false); }
  };
  const saveRetention = async (days: number) => {
    setBusy(true); setError(""); setMessage("");
    try {
      await request("/retention", "PUT", { days });
      await status.refetch();
      setRetention(null);
      setMessage(days === 0 ? "Automatic recording cleanup disabled." : "Recording retention saved.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Retention settings could not be saved");
    } finally { setBusy(false); }
  };
  return <div className="space-y-5">
    <h2 className="text-xl font-semibold">Storage &amp; Recordings</h2>
    <p className="text-sm text-text-secondary">Record every SSH shell and command connection, from a browser or terminal. Play or download recordings from Sessions. SFTP connections keep connection history without recording file contents.</p>
    <Callout variant="info">Completed recordings are archived to your selected destination. Session metadata and captured output remain in PostgreSQL as a recovery copy. If archive storage is unavailable, connections continue and archiving retries automatically. Changing storage does not move or erase existing recordings.</Callout>
    {(error || status.error) && <Callout variant="error">{error || status.error?.message}</Callout>}
    {message && <Callout variant="success">{message}</Callout>}
    {status.data?.last_error && <Callout variant="warning">{status.data.last_error}</Callout>}
    <Card className="p-5 space-y-2"><p className="font-medium">Session recording is required for all connections</p><p className="text-sm text-text-secondary">Terminal output, command output, errors, timing and terminal resizing are captured. Password keystrokes and SFTP file contents are excluded. Anything printed by a remote program can appear in a recording.</p>{status.data && <p className="text-sm">Archive check: {status.data.last_success && !status.data.last_success.startsWith("0001") ? new Date(status.data.last_success).toLocaleString() : "Waiting for the first check"}</p>}</Card>
    <Card className="p-5 space-y-3">
      <h3 className="font-medium">Recording retention</h3>
      <p className="text-sm text-text-secondary">Automatically delete completed recordings after this many days. Session history is preserved. Enter 0 to keep recordings indefinitely under this policy.</p>
      <form className="space-y-3" onSubmit={(event) => {
        event.preventDefault();
        const days = Number(retention ?? status.data?.retention_days ?? 0);
        void saveRetention(days);
      }}>
        <label className="block text-sm max-w-xs">Retention days
          <input className={inputClass} type="number" min={0} max={3650} step={1} required disabled={busy || !status.data}
            value={retention ?? String(status.data?.retention_days ?? 0)} onChange={(event) => setRetention(event.target.value)} />
        </label>
        <Button type="submit" disabled={busy || !status.data || retention === null}>Save retention</Button>
      </form>
    </Card>
    <form className="space-y-4" onSubmit={(event) => { event.preventDefault(); void run(true); }}>
      <label className="block text-sm">Destination<select className={inputClass} value={config.kind} onChange={(e) => setConfig({ kind: e.target.value as StorageConfig["kind"], path: e.target.value === "nfs" ? "/mnt/recordings" : "/recordings/files", region: "us-east-1" })}><option value="local">Local disk</option><option value="s3">S3 / S3-compatible storage</option><option value="nfs">NFS</option></select></label>
      {config.kind !== "s3" ? <>
        <label className="block text-sm">Recording folder<input className={inputClass} required value={config.path ?? ""} onChange={(e) => field("path", e.target.value)} /></label>
        <p className="text-sm text-text-secondary">{config.kind === "local" ? "Choose a folder under /recordings. This is a persistent server volume; an operator can map it to a chosen host disk." : "An operator must mount the NFS share at /mnt/recordings on the server first. An unavailable NFS mount is refused so recordings cannot silently end up on a local disk."}</p>
      </> : <>
        {location.protocol !== "https:" && <Callout variant="warning">Enable trusted HTTPS before entering S3 credentials.</Callout>}
        <label className="block text-sm">S3 endpoint<input className={inputClass} type="url" placeholder="https://s3.us-east-1.amazonaws.com" value={config.endpoint ?? ""} onChange={(e) => field("endpoint", e.target.value)} /></label>
        <label className="block text-sm">Region<input className={inputClass} required value={config.region ?? "us-east-1"} onChange={(e) => field("region", e.target.value)} /></label>
        <label className="block text-sm">Bucket<input className={inputClass} required value={config.bucket ?? ""} onChange={(e) => field("bucket", e.target.value)} /></label>
        <label className="block text-sm">Object prefix<input className={inputClass} placeholder="cloudmatehub" value={config.prefix ?? ""} onChange={(e) => field("prefix", e.target.value)} /></label>
        <label className="block text-sm">Access key<input className={inputClass} autoComplete="off" disabled={location.protocol !== "https:"} placeholder={config.credentials_set ? "Saved — leave blank to keep" : ""} value={config.access_key ?? ""} onChange={(e) => field("access_key", e.target.value)} /></label>
        <label className="block text-sm">Secret key<input className={inputClass} type="password" autoComplete="new-password" disabled={location.protocol !== "https:"} placeholder={config.credentials_set ? "Saved — leave blank to keep" : ""} value={config.secret_key ?? ""} onChange={(e) => field("secret_key", e.target.value)} /></label>
        <label className="block text-sm">Session token (optional)<input className={inputClass} type="password" autoComplete="off" disabled={location.protocol !== "https:"} value={config.session_token ?? ""} onChange={(e) => field("session_token", e.target.value)} /></label>
        <p className="text-sm text-text-secondary">Supports Amazon S3 and compatible providers using HTTPS and path-style access. Use a private bucket with read, write and delete permission restricted to this prefix. Credentials are encrypted on the server and never returned by this page.</p>
      </>}
      <div className="flex gap-3"><Button type="button" variant="secondary" disabled={busy || !loaded} onClick={() => void run(false)}>Test storage</Button><Button type="submit" disabled={busy || !loaded}>Verify and save</Button></div>
    </form>
  </div>;
}
