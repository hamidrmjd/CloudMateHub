import { useAuthStore } from "@/stores/authStore";
export type DirectoryKind = "ad" | "openldap";
export type DirectoryConfig = { kind: DirectoryKind; id: string; label: string; enabled: boolean; url: string; base_dn: string; bind_dn: string; bind_password?: string; password_set: boolean; ca_pem: string; user_filter: string; username_attribute: string; id_attribute: string; email_attribute: string; name_attribute: string; required_group_dn: string; provision_accounts: boolean };
export async function directoryRequest<T>(path: string, method = "GET", body?: unknown): Promise<T> {
  const response = await fetch(`/api${path}`, { method, headers: { Authorization: `Bearer ${useAuthStore.getState().token ?? ""}`, "Content-Type": "application/json" }, body: body ? JSON.stringify(body) : undefined, cache: "no-store" });
  const data = await response.json().catch(() => ({})) as T & { error?: string };
  if (!response.ok) throw Object.assign(new Error(data.error ?? "The request failed. Please try again."), { status: response.status, headers: response.headers });
  return data;
}
export const directoryInput = "block mt-1 w-full rounded border border-border bg-background px-3 py-2 text-sm";
