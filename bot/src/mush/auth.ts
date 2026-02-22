export interface AuthResult {
  token: string;
  playerRef: number;
  playerName: string;
}

interface MushAuthConfig {
  restUrl: string;
  authMode: "password" | "apikey";
  user?: string;
  pass?: string;
  dbref?: string;
  apikey?: string;
}

export async function authenticate(config: MushAuthConfig): Promise<AuthResult> {
  if (config.authMode === "apikey") {
    return authenticateApikey(config);
  }
  return authenticatePassword(config);
}

async function authenticatePassword(config: MushAuthConfig): Promise<AuthResult> {
  if (!config.user || !config.pass) {
    throw new Error("MUSH_USER and MUSH_PASS required for password auth");
  }

  const res = await fetch(`${config.restUrl}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: config.user, password: config.pass }),
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(`Login failed (${res.status}): ${text}`);
  }

  const data = (await res.json()) as { token?: string };
  if (!data.token) {
    throw new Error("Login response missing token");
  }

  // Decode JWT payload to get player info
  const payload = decodeJwtPayload(data.token);
  return {
    token: data.token,
    playerRef: payload.player_ref,
    playerName: payload.player_name,
  };
}

async function authenticateApikey(config: MushAuthConfig): Promise<AuthResult> {
  if (!config.dbref || !config.apikey) {
    throw new Error("MUSH_DBREF and MUSH_APIKEY required for apikey auth");
  }

  const res = await fetch(`${config.restUrl}/api/v1/auth/apikey`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ dbref: config.dbref, apikey: config.apikey }),
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(`API key auth failed (${res.status}): ${text}`);
  }

  const data = (await res.json()) as { token?: string };
  if (!data.token) {
    throw new Error("API key auth response missing token");
  }

  const payload = decodeJwtPayload(data.token);
  return {
    token: data.token,
    playerRef: payload.player_ref,
    playerName: payload.player_name,
  };
}

function decodeJwtPayload(token: string): {
  player_ref: number;
  player_name: string;
} {
  const parts = token.split(".");
  if (parts.length !== 3) throw new Error("Invalid JWT format");
  const payload = JSON.parse(
    Buffer.from(parts[1]!, "base64url").toString("utf-8")
  );
  return {
    player_ref: payload.player_ref,
    player_name: payload.player_name,
  };
}

export async function refreshToken(
  restUrl: string,
  currentToken: string
): Promise<string> {
  const res = await fetch(`${restUrl}/api/v1/auth/refresh`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${currentToken}`,
    },
  });
  if (!res.ok) {
    throw new Error(`Token refresh failed (${res.status})`);
  }
  const data = (await res.json()) as { token?: string };
  if (!data.token) throw new Error("Refresh response missing token");
  return data.token;
}
