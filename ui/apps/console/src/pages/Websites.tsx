import { useState, type FormEvent } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { GlobeAltIcon, TrashIcon } from "@heroicons/react/24/outline";
import { Button, Callout } from "@cloudmatehub/design-system/primitives";
import { directoryRequest } from "@/api/directory";
import PageHeader from "@/components/common/PageHeader";
import ConfirmDialog from "@/components/common/ConfirmDialog";
import { useAuthStore } from "@/stores/authStore";

const endpoint = "/assigned-websites";
const kinds = ["user", "namespace", "ad", "openldap"] as const;
type Assignment = { kind: (typeof kinds)[number]; value: string };
type Website = { id: string; name: string; url: string; assignments?: Assignment[] };
const empty = { id: "", name: "", url: "", assignments: [{ kind: "user", value: "" }] } satisfies Website;
const inputClass = "w-full rounded border border-border bg-background px-3 py-2 text-sm";

export default function Websites() {
  const isAdmin = useAuthStore((s) => s.isAdmin);
  const userId = useAuthStore((s) => s.userId);
  const client = useQueryClient();
  const queryKey = ["assigned-websites", userId, isAdmin];
  const sites = useQuery({ queryKey, queryFn: () => directoryRequest<Website[]>(endpoint) });
  const [draft, setDraft] = useState<Website | null>(null);
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
    if (draft) change.mutate({ site: draft, method: draft.id ? "PUT" : "POST" });
  }

  function updateAssignment(index: number, assignment: Assignment) {
    setDraft((site) => site && {
      ...site,
      assignments: site.assignments?.map((item, i) => i === index ? assignment : item),
    });
  }

  return (
    <div className="space-y-5">
      <PageHeader icon={<GlobeAltIcon className="h-6 w-6" />} title="Websites" description="Open assigned websites in a new browser tab." />
      {isAdmin && !draft && <Button onClick={() => { change.reset(); setDraft(empty); }}>Add website</Button>}
      {(sites.error || change.error) && <Callout variant="error">{(change.error ?? sites.error)?.message}</Callout>}
      {isAdmin && draft && (
        <form onSubmit={save} className="max-w-xl space-y-3 rounded border border-border p-4">
          <h2 className="font-semibold">{draft.id ? "Edit website" : "Add website"}</h2>
          <label className="block">Name<input required maxLength={120} className={inputClass} value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} /></label>
          <label className="block">Website URL<input required type="url" maxLength={2048} className={inputClass} value={draft.url} onChange={(e) => setDraft({ ...draft, url: e.target.value })} /></label>
          <p className="text-sm text-text-secondary">Anyone matching at least one assignment can see this link.</p>
          {draft.assignments?.map((assignment, index) => (
            <div key={index} className="flex gap-2">
              <select aria-label={`Assignment ${index + 1} type`} className={inputClass} value={assignment.kind} onChange={(e) => updateAssignment(index, { kind: e.target.value as Assignment["kind"], value: "" })}>
                <option value="user">Hub user ID</option>
                <option value="namespace">Namespace ID</option>
                <option value="ad">Active Directory group DN</option>
                <option value="openldap">OpenLDAP group DN</option>
              </select>
              <input required aria-label={`Assignment ${index + 1} value`} maxLength={512} className={inputClass} value={assignment.value} onChange={(e) => updateAssignment(index, { ...assignment, value: e.target.value })} />
              <Button type="button" disabled={draft.assignments?.length === 1} onClick={() => setDraft({ ...draft, assignments: draft.assignments?.filter((_, i) => i !== index) })}>Remove</Button>
            </div>
          ))}
          <div className="flex flex-wrap gap-2">
            <Button type="button" disabled={(draft.assignments?.length ?? 0) >= 32} onClick={() => setDraft({ ...draft, assignments: [...(draft.assignments ?? []), { kind: "user", value: "" }] })}>Add assignment</Button>
            <Button type="submit" disabled={change.isPending}>Save</Button>
            <Button type="button" disabled={change.isPending} onClick={() => { change.reset(); setDraft(null); }}>Cancel</Button>
          </div>
        </form>
      )}
      {sites.isPending && <p>Loading websites…</p>}
      {sites.data?.length === 0 && <p>No websites are assigned.</p>}
      {sites.data?.map((site) => (
        <article key={site.id} className="flex flex-wrap items-center gap-3 rounded border border-border p-4">
          <h2 className="mr-auto font-semibold">{site.name}</h2>
          <a className="underline" href={site.url} target="_blank" rel="noopener noreferrer">Open website</a>
          {isAdmin && <>
            <Button onClick={() => { change.reset(); setDraft(site); }}>Edit</Button>
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
