import { useAuthStore } from "@/stores/authStore";

export async function vpnRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = useAuthStore.getState().token;
  const response = await fetch(`/api/vpn${path}`, {
    ...init,
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${token ?? ""}`, ...init.headers },
  });
  if (!response.ok) {
    const failure = await response.json().catch(() => ({ error: "VPN service is unavailable" })) as { error?: string };
    throw new Error(failure.error ?? "VPN request failed");
  }
  return response.json() as Promise<T>;
}
