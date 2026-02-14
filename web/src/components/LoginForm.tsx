import { useRef, useState } from "preact/hooks";

interface LoginFormProps {
  onLogin: (name: string, password: string) => Promise<void>;
}

export function LoginForm({ onLogin }: LoginFormProps) {
  const nameRef = useRef<HTMLInputElement>(null);
  const passRef = useRef<HTMLInputElement>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    const name = nameRef.current?.value ?? "";
    const password = passRef.current?.value ?? "";
    if (!name || !password) return;

    setLoading(true);
    setError(null);
    try {
      await onLogin(name, password);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Login failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div class="flex items-center justify-center h-full">
      <form
        onSubmit={handleSubmit}
        class="bg-mush-surface border border-mush-panel rounded-lg p-8 w-80 space-y-4"
      >
        <h1 class="text-xl font-bold text-center text-mush-accent">
          GoTinyMUSH
        </h1>
        <p class="text-mush-dim text-center text-sm">Connect to the game</p>

        {error && (
          <div class="text-red-400 text-sm text-center">{error}</div>
        )}

        <input
          ref={nameRef}
          type="text"
          placeholder="Character name"
          class="w-full bg-mush-input border border-mush-panel rounded px-3 py-2 text-sm text-mush-text outline-none focus:border-mush-accent"
          autofocus
        />
        <input
          ref={passRef}
          type="password"
          placeholder="Password"
          class="w-full bg-mush-input border border-mush-panel rounded px-3 py-2 text-sm text-mush-text outline-none focus:border-mush-accent"
        />
        <button
          type="submit"
          disabled={loading}
          class="w-full bg-mush-accent hover:bg-red-500 text-white font-bold py-2 rounded text-sm transition-colors disabled:opacity-50"
        >
          {loading ? "Connecting..." : "Connect"}
        </button>
      </form>
    </div>
  );
}
