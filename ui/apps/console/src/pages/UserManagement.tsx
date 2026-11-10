import { useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Button, Callout, Card } from "@cloudmatehub/design-system/primitives";
import CreateNamespaceDialog from "@/components/common/CreateNamespaceDialog";
import { useAuthStore } from "@/stores/authStore";
import { directoryRequest, directoryInput } from "@/api/directory";
type User = { id: string; username: string; name: string; email: string; origin: string; admin: boolean; status: string; awaiting_approval: boolean };
const source = (origin: string) => origin === "ad" ? "Active Directory" : origin === "ldap" ? "OpenLDAP" : origin.toLowerCase() === "saml" ? "SAML" : "Local";
export default function UserManagement() {
  const admin = useAuthStore((s) => s.isAdmin);
  const user = useAuthStore((s) => s.userId);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [kind, setKind] = useState("local");
  const [createGroup, setCreateGroup] = useState(false);
  const [create, setCreate] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [form, setForm] = useState({ name: "", username: "", email: "", password: "" });
  const users = useQuery({ queryKey: ["managed-users", user, page, search], queryFn: () => directoryRequest<{ users: User[]; total: number; per_page: number }>(`/user-management/users?page=${page}&search=${encodeURIComponent(search)}`), enabled: admin, retry: false });
  if (!admin) return <Navigate to="/dashboard" replace />;
  const submit = async () => {
    setBusy(true); setError(""); setMessage("");
    try { await directoryRequest("/user-management/users", "POST", form); setForm({ name: "", username: "", email: "", password: "" }); setCreate(false); setMessage("Local account created. Use Team to grant access to a namespace."); await users.refetch(); }
    catch (e) { setError(e instanceof Error ? e.message : "Account creation failed"); }
    finally { setBusy(false); }
  };
  return <div className="space-y-5"><h1 className="text-2xl font-semibold">User Management</h1><p className="text-sm text-text-secondary">Manage Local, Active Directory and OpenLDAP accounts. Namespace permissions and invitations are managed in <Link className="underline" to="/team">Team</Link>.</p>
    {message && <Callout variant="success">{message}</Callout>}{(error || users.error) && <Callout variant="error">{error || users.error?.message}</Callout>}
    <div className="flex gap-3"><Button onClick={() => setCreate(!create)}>Add user</Button><Button variant="secondary" onClick={() => setCreateGroup(true)}>Add group</Button><Link className="underline self-center text-sm" to="/settings/directories">Directory connections</Link></div>
    {createGroup && <CreateNamespaceDialog open onClose={() => setCreateGroup(false)} />}
    {create && <Card className="p-5 space-y-4"><label className="block text-sm">Account type<select className={directoryInput} value={kind} onChange={(e) => { setKind(e.target.value); setForm((f) => ({ ...f, password: "" })); }}><option value="local">Local</option><option value="ad">Active Directory</option><option value="openldap">OpenLDAP</option></select></label>
      {kind !== "local" ? <p className="text-sm">Configure and test the {kind === "ad" ? "Active Directory" : "OpenLDAP"} connection in <Link className="underline" to="/settings/directories">Directory connections</Link>, then enable account creation on first sign-in. Directory passwords stay managed by your directory.</p> : <form className="space-y-4" onSubmit={(e) => { e.preventDefault(); void submit(); }}>
        {location.protocol !== "https:" && <Callout variant="warning">This connection uses HTTP. The new account password will travel without encryption.</Callout>}
        {(["name", "username", "email", "password"] as const).map((key) => <label className="block text-sm capitalize" key={key}>{key}<input className={directoryInput} required minLength={key === "password" ? 12 : key === "username" ? 3 : 1} maxLength={key === "password" || key === "username" ? 32 : key === "name" ? 64 : 320} type={key === "password" ? "password" : key === "email" ? "email" : "text"} autoComplete={key === "password" ? "new-password" : "off"} disabled={busy} value={form[key]} onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value }))} /></label>)}
        <p className="text-sm text-text-secondary">Local accounts start without admin privileges or namespace access. Share the initial password securely.</p><Button type="submit" disabled={busy}>Create local account</Button>
      </form>}
    </Card>}
    <label className="block text-sm max-w-md">Search username<input className={directoryInput} value={search} onChange={(e) => { setSearch(e.target.value); setPage(1); }} /></label>
    <div className="overflow-x-auto"><table className="w-full text-sm text-left"><thead><tr>{["User", "Email", "Account type", "Access", "Status"].map((label) => <th className="p-3 border-b border-border" key={label}>{label}</th>)}</tr></thead><tbody>{users.data?.users.map((u) => <tr key={u.id}><td className="p-3 border-b border-border"><p className="font-medium">{u.name}</p><p className="text-text-secondary">{u.username}</p></td><td className="p-3 border-b border-border">{u.email}</td><td className="p-3 border-b border-border">{source(u.origin)}</td><td className="p-3 border-b border-border">{u.admin ? "Administrator" : "Member"}</td><td className="p-3 border-b border-border">{u.awaiting_approval ? "Awaiting approval" : u.status === "confirmed" ? "Active" : "Unconfirmed"}</td></tr>)}</tbody></table></div>
    {users.isPending && <p>Loading users…</p>}{users.data?.users.length === 0 && <p>No users found.</p>}
    <div className="flex gap-3 items-center"><Button variant="secondary" disabled={page === 1 || users.isFetching} onClick={() => setPage(page - 1)}>Previous</Button><span className="text-sm">Page {page}{users.data ? ` · ${users.data.total} accounts` : ""}</span><Button variant="secondary" disabled={!users.data || page * users.data.per_page >= users.data.total || users.isFetching} onClick={() => setPage(page + 1)}>Next</Button></div>
  </div>;
}
