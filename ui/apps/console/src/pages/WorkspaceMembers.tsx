import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Callout } from "@cloudmatehub/design-system/primitives";
import { directoryRequest } from "@/api/directory";
import type { MemberView } from "@/client";
import Pagination from "@/components/common/Pagination";
import { useAuthStore } from "@/stores/authStore";

export type WorkspacePage<T> = { items: T[]; total: number; per_page: number };

const accountStatus = {
  active: "Active",
  awaiting_approval: "Awaiting approval",
  "not-confirmed": "Activation pending",
};

export default function WorkspaceMembers({ tenant }: { tenant: string }) {
  const user = useAuthStore((state) => state.userId);
  const [page, setPage] = useState(1);
  const members = useQuery({
    queryKey: ["admin-workspace-members", user, tenant, page],
    queryFn: () => directoryRequest<WorkspacePage<MemberView>>(
      `/user-management/workspaces/${encodeURIComponent(tenant)}/members?page=${page}`,
    ),
    retry: false,
  });

  if (members.isPending) return <p role="status">Loading members…</p>;
  if (members.isError) {
    return <Callout variant="error">Could not load members.{" "}
      <button type="button" className="underline" onClick={() => void members.refetch()}>Try again</button>
    </Callout>;
  }
  const { items, total, per_page } = members.data;
  return (
    <div className="border-t border-border pt-4">
      {items.length === 0 && <p className="text-sm text-text-secondary">No members on this page.</p>}
      <ul className="space-y-3">
        {items.map((member) => (
          <li key={member.id ?? member.email} className="flex flex-wrap items-start justify-between gap-2 text-sm">
            <div className="min-w-0">
              <p className="break-all font-medium">{member.name || member.username || member.email}</p>
              {(member.name || member.username) && (
                <p className="break-all text-xs text-text-secondary">{member.email}</p>
              )}
            </div>
            <div className="text-right">
              <p className="capitalize">{member.role}</p>
              {member.status && <p className="text-xs text-text-secondary">{accountStatus[member.status]}</p>}
            </div>
          </li>
        ))}
      </ul>
      <Pagination page={page} totalPages={Math.ceil(total / per_page)}
        totalCount={total} itemLabel="member" onPageChange={setPage} />
    </div>
  );
}
