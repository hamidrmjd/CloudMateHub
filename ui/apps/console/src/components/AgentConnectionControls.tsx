import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowPathIcon } from "@heroicons/react/24/outline";
import { Button } from "@cloudmatehub/design-system/primitives";
import { directoryRequest } from "@/api/directory";
import ConfirmDialog from "@/components/common/ConfirmDialog";
import { useHasPermission } from "@/hooks/useHasPermission";
import { useAuthStore } from "@/stores/authStore";

type Connection = { connected: boolean; paused: boolean };
type Action = "disconnect" | "reconnect";

function Controls({ uid }: { uid: string }) {
  const user = useAuthStore((state) => state.userId);
  const tenant = useAuthStore((state) => state.tenant);
  const queryClient = useQueryClient();
  const [action, setAction] = useState<Action | null>(null);
  const path = `/devices/${encodeURIComponent(uid)}/agent-connection`;
  const queryKey = ["agent-connection", user, tenant, uid];
  const state = useQuery({
    queryKey,
    queryFn: () => directoryRequest<Connection>(path),
    refetchInterval: 5000,
    retry: false,
  });
  const change = useMutation({
    mutationFn: (selected: Action) => directoryRequest(path, "PUT", { action: selected }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey });
      setAction(null);
    },
  });

  if (!state.data) return null;
  const { connected, paused } = state.data;
  const title = action === "disconnect" ? "Disconnect agent" : "Reconnect agent";
  return (
    <div className="flex flex-wrap items-center gap-2">
      {paused && <span className="text-xs text-text-secondary">Agent disconnected</span>}
      {connected && <Button variant="secondary" onClick={() => {
        change.reset();
        setAction("disconnect");
      }}>Disconnect agent</Button>}
      <Button variant="secondary" onClick={() => {
        change.reset();
        setAction("reconnect");
      }}>Reconnect agent</Button>
      {action && <ConfirmDialog
        open
        icon={<ArrowPathIcon className="h-5 w-5" />}
        title={title}
        description={action === "disconnect"
          ? "Close the agent tunnel and pause new connections until you reconnect it. Active connections may be interrupted."
          : "Allow the agent to connect again. If a tunnel is already open, it will close and the running agent will retry."}
        confirmLabel={title}
        errorMessage={change.error?.message}
        onClose={() => { if (!change.isPending) setAction(null); }}
        onConfirm={() => change.mutateAsync(action).then(() => undefined)}
      />}
    </div>
  );
}

export default function AgentConnectionControls({ uid }: { uid: string }) {
  const allowed = useHasPermission("device:remove");
  const tenant = useAuthStore((state) => state.tenant);
  const user = useAuthStore((state) => state.userId);
  return allowed ? <Controls key={`${user}:${tenant}:${uid}`} uid={uid} /> : null;
}
