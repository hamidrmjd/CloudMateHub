import { useState } from "react";
import { Link } from "react-router-dom";
import { CommandLineIcon, ArrowPathIcon } from "@heroicons/react/24/outline";
import { Button, Callout, Card } from "@cloudmatehub/design-system/primitives";
import PageHeader from "@/components/common/PageHeader";
import ConnectModal from "@/components/ConnectModal";
import OnlineDot from "@/components/common/OnlineDot";
import { useDevices, type NormalizedDevice } from "@/hooks/useDevices";
import { useHasPermission } from "@/hooks/useHasPermission";
import { useNamespace } from "@/hooks/useNamespaceRequests";
import { useNamespaceLegacy } from "@/hooks/useNamespaceLegacy";
import { useDebouncedValue } from "@/hooks/useDebouncedValue";
import { useAuthStore } from "@/stores/authStore";
import { useTerminalStore } from "@/stores/terminalStore";
import { buildSshid } from "@/utils/sshid";

const PAGE_SIZE = 12;

/** Client entry point into the existing namespace-scoped, recorded SSH bridge. */
export default function BrowserConsole() {
  const tenant = useAuthStore((s) => s.tenant);
  const user = useAuthStore((s) => s.userId);
  // Clear selected devices and pagination when the acting identity changes.
  return <ConsoleWorkspace key={`${user}:${tenant}`} tenant={tenant ?? ""} />;
}

function ConsoleWorkspace({ tenant }: { tenant: string }) {
  const canConnect = useHasPermission("device:connect");
  const admin = useAuthStore((s) => s.isAdmin);
  const legacy = useNamespaceLegacy();
  const { namespace } = useNamespace(tenant);
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [target, setTarget] = useState<NormalizedDevice | null>(null);
  const debouncedSearch = useDebouncedValue(search, 250);
  const devices = useDevices({ status: "accepted", page, perPage: PAGE_SIZE, search: debouncedSearch, enabled: !!tenant && canConnect, refetchInterval: 15000 });
  const sessions = useTerminalStore((s) => s.sessions);
  const restore = useTerminalStore((s) => s.restore);
  const ownSessions = sessions.filter((s) => s.tenant === tenant);
  const needsHttps = legacy === false && !window.isSecureContext;
  const open = (device: NormalizedDevice) => {
    if (!canConnect || !device.online || needsHttps) return;
    const existing = ownSessions.find((s) => s.deviceUid === device.uid && s.connectionStatus !== "disconnected");
    if (existing) restore(existing.id);
    else setTarget(device);
  };
  return <div className="space-y-5">
    <PageHeader overline="SSH" title="Browser Console" description="Open an SSH terminal for your connected devices, directly in your browser." icon={<CommandLineIcon className="w-6 h-6" />} />
    {!canConnect ? <Callout variant="info">Your namespace role allows viewing devices. Ask a namespace administrator for Operator access to open an SSH console.</Callout> : <>
      <Callout variant="info">Choose an online device and enter its Linux username. Your SSH identity and access rules apply. Terminal sessions are recorded and remain open while you move around the dashboard.</Callout>
      {needsHttps && <Callout variant="warning">This namespace uses browser SSH keys, which require HTTPS or localhost. {admin ? <Link className="underline" to="/settings/https">Set up HTTPS &amp; Certificates</Link> : "Ask your administrator to enable trusted HTTPS, then open the dashboard using its HTTPS address."}</Callout>}
      {ownSessions.length > 0 && <section className="space-y-3" aria-label="Open browser terminals"><h2 className="font-semibold">Your open terminals</h2><div className="grid gap-3 md:grid-cols-2">{ownSessions.map((s) => <Card key={s.id} className="p-4 flex gap-3 items-center justify-between"><div><p className="font-medium">{s.username}@{s.deviceName}</p><p className="text-xs text-text-secondary capitalize">{s.connectionStatus}</p></div><Button variant="secondary" onClick={() => restore(s.id)}>Resume terminal</Button></Card>)}</div></section>}
      <div className="flex gap-3 items-end flex-wrap"><label className="block text-sm flex-1 min-w-48">Find a device<input className="block mt-1 w-full rounded-lg border border-border bg-background px-3 py-2" placeholder="Search device names" value={search} onChange={(e) => { setSearch(e.target.value); setPage(1); }} /></label><Button variant="secondary" icon={<ArrowPathIcon className="w-4 h-4" />} onClick={() => void devices.refetch()}>Refresh devices</Button></div>
      {devices.error && <Callout variant="error">Could not load devices. Check your connection and try Refresh devices.</Callout>}
      {devices.isLoading && <p className="text-sm text-text-secondary">Loading your devices…</p>}
      {!devices.isLoading && !devices.error && devices.devices.length === 0 && <Card className="p-8 text-center"><h2 className="font-semibold">{search ? "No matching devices" : "No connected devices yet"}</h2><p className="mt-2 text-sm text-text-secondary">{search ? "Try another device name." : "A device must be enrolled and accepted in your namespace before you can open its console."}</p><Link className="inline-block mt-3 underline text-sm" to="/devices">View devices</Link></Card>}
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{devices.devices.map((device) => <Card key={device.uid} className="p-5 space-y-4"><div className="flex gap-2 items-center"><OnlineDot online={!!device.online} /><h2 className="font-semibold truncate">{device.name}</h2><span className="ml-auto text-xs text-text-secondary">{device.online ? "Online" : "Offline"}</span></div><p className="text-xs text-text-secondary">{device.info?.pretty_name ?? "Linux device"}</p><Button fullWidth disabled={!device.online || needsHttps} onClick={() => open(device)} icon={<CommandLineIcon className="w-4 h-4" />}>{ownSessions.some((s) => s.deviceUid === device.uid && s.connectionStatus !== "disconnected") ? "Resume console" : "Open console"}</Button>{!device.online && <p className="text-xs text-text-secondary">The device must come online before connecting.</p>}</Card>)}</div>
      <div className="flex gap-3 items-center"><Button variant="secondary" disabled={page === 1} onClick={() => setPage(page - 1)}>Previous</Button><span className="text-sm">Page {page}</span><Button variant="secondary" disabled={page * PAGE_SIZE >= devices.totalCount} onClick={() => setPage(page + 1)}>Next</Button></div>
      {target && <ConnectModal open onClose={() => setTarget(null)} deviceUid={target.uid} deviceName={target.name} sshid={namespace?.name ? buildSshid(namespace.name, target.name) : target.uid} />}
    </>}
  </div>;
}
