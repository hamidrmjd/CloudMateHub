import { useRef, useState } from "react";
import { Navigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Button, Callout, Card } from "@cloudmatehub/design-system/primitives";
import { useAuthStore } from "@/stores/authStore";

const actions: Record<string, string> = { validated: "Certificate validated", "activation-pending": "Waiting for HTTPS confirmation", confirmed: "HTTPS confirmed", restored: "Previous certificate restored", "manual-rollback": "Activation cancelled", "restart-rollback": "Previous configuration preserved after restart", "confirmation-timeout-rollback": "Confirmation timed out; previous configuration preserved", "state-error-use-local-recovery": "Storage needs attention; open local recovery" };
type Certificate = { id: string; expires: string; trust: string };
type Status = { host: string; https_url: string; active: string; previous: string; pending: string; deadline: string; secure_upload: boolean; certificates: Certificate[]; history: { at: string; action: string; revision?: string }[] };
async function request<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(`/api/https/${path}`, { method: body ? "POST" : "GET", headers: { Authorization: `Bearer ${useAuthStore.getState().token ?? ""}`, "Content-Type": "application/json" }, body: body ? JSON.stringify(body) : undefined, cache: "no-store" });
  const data = await response.json() as T & { error?: string };
  if (!response.ok) throw new Error(data.error ?? "HTTPS controller is unavailable");
  return data;
}
export default function HttpsSettings() {
  const admin = useAuthStore((s) => s.isAdmin);
  const user = useAuthStore((s) => s.userId);
  const status = useQuery({ queryKey: ["https-status", user], queryFn: () => request<Status>("status"), enabled: admin, refetchInterval: 3000, retry: false });
  const [cert, setCert] = useState<File | null>(null);
  const [key, setKey] = useState<File | null>(null);
  const [ca, setCa] = useState<File | null>(null);
  const [candidate, setCandidate] = useState<Certificate | null>(null);
  const [confirmation, setConfirmation] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const form = useRef<HTMLFormElement>(null);
  if (!admin) return <Navigate to="/settings/general" replace />;
  const active = status.data?.certificates.find((item) => item.id === status.data?.active);
  const days = active ? Math.ceil((Date.parse(active.expires) - Date.now()) / 86400000) : null;
  const stage = async () => {
    if (!cert || !key) return;
    setBusy(true); setError("");
    try {
      setCandidate(await request<Certificate>("candidates", { certificate: await cert.text(), private_key: await key.text(), ca: ca ? await ca.text() : "" }));
      setCert(null); setKey(null); setCa(null); form.current?.reset();
      await status.refetch();
    } catch (e) { setError(e instanceof Error ? e.message : "Certificate validation failed"); }
    finally { setBusy(false); }
  };
  const activate = async () => {
    if (!candidate) return;
    setBusy(true); setError("");
    try { const result = await request<{ confirmation_url: string }>("activate", { id: candidate.id }); setConfirmation(result.confirmation_url); await status.refetch(); }
    catch (e) { setError(e instanceof Error ? e.message : "Activation failed"); }
    finally { setBusy(false); }
  };
  const restore = async () => {
    setBusy(true); setError("");
    try { await request("restore", {}); setConfirmation(""); await status.refetch(); }
    catch (e) { setError(e instanceof Error ? e.message : "Restore failed"); }
    finally { setBusy(false); }
  };
  return <div className="space-y-5">
    <h2 className="text-xl font-semibold">HTTPS &amp; Certificates</h2>
    <p className="text-sm text-text-secondary">Instance-wide certificate management. Upload, validate, test, then confirm the new HTTPS address within 60 seconds. Unconfirmed changes roll back automatically.</p>
    {(error || status.error) && <Callout variant="error">{error || status.error?.message}</Callout>}
    {status.data && <Card className="p-5 space-y-3">
      <p>Address: <strong>{status.data.host}</strong></p>
      <p>{active ? `HTTPS active · Expires ${new Date(active.expires).toLocaleDateString()} · ${active.trust}` : "HTTPS has not been confirmed yet. The existing dashboard remains available."}</p>
      {active && <a className="text-primary underline" href={status.data.https_url}>Open HTTPS dashboard</a>}
      {days !== null && days <= 30 && <Callout variant="warning">This certificate expires in {days} days. Upload a replacement before it expires.</Callout>}
      {status.data.pending && <Callout variant="warning">Activation is awaiting confirmation until {new Date(status.data.deadline).toLocaleTimeString()}. Closing the confirmation page will leave the change unconfirmed and restore the previous configuration.</Callout>}
      <Button variant="secondary" disabled={busy || (!status.data.pending && !status.data.previous)} onClick={() => void restore()}>Restore previous / cancel activation</Button>
    </Card>}
    {status.data && !status.data.secure_upload && <Callout variant="info">Certificate keys must be uploaded over trusted HTTPS or through the server-local recovery page at <a className="underline" href="http://localhost:9080" target="_blank" rel="noreferrer">http://localhost:9080</a>. For a remote server, reach that page through an SSH tunnel. Run ./deployments/cloudmatehub recovery on the server for the private access link. Recovery remains available independently of the application gateway.</Callout>}
    <Callout variant="info">If using a private CA, install its trust certificate on browsers and device agents before activation. Uploading the CA here validates the chain; it does not install trust on those devices. Keep the HTTPS state volume and its encryption key together in protected backups.</Callout>
    <form ref={form} className="space-y-4" onSubmit={(e) => { e.preventDefault(); void stage(); }}>
      <label className="block text-sm">Certificate and intermediate chain (PEM)<input className="block mt-2" type="file" accept=".pem,.crt,.cer" onChange={(e) => setCert(e.target.files?.[0] ?? null)} /></label>
      <label className="block text-sm">Private key (PEM)<input className="block mt-2" type="file" accept=".pem,.key" onChange={(e) => setKey(e.target.files?.[0] ?? null)} /></label>
      <label className="block text-sm">Private CA bundle (optional)<input className="block mt-2" type="file" accept=".pem,.crt" onChange={(e) => setCa(e.target.files?.[0] ?? null)} /></label>
      <Button type="submit" disabled={busy || !cert || !key || !status.data?.secure_upload}>Validate certificate</Button>
    </form>
    {!!status.data?.certificates.length && <label className="block text-sm">Saved validated certificates<select className="block mt-2 rounded border border-border bg-background p-2" value={candidate?.id ?? ""} onChange={(e) => setCandidate(status.data?.certificates.find((item) => item.id === e.target.value) ?? null)}><option value="">Select a certificate</option>{status.data.certificates.filter((item) => item.id !== status.data?.active).map((item) => <option value={item.id} key={item.id}>{new Date(item.expires).toLocaleDateString()} · {item.trust} · {item.id.slice(0, 8)}</option>)}</select></label>}
    {candidate && <Card className="p-5 space-y-3"><p>Certificate validated · Expires {new Date(candidate.expires).toLocaleDateString()} · {candidate.trust}</p><Button disabled={busy || !!status.data?.pending || !status.data?.secure_upload} onClick={() => void activate()}>Test and activate for 60 seconds</Button></Card>}
    {confirmation && status.data?.pending && <Callout variant="warning"><a className="underline" href={confirmation} target="_blank" rel="noreferrer">Open HTTPS confirmation</a> and choose Keep HTTPS before the deadline. No confirmation means automatic rollback.</Callout>}
    {!!status.data?.history.length && <Card className="p-5"><h3 className="font-semibold mb-3">Restore history</h3><ul className="space-y-2 text-sm">{[...status.data.history].reverse().map((item, i) => <li key={`${item.at}-${i}`}>{new Date(item.at).toLocaleString()} · {actions[item.action] ?? item.action}</li>)}</ul></Card>}
  </div>;
}
