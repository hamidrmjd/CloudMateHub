import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { FolderPlusIcon, Squares2X2Icon } from "@heroicons/react/24/outline";
import { Badge, Button, Callout, Card } from "@cloudmatehub/design-system/primitives";
import { directoryRequest } from "@/api/directory";
import Pagination from "@/components/common/Pagination";
import WorkspaceMembers, { type WorkspacePage } from "./WorkspaceMembers";
import CreateNamespaceDialog from "@/components/common/CreateNamespaceDialog";
import PageHeader from "@/components/common/PageHeader";
import { useNamespaces, useSwitchNamespace } from "@/hooks/useNamespaceRequests";
import { useAuthStore } from "@/stores/authStore";

type Workspace = { tenant_id: string; name: string };

export default function Workspaces() {
  const isAdmin = useAuthStore((state) => state.isAdmin);
  const user = useAuthStore((state) => state.userId);
  const tenant = useAuthStore((state) => state.tenant);
  const membership = useNamespaces();
  const switchWorkspace = useSwitchNamespace();
  const [creating, setCreating] = useState(false);
  const [page, setPage] = useState(1);
  const [expanded, setExpanded] = useState<string | null>(null);
  const inventory = useQuery({
    queryKey: ["admin-workspaces", user, page],
    queryFn: () => directoryRequest<WorkspacePage<Workspace>>(`/user-management/workspaces?page=${page}`),
    enabled: isAdmin,
    retry: false,
    refetchOnMount: "always",
  });
  const workspaces = isAdmin ? inventory.data?.items ?? [] : membership.namespaces;
  const { isLoading, error, refetch } = isAdmin ? inventory : membership;
  const joined = new Set(membership.namespaces.map((workspace) => workspace.tenant_id));

  return (
    <div>
      <PageHeader
        icon={<Squares2X2Icon className="h-6 w-6" />}
        title="Workspaces"
        description={isAdmin ? "Browse all workspaces and see who belongs to each one." : "Choose a workspace to view its devices and sessions or open its team."}
      >
        {isAdmin && (
          <Button icon={<FolderPlusIcon className="h-4 w-4" />} onClick={() => setCreating(true)}>
            Create workspace
          </Button>
        )}
      </PageHeader>
      {error && (
        <Callout variant="error">
          Could not load workspaces.{" "}
          <button type="button" className="underline" onClick={() => void refetch()}>Try again</button>
        </Callout>
      )}
      {switchWorkspace.isError && (
        <Callout variant="error">Could not open that workspace. Check your membership and try again.</Callout>
      )}
      {isLoading && <p role="status">Loading workspaces…</p>}
      {!isLoading && !error && workspaces.length === 0 && (
        <p className="text-text-secondary">
          {isAdmin ? "Create a workspace to get started." : "Ask a workspace administrator to add you to their team."}
        </p>
      )}
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {workspaces.map((workspace) => (
          <Card key={workspace.tenant_id} className="space-y-4 p-5">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="mr-auto break-all font-semibold">{workspace.name}</h2>
              {tenant === workspace.tenant_id && <Badge tone="green">Active</Badge>}
            </div>
            <div className="flex flex-wrap gap-2">
              {isAdmin && (
                <Button variant="secondary" aria-expanded={expanded === workspace.tenant_id}
                  onClick={() => setExpanded((current) => current === workspace.tenant_id ? null : workspace.tenant_id)}>
                  {expanded === workspace.tenant_id ? "Hide members" : "Show members"}
                </Button>
              )}
              {joined.has(workspace.tenant_id) && (
                <>
                  <Button disabled={switchWorkspace.isPending}
                    onClick={() => switchWorkspace.mutate({ tenantId: workspace.tenant_id })}>
                    Open workspace
                  </Button>
                  <Button variant="secondary" disabled={switchWorkspace.isPending}
                    onClick={() => switchWorkspace.mutate({ tenantId: workspace.tenant_id, redirectTo: "/team" })}>
                    Open team
                  </Button>
                </>
              )}
            </div>
            {isAdmin && expanded === workspace.tenant_id && <WorkspaceMembers key={workspace.tenant_id} tenant={workspace.tenant_id} />}
          </Card>
        ))}
      </div>
      {isAdmin && inventory.data && (
        <Pagination page={page} totalPages={Math.ceil(inventory.data.total / inventory.data.per_page)}
          totalCount={inventory.data.total} itemLabel="workspace"
          onPageChange={(next) => { setPage(next); setExpanded(null); }} />
      )}
      {creating && <CreateNamespaceDialog open workspace onClose={() => setCreating(false)} />}
    </div>
  );
}
