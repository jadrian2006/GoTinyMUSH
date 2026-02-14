import type {
  AuthResponse,
  WhoEntry,
  ChannelInfo,
  ScrollbackMessage,
} from "../types/events";

let baseUrl = "";

export function setBaseUrl(url: string) {
  baseUrl = url;
}

function authHeaders(token: string): Record<string, string> {
  return {
    Authorization: `Bearer ${token}`,
    "Content-Type": "application/json",
  };
}

export async function login(
  name: string,
  password: string,
): Promise<AuthResponse> {
  const res = await fetch(`${baseUrl}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name, password }),
  });
  if (!res.ok) throw new Error("Invalid credentials");
  return res.json();
}

export async function refreshToken(token: string): Promise<AuthResponse> {
  const res = await fetch(`${baseUrl}/api/v1/auth/refresh`, {
    method: "POST",
    headers: authHeaders(token),
  });
  if (!res.ok) throw new Error("Token refresh failed");
  return res.json();
}

export async function getWho(
  token?: string,
): Promise<{ players: WhoEntry[]; count: number }> {
  const headers: Record<string, string> = {};
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(`${baseUrl}/api/v1/who`, { headers });
  if (!res.ok) throw new Error("Failed to fetch WHO");
  return res.json();
}

export async function executeCommand(
  token: string,
  command: string,
): Promise<{ output: string[] }> {
  const res = await fetch(`${baseUrl}/api/v1/command`, {
    method: "POST",
    headers: authHeaders(token),
    body: JSON.stringify({ command }),
  });
  if (!res.ok) throw new Error("Command execution failed");
  return res.json();
}

export async function getChannels(
  token: string,
): Promise<{ channels: ChannelInfo[] }> {
  const res = await fetch(`${baseUrl}/api/v1/channels`, {
    headers: authHeaders(token),
  });
  if (!res.ok) throw new Error("Failed to fetch channels");
  return res.json();
}

export async function getChannelHistory(
  token: string,
  channel: string,
  since?: number,
): Promise<{ messages: ScrollbackMessage[] }> {
  let url = `${baseUrl}/api/v1/channels/${encodeURIComponent(channel)}/history`;
  if (since) url += `?since=${since}`;
  const res = await fetch(url, { headers: authHeaders(token) });
  if (!res.ok) throw new Error("Failed to fetch channel history");
  return res.json();
}
