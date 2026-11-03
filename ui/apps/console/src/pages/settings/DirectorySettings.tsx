import { useState } from "react";
import { Navigate, Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Button, Callout, Card } from "@cloudmatehub/design-system/primitives";
import { useAuthStore } from "@/stores/authStore";
import { directoryRequest, directoryInput, type DirectoryConfig, type DirectoryKind } from "@/api/directory";
export default function DirectorySettings() {
  const admin = useAuthStore((s) => s.isAdmin);
  const user = useAuthStore((s) => s.userId);
  const profiles = useQuery({ queryKey: ["directory-settings", user], queryFn: () => directoryRequest<DirectoryConfig[]>("/directories"), enabled: admin, retry: false });
  const [kind, setKind] = useState<DirectoryKind>("ad");
  if (!admin) return <Navigate to="/settings/general" replace />;
  return <div className="space-y-5"><h2 className="text-xl font-semibold">Directory connections</h2><p className="text-sm text-text-secondary">Connect Active Directory and OpenLDAP independently. Local administrator sign-in stays available. Manage Hub accounts in <Link className="underline" to="/users">User Management</Link>.</p>
    <div className="flex gap-3" role="tablist" aria-label="Directory type">{(["ad", "openldap"] as const).map((k) => <Button key={k} role="tab" aria-selected={kind === k} variant={kind === k ? "primary" : "secondary"} onClick={() => setKind(k)}>{k === "ad" ? "Active Directory" : "OpenLDAP"}</Button>)}</div>
    {profiles.error && <Callout variant="error">{profiles.error.message}</Callout>}
    {profiles.isPending && <p>Loading connections…</p>}
    {profiles.data?.filter((c) => c.kind === kind).map((c) => <Connection key={`${kind}-${c.id}`} initial={c} refresh={async () => { await profiles.refetch(); }} />)}
  </div>;
}
function Connection({ initial, refresh }: { initial: DirectoryConfig; refresh: () => Promise<void> }) {
  const [config, setConfig] = useState(initial);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const secure = location.protocol === "https:";
  const change = (key: keyof DirectoryConfig, value: string | boolean) => setConfig((old) => ({ ...old, [key]: value }));
  const run = async (action: "test" | "save" | "login" | "disable") => {
    setBusy(true); setMessage(""); setError("");
    try {
      const body = action === "login" ? { config, username, password } : { ...config, enabled: action === "disable" ? false : action === "save" ? true : config.enabled };
      await directoryRequest(`/directories/${config.kind}${action === "test" ? "/test" : action === "login" ? "/test-login" : ""}`, action === "test" || action === "login" ? "POST" : "PUT", body);
      setMessage(action === "test" ? "Connection, TLS, service account and base DN verified." : action === "login" ? "User sign-in and group access verified. No Hub account was created." : action === "disable" ? "Directory sign-in disabled. Existing accounts are preserved." : "Connection verified and enabled. Directory sign-in is now available.");
      if (action === "save" || action === "disable") { setConfig((c) => ({ ...c, enabled: action === "save", bind_password: "", password_set: action === "save" || c.password_set })); await refresh(); }
    } catch (e) { setError(e instanceof Error ? e.message : "Connection check failed"); }
    finally { setPassword(""); setBusy(false); }
  };
  const field = (key: keyof DirectoryConfig, label: string, placeholder = "", secret = false) => <label className="block text-sm" key={key}>{label}<input className={directoryInput} type={secret ? "password" : "text"} disabled={busy || (secret && !secure)} autoComplete={secret ? "new-password" : "off"} placeholder={placeholder} value={String(config[key] ?? "")} onChange={(e) => change(key, e.target.value)} /></label>;
  return <Card className="p-5 space-y-5">
    <p className="font-medium">{config.label} — {config.enabled ? "Enabled" : "Disabled"}</p>
    {!secure && <Callout variant="warning">Enable trusted HTTPS in HTTPS &amp; Certificates before entering credentials or testing a connection.</Callout>}
    {error && <Callout variant="error">{error}</Callout>}{message && <Callout variant="success">{message}</Callout>}
    <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); void run("save"); }}>
      {field("label", "Connection name")}{field("url", "Server address", "ldaps://directory.example.com:636")}
      <p className="text-sm text-text-secondary">LDAPS and mandatory StartTLS are supported. Certificates and server names are always verified.</p>
      {field("base_dn", "Base DN", "DC=example,DC=com")}{field("bind_dn", "Read-only service account", config.kind === "ad" ? "service@example.com or account DN" : "cn=reader,dc=example,dc=com")}{field("bind_password", "Service account password", config.password_set ? "Saved — leave blank to keep for the same server and account" : "", true)}
      <label className="block text-sm">CA certificate PEM (optional)<textarea className={directoryInput} rows={4} value={config.ca_pem} onChange={(e) => change("ca_pem", e.target.value)} /></label>
      {field("required_group_dn", "Required group DN (optional)")}
      <p className="text-sm text-text-secondary">Active Directory supports nested group membership. OpenLDAP supports member, uniqueMember and memberUid groups.</p>
      <details><summary className="cursor-pointer text-sm">User search and attribute mappings</summary><div className="mt-3 space-y-4">{field("user_filter", "User filter (include {username})")}{field("username_attribute", "Username attribute")}{field("id_attribute", "Permanent user ID attribute")}{field("email_attribute", "Email attribute")}{field("name_attribute", "Display name attribute")}<p className="text-sm text-text-secondary">Use objectGUID for AD or entryUUID for OpenLDAP. Users need a valid email. Changing server, base DN or permanent ID mapping creates a new connection identity; existing accounts are preserved and are never merged by email.</p></div></details>
      <label className="flex gap-2 text-sm"><input type="checkbox" checked={config.provision_accounts} onChange={(e) => change("provision_accounts", e.target.checked)} />Create Hub accounts on first successful directory sign-in</label>
      <p className="text-sm text-text-secondary">New accounts are members without admin privileges or device access. Add them through Team to grant access. No directory accounts are imported until they sign in.</p>
      <div className="flex flex-wrap gap-3"><Button type="button" variant="secondary" disabled={busy || !secure} onClick={() => void run("test")}>Test connection</Button><Button type="submit" disabled={busy || !secure}>Verify and enable</Button>{config.enabled && <Button type="button" variant="secondary" disabled={busy} onClick={() => void run("disable")}>Disable connection</Button>}</div>
    </form>
    <details><summary className="cursor-pointer text-sm">Check a user sign-in</summary><form className="mt-4 space-y-4" onSubmit={(e) => { e.preventDefault(); void run("login"); }}><p className="text-sm text-text-secondary">Checks credentials, user attributes and required group without creating an account.</p><label className="block text-sm">Directory username<input className={directoryInput} value={username} autoComplete="off" onChange={(e) => setUsername(e.target.value)} /></label><label className="block text-sm">Directory password<input className={directoryInput} type="password" autoComplete="new-password" disabled={!secure} value={password} onChange={(e) => setPassword(e.target.value)} /></label><Button type="submit" disabled={busy || !secure || !username || !password}>Test user sign-in</Button></form></details>
  </Card>;
}
