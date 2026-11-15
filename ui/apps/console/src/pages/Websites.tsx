import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { GlobeAltIcon, TrashIcon } from "@heroicons/react/24/outline";
import { Button, Callout } from "@cloudmatehub/design-system/primitives";
import { directoryRequest } from "@/api/directory";
import { Link } from "react-router-dom";
import PageHeader from "@/components/common/PageHeader";
import ConfirmDialog from "@/components/common/ConfirmDialog";
import { useAuthStore } from "@/stores/authStore";

const endpoint = "/assigned-websites";
const kinds = ["user", "namespace", "ad", "openldap"] as const;
type Assignment = { kind: (typeof kinds)[number]; value: string };
type Website = { id: string; name: string; url: string; through_hub?: boolean; assignments?: Assignment[] };
const empty = { id: "", name: "", url: "", assignments: [{ kind: "user", value: "" }] } satisfies Website;
type DirectoryUser = { id: string; username: string; name: string };
const inputClass = "w-full rounded border border-border bg-background px-3 py-2 text-sm";

function UserPicker({ value, onSelect, account }: { value: string; onSelect: (id: string) => void; account: string | null }) {
  const [search, setSearch] = useState("");
  const username = search.trim();
  const users = useQuery({
    queryKey: ["website-user-search", account, username],
    queryFn: () => directoryRequest<{ users: DirectoryUser[] }>(`/user-management/users?search=${encodeURIComponent(username)}`),
    enabled: username.length > 0,
    retry: false,
  });

  return (
    <div className="w-full space-y-1">
      <input type="search" aria-label="Search by Hub username" placeholder="Search by Hub username" className={inputClass}
        value={search} maxLength={255} onChange={(e) => setSearch(e.target.value)} />
      {value && <p className="break-all text-xs text-text-secondary">Selected user ID: {value}</p>}
      {username && users.isPending && <p role="status">Searching…</p>}
      {users.isError && username && <p role="alert">Could not search Hub users.</p>}
      {users.data?.users.length === 0 && <p role="status">No matching users.</p>}
      {users.data?.users.map((user) => (
        <button key={user.id} type="button" aria-pressed={value === user.id}
          className="block w-full rounded border border-border px-2 py-1 text-left text-sm"
          onClick={() => onSelect(user.id)}>
          {user.username}{user.name && ` · ${user.name}`}
        </button>
      ))}
    </div>
  );
}

export default function Websites() {
  const isAdmin = useAuthStore((s) => s.isAdmin);
  const userId = useAuthStore((s) => s.userId);
  const workspace = useAuthStore((s) => s.tenant);
  const client = useQueryClient();
  const queryKey = ["assigned-websites", userId, isAdmin];
  const sites = useQuery({ queryKey, queryFn: () => directoryRequest<Website[]>(endpoint) });
  const [draft, setDraft] = useState<Website | null>(null);
  const [wholeWorkspace, setWholeWorkspace] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<Website | null>(null);
  const change = useMutation({
    mutationFn: ({ site, method }: { site: Website; method: "POST" | "PUT" | "DELETE" }) =>
      directoryRequest(site.id ? `${endpoint}/${site.id}` : endpoint, method, method === "DELETE" ? undefined : site),
    onSuccess: async () => {
      setDraft(null);
      setPendingDelete(null);
      await client.invalidateQueries({ queryKey });
    },
  });

  function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!draft || (wholeWorkspace && !workspace)) return;
    const site: Website = wholeWorkspace && workspace
      ? { ...draft, assignments: [{ kind: "namespace", value: workspace }] }
      : draft;
    change.mutate({ site, method: site.id ? "PUT" : "POST" });
  }

  function updateAssignment(index: number, assignment: Assignment) {
    setDraft((site) => site && {
      ...site,
      assignments: site.assignments?.map((item, i) => i === index ? assignment : item),
    });
  }

  const canSave = wholeWorkspace ? !!workspace : !!draft?.assignments?.length && draft.assignments.every((item) => item.value.trim());

  return (
    <div className="space-y-5">
      <PageHeader icon={<GlobeAltIcon className="h-6 w-6" />} title="Websites" description="Open assigned websites in a new browser tab." />
      {isAdmin && !draft && <div className="flex flex-wrap gap-2">
        <Button onClick={() => { change.reset(); setWholeWorkspace(false); setDraft(empty); }}>Add website</Button>
        {!sites.data?.some((site) => site.url === "https://www.nvidia.com/en-us/") &&
          <Button onClick={() => {
            change.reset();
            setWholeWorkspace(false);
            setDraft({ ...empty, name: "NVIDIA", url: "https://www.nvidia.com/en-us/", through_hub: true });
          }}>Add NVIDIA example</Button>}
      </div>}
      {(sites.error || change.error) && <Callout variant="error">{(change.error ?? sites.error)?.message}</Callout>}
      {isAdmin && draft && (
        <form onSubmit={save} className="max-w-xl space-y-3 rounded border border-border p-4">
          <h2 className="font-semibold">{draft.id ? "Edit website" : "Add website"}</h2>
          <label className="block">Name<input required maxLength={120} className={inputClass} value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} /></label>
          <label className="block">Website URL<input required type="url" maxLength={2048} className={inputClass} value={draft.url} onChange={(e) => setDraft({ ...draft, url: e.target.value })} /></label>
          <label className="flex items-center gap-2">
            <input type="checkbox" checked={draft.through_hub ?? false}
              onChange={(e) => setDraft({ ...draft, through_hub: e.target.checked })} />
            Proxy traffic through the Hub
          </label>
          <p className="text-sm text-text-secondary">Employees can view the page using the Hub’s internet connection. File downloads are disabled.</p>
          <label className="flex items-center gap-2">
            <input type="checkbox" checked={wholeWorkspace} disabled={!workspace}
              onChange={(e) => setWholeWorkspace(e.target.checked)} />
            Show to everyone in this workspace
          </label>
          <p className="text-sm text-text-secondary">{wholeWorkspace
            ? "Signed-in members of this workspace can see the link."
            : "Signed-in users who match any selected user or group can see the link."}</p>
          {!wholeWorkspace && draft.assignments?.map((assignment, index) => (
            <div key={index} className="flex gap-2">
              <select aria-label={`Assignment ${index + 1} type`} className={inputClass} value={assignment.kind} onChange={(e) => updateAssignment(index, { kind: e.target.value as Assignment["kind"], value: "" })}>
                <option value="user">Hub username</option>
                <option value="namespace">Workspace group (namespace ID)</option>
                <option value="ad">Active Directory group DN</option>
                <option value="openldap">OpenLDAP group DN</option>
              </select>
              {assignment.kind === "user" ? (
                <UserPicker value={assignment.value} account={userId ?? null}
                  onSelect={(id) => updateAssignment(index, { kind: "user", value: id })} />
              ) : (
                <input required aria-label={`Assignment ${index + 1} value`} maxLength={512} className={inputClass}
                  value={assignment.value} onChange={(e) => updateAssignment(index, { ...assignment, value: e.target.value })} />
              )}
              <Button type="button" disabled={draft.assignments?.length === 1} onClick={() => setDraft({ ...draft, assignments: draft.assignments?.filter((_, i) => i !== index) })}>Remove</Button>
            </div>
          ))}
          <div className="flex flex-wrap gap-2">
            {!wholeWorkspace && <Button type="button" disabled={(draft.assignments?.length ?? 0) >= 32} onClick={() => setDraft({ ...draft, assignments: [...(draft.assignments ?? []), { kind: "user", value: "" }] })}>Add assignment</Button>}
            <Button type="submit" disabled={change.isPending || !canSave}>Save</Button>
            <Button type="button" disabled={change.isPending} onClick={() => { change.reset(); setDraft(null); }}>Cancel</Button>
          </div>
        </form>
      )}
      {sites.isPending && <p>Loading websites…</p>}
      {sites.data?.length === 0 && <p>No websites are assigned.</p>}
      {sites.data?.map((site) => (
        <article key={site.id} className="flex flex-wrap items-center gap-3 rounded border border-border p-4">
          <h2 className="mr-auto font-semibold">{site.name}</h2>
          {site.through_hub
            ? <Link className="underline" to={`/websites/${encodeURIComponent(site.id)}/view`} target="_blank" rel="noopener noreferrer">Open through Hub</Link>
            : <a className="underline" href={site.url} target="_blank" rel="noopener noreferrer">Open directly</a>}
          {isAdmin && <>
            <Button onClick={() => { change.reset();
              const assignment = site.assignments?.length === 1 && site.assignments[0];
              const isWholeWorkspace = !!workspace && !!assignment && assignment.kind === "namespace" && assignment.value === workspace;
              setWholeWorkspace(isWholeWorkspace);
              setDraft(isWholeWorkspace ? { ...site, assignments: empty.assignments } : site); }}>Edit</Button>
            <Button onClick={() => { change.reset(); setPendingDelete(site); }}>Delete</Button>
          </>}
        </article>
      ))}
      {pendingDelete && <ConfirmDialog
        open
        icon={<TrashIcon className="h-5 w-5" />}
        errorMessage={change.error?.message}
        title="Delete website"
        description={`Remove the link to ${pendingDelete.name} from the hub?`}
        onClose={() => setPendingDelete(null)}
        onConfirm={() => change.mutateAsync({ site: pendingDelete, method: "DELETE" }).then(() => undefined)}
      />}
    </div>
  );
}
