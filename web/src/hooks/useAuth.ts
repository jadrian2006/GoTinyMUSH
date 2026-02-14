import { useCallback } from "preact/hooks";
import * as api from "../services/api";
import { setAuth, clearAuth, token } from "../stores/gameStore";

export function useAuth() {
  const doLogin = useCallback(async (name: string, password: string) => {
    const res = await api.login(name, password);
    // Decode JWT payload to get player info
    const payload = JSON.parse(atob(res.token.split(".")[1]));
    setAuth(res.token, payload.player_name, payload.player_ref);
    return res.token;
  }, []);

  const doLogout = useCallback(() => {
    clearAuth();
  }, []);

  const doRefresh = useCallback(async () => {
    const t = token.value;
    if (!t) return;
    try {
      const res = await api.refreshToken(t);
      const payload = JSON.parse(atob(res.token.split(".")[1]));
      setAuth(res.token, payload.player_name, payload.player_ref);
    } catch {
      clearAuth();
    }
  }, []);

  return { login: doLogin, logout: doLogout, refresh: doRefresh };
}
