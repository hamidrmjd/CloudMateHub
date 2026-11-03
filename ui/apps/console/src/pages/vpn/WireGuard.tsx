import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import QRCode from "qrcode";
import { ArrowDownTrayIcon, ShieldCheckIcon, QrCodeIcon, ArrowPathIcon } from "@heroicons/react/24/outline";
import { Button, Callout, Card } from "@cloudmatehub/design-system/primitives";
import PageHeader from "@/components/common/PageHeader";
import { useAuthStore } from "@/stores/authStore";
import { vpnRequest } from "./api";

type Assignment = { id: string; user_id: string; label: string; configuration: string; peer_id: string };
type Configuration = { name: string; peers: { id: string; name: string; allowed_ip: string; status: string }[] };
type Download = { file: string; fileName: string };
const inputClass = "w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-text-primary";

function AssignmentManager() {
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const directory = useQuery({ queryKey: ["vpn", "users", search, page], queryFn: () => vpnRequest<{ id: string; username: string; name: string }[]>(`/users?search=${encodeURIComponent(search)}&page=${page}`), retry: false });
  const users = directory.data ?? [];
  const [userId, setUserId] = useState("");
  const [configuration, setConfiguration] = useState("");
  const [peerId, setPeerId] = useState("");
  const [label, setLabel] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const catalog = useQuery({ queryKey: ["vpn", "catalog"], queryFn: () => vpnRequest<Configuration[]>("/catalog"), retry: false });
  const assignments = useQuery({ queryKey: ["vpn", "all-assignments"], queryFn: () => vpnRequest<Assignment[]>("/assignments?all=true") });
  const peers = catalog.data?.find((item) => item.name === configuration)?.peers ?? [];
  const assign = async () => {
    setBusy(true); setError("");
    try {
      await vpnRequest("/assignments", { method: "POST", body: JSON.stringify({ user_id: userId, configuration, peer_id: peerId, label }) });
      await assignments.refetch();
    } catch (e) { setError(e instanceof Error ? e.message : "Assignment failed"); }
    finally { setBusy(false); }
  };
  const unassign = async (id: string) => {
    setError("");
    try { await vpnRequest(`/assignments/${encodeURIComponent(id)}`, { method: "DELETE" }); await assignments.refetch(); }
    catch (e) { setError(e instanceof Error ? e.message : "Unassignment failed"); }
  };
  return <div className="space-y-5">
    <Callout variant="info">Create a peer under Interfaces, then assign it to a CloudMateHub user here. Each peer has one owner. Removing an assignment removes download access; restrict or delete the peer in WGDashboard to revoke an existing VPN connection.</Callout>
    {(error || catalog.error || assignments.error || directory.error) && <Callout variant="error">{error || catalog.error?.message || assignments.error?.message || directory.error?.message}</Callout>}
    <Card className="p-5 space-y-4">
      <div className="flex items-center justify-between"><h2 className="font-semibold">Assign a configuration</h2><Button variant="ghost" onClick={() => void catalog.refetch()} icon={<ArrowPathIcon className="w-4 h-4" />}>Refresh peers</Button></div>
      <div className="grid gap-4 md:grid-cols-2">
        <label className="text-sm space-y-2"><span>Find a user</span><input className={inputClass} value={search} onChange={(e) => { setSearch(e.target.value); setPage(1); setUserId(""); }} placeholder="Search usernames" /></label>
        <label className="text-sm space-y-2"><span>User</span><select className={inputClass} value={userId} onChange={(e) => setUserId(e.target.value)}><option value="">Select a user</option>{users.map((user) => <option key={user.id} value={user.id}>{user.username} — {user.name}</option>)}</select></label>
        <label className="text-sm space-y-2"><span>WireGuard interface</span><select className={inputClass} value={configuration} onChange={(e) => { setConfiguration(e.target.value); setPeerId(""); }}><option value="">Select an interface</option>{catalog.data?.map((item) => <option key={item.name}>{item.name}</option>)}</select></label>
        <label className="text-sm space-y-2"><span>Peer</span><select className={inputClass} value={peerId} onChange={(e) => setPeerId(e.target.value)}><option value="">Select a peer</option>{peers.map((peer) => <option key={peer.id} value={peer.id}>{peer.name || peer.id} · {peer.allowed_ip}</option>)}</select></label>
        <label className="text-sm space-y-2"><span>Display name</span><input className={inputClass} value={label} maxLength={255} onChange={(e) => setLabel(e.target.value)} placeholder="Work laptop" /></label>
      </div>
      <div className="flex gap-2"><Button variant="secondary" disabled={page === 1} onClick={() => { setPage((n) => n - 1); setUserId(""); }}>Previous users</Button><Button variant="secondary" disabled={users.length < 50} onClick={() => { setPage((n) => n + 1); setUserId(""); }}>More users</Button></div>
      <Button disabled={busy || !userId || !configuration || !peerId || !label.trim()} onClick={() => void assign()}>{busy ? "Assigning…" : "Assign to user"}</Button>
    </Card>
    <div className="space-y-3">{assignments.data?.map((item) => <Card key={item.id} className="p-4 flex flex-wrap gap-3 items-center justify-between"><div><strong>{item.label}</strong><p className="text-xs text-text-secondary">{item.configuration} · User {item.user_id}</p></div><Button variant="secondary" onClick={() => void unassign(item.id)}>Unassign download access</Button></Card>)}</div>
  </div>;
}

const managementSections = [
  { id: "interfaces", name: "Interfaces", route: "/" },
  { id: "create", name: "Add interface", route: "/new_configuration" },
  { id: "restore", name: "Restore", route: "/restore_configuration" },
  { id: "clients", name: "WG clients", route: "/clients" },
  { id: "peer-defaults", name: "Peer defaults", route: "/settings/peers_settings" },
  { id: "interface-settings", name: "Interface settings", route: "/settings/wireguard_settings" },
  { id: "webhooks", name: "Webhooks", route: "/webhooks" },
];

function Dashboard({ route, title }: { route: string; title: string }) {
  const [url, setUrl] = useState("");
  const [error, setError] = useState("");
  const [generation, setGeneration] = useState(0);
  useEffect(() => {
    let active = true;
    const open = async () => {
      try {
        const result = await vpnRequest<{ url: string }>("/session", { method: "POST" });
        if (active) { setUrl(result.url); setError(""); }
      } catch (e) { if (active) setError(e instanceof Error ? e.message : "Could not open administration"); }
    };
    void open();
    const timer = window.setInterval(() => void open(), 240000);
    return () => { active = false; window.clearInterval(timer); };
  }, [generation]);
  useEffect(() => {
    const expired = (event: MessageEvent) => {
      const frame = document.querySelector<HTMLIFrameElement>('iframe[title="WireGuard management"]');
      if (event.origin !== window.location.origin || event.source !== frame?.contentWindow) return;
      if (event.data?.type === "cmh-vpn-session-error") {
        setUrl("");
        setError("The management session or service is unavailable. Reload the workspace to reconnect using your Hub account.");
      }
    };
    window.addEventListener("message", expired);
    return () => window.removeEventListener("message", expired);
  }, []);
  return <div className="space-y-4">
    <div className="flex flex-wrap items-center justify-between gap-3"><p className="text-sm text-text-secondary">{title} · Managed with your Hub administrator account</p><Button variant="secondary" onClick={() => { setUrl(""); setGeneration((n) => n + 1); }}>Reload workspace</Button></div>
    {error && <Callout variant="error">{error}</Callout>}
    {url && <iframe key={generation} title="WireGuard management" src={`${url}#${route}`} className="w-full min-h-[75vh] rounded-2xl border border-border bg-surface" referrerPolicy="same-origin" sandbox="allow-scripts allow-same-origin allow-forms allow-downloads allow-popups" />}
  </div>;
}

export default function WireGuard() {
  const userId = useAuthStore((s) => s.userId);
  const isAdmin = useAuthStore((s) => s.isAdmin);
  const [tab, setTab] = useState(isAdmin ? "interfaces" : "mine");
  useEffect(() => { setTab(isAdmin ? "interfaces" : "mine"); }, [isAdmin, userId]);
  const section = managementSections.find((item) => item.id === tab);
  const [error, setError] = useState("");
  const [qr, setQr] = useState("");
  useEffect(() => { setQr(""); setError(""); }, [userId]);
  const status = useQuery({ queryKey: ["vpn", "status", userId], queryFn: () => vpnRequest<{ connected: boolean }>("/status"), retry: false });
  const assignments = useQuery({ queryKey: ["vpn", "assignments", userId], queryFn: () => vpnRequest<Assignment[]>("/assignments") });
  const download = async (item: Assignment, asQr = false) => {
    setError(""); setQr("");
    try {
      const result = await vpnRequest<Download>(`/assignments/${encodeURIComponent(item.id)}/configuration`);
      if (asQr) { setQr(await QRCode.toDataURL(result.file, { width: 320, margin: 2 })); return; }
      const url = URL.createObjectURL(new Blob([result.file], { type: "text/plain" }));
      const anchor = document.createElement("a"); anchor.href = url; anchor.download = result.fileName; anchor.click(); URL.revokeObjectURL(url);
    } catch (e) { setError(e instanceof Error ? e.message : "Could not load configuration"); }
  };
  return <div className="space-y-5">
    <PageHeader overline="VPN" title="WireGuard" description="Your private connection, managed in one place." icon={<ShieldCheckIcon className="w-6 h-6" />} />
    <Callout variant="info">Local preparation: management is available here. A reachable Ubuntu VPN host and endpoint are still needed before these configurations can provide a real remote connection.</Callout>
    {status.error && <Callout variant="error">{status.error.message}</Callout>}
    {status.data && !status.data.connected && <Callout variant="warning">WGDashboard is not ready. Ask an instance administrator to check the VPN service.</Callout>}
    <div role="tablist" aria-label="WireGuard sections" className="flex gap-2 flex-wrap border-b border-border pb-3">{[{ id: "mine", name: "My configurations" }, ...(isAdmin ? [...managementSections, { id: "assign", name: "User assignments" }] : [])].map((item) => <button key={item.id} role="tab" aria-selected={tab === item.id} className={`px-4 py-2 rounded-lg text-sm ${tab === item.id ? "bg-primary/15 text-primary" : "text-text-secondary hover:bg-hover-medium"}`} onClick={() => { setTab(item.id); setQr(""); if (item.id === "mine") void assignments.refetch(); }}>{item.name}</button>)}</div>
    {tab === "mine" && <div className="space-y-4">
      {(error || assignments.error) && <Callout variant="error">{error || assignments.error?.message}</Callout>}
      {assignments.isLoading && <p className="text-text-secondary">Loading your configurations…</p>}
      {assignments.data?.length === 0 && <Card className="p-10 text-center"><ShieldCheckIcon className="w-10 h-10 mx-auto text-primary mb-4" /><h2 className="text-lg font-semibold">Your VPN starts here</h2><p className="mt-2 text-sm text-text-secondary">No configuration has been assigned to you yet. Ask your instance administrator to create and assign your WireGuard peer.</p></Card>}
      {assignments.data?.map((item) => <Card key={item.id} className="p-5 flex flex-wrap gap-4 items-center justify-between"><div><h2 className="font-semibold">{item.label}</h2><p className="text-xs text-text-muted mt-1">{item.configuration} · Personal configuration</p></div><div className="flex gap-2"><Button variant="secondary" icon={<QrCodeIcon className="w-4 h-4" />} onClick={() => void download(item, true)}>QR code</Button><Button icon={<ArrowDownTrayIcon className="w-4 h-4" />} onClick={() => void download(item)}>Download .conf</Button></div></Card>)}
      {qr && <Card className="p-5 max-w-sm"><h2 className="font-semibold">Scan with your WireGuard app</h2><p className="text-xs text-text-secondary my-3">This code contains your private configuration. Keep it secret.</p><img src={qr} alt="Private WireGuard configuration QR code" className="w-full rounded-lg" /><Button className="mt-3" variant="secondary" onClick={() => setQr("")}>Hide QR code</Button></Card>}
    </div>}
    {isAdmin && section && <Dashboard route={section.route} title={section.name} />}
    {isAdmin && tab === "assign" && <AssignmentManager />}
  </div>;
}
