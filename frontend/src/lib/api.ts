// Thin fetch wrapper around the Go backend API.
//
// The base URL is configurable via NEXT_PUBLIC_API_URL (baked at build time).
// When unset it defaults to "" — i.e. same-origin, relative "/api/v1/..." paths —
// which is exactly what the single unified image needs, since the Go server
// serves this frontend and the API from the same host. For local `bun run dev`
// against a separately-running backend, set NEXT_PUBLIC_API_URL=http://localhost:8080
// (the dev docker-compose already does this).

export const API_BASE = process.env.NEXT_PUBLIC_API_URL || "";

const TOKEN_KEY = "ts_token";

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string) {
  window.localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken() {
  window.localStorage.removeItem(TOKEN_KEY);
}

interface RequestOptions extends RequestInit {
  auth?: boolean;
}

export async function api<T = any>(
  path: string,
  opts: RequestOptions = {}
): Promise<T> {
  const headers = new Headers(opts.headers);
  if (!(opts.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }
  if (opts.auth !== false) {
    const token = getToken();
    if (token) headers.set("Authorization", `Bearer ${token}`);
  }

  const res = await fetch(`${API_BASE}${path}`, { ...opts, headers });

  if (res.status === 401 && typeof window !== "undefined") {
    clearToken();
  }

  const contentType = res.headers.get("content-type") || "";
  if (!res.ok) {
    let message = res.statusText;
    if (contentType.includes("application/json")) {
      const data = await res.json().catch(() => null);
      if (data?.error) message = data.error;
      else if (data?.message) message = data.message;
    }
    throw new Error(message);
  }

  if (contentType.includes("application/json")) {
    const json = await res.json();
    if (json && typeof json === "object" && "data" in json && "code" in json) {
      return json.data as T;
    }
    return json as T;
  }
  // Non-JSON (e.g. file downloads) returned as blob.
  return res.blob() as unknown as T;
}

// downloadFile POSTs a JSON body and triggers a browser download of the
// returned binary stream (used for timesheet generation).
export async function downloadFile(
  path: string,
  body: unknown,
  fallbackName: string
) {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  const token = getToken();
  if (token) headers["Authorization"] = `Bearer ${token}`;

  const res = await fetch(`${API_BASE}${path}`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(data.error || data.message || "download failed");
  }

  const disposition = res.headers.get("content-disposition") || "";
  const match = disposition.match(/filename=([^;]+)/);
  const filename = match ? match[1].trim() : fallbackName;

  const blob = await res.blob();
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  window.URL.revokeObjectURL(url);
}
