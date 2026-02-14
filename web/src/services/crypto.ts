const SALT = new TextEncoder().encode("gotinymush-scrollback-v1");

async function deriveKey(password: string): Promise<CryptoKey> {
  const keyMaterial = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(password),
    "PBKDF2",
    false,
    ["deriveKey"],
  );
  return crypto.subtle.deriveKey(
    { name: "PBKDF2", salt: SALT, iterations: 100000, hash: "SHA-256" },
    keyMaterial,
    { name: "AES-GCM", length: 256 },
    false,
    ["encrypt", "decrypt"],
  );
}

export async function encryptScrollback(
  password: string,
  plaintext: string,
): Promise<{ encrypted_data: number[]; iv: number[] }> {
  const key = await deriveKey(password);
  const iv = crypto.getRandomValues(new Uint8Array(12));
  const encoded = new TextEncoder().encode(plaintext);
  const ciphertext = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv },
    key,
    encoded,
  );
  return {
    encrypted_data: Array.from(new Uint8Array(ciphertext)),
    iv: Array.from(iv),
  };
}

export async function decryptScrollback(
  password: string,
  encryptedData: number[],
  iv: number[],
): Promise<string> {
  const key = await deriveKey(password);
  const plaintext = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: new Uint8Array(iv) },
    key,
    new Uint8Array(encryptedData),
  );
  return new TextDecoder().decode(plaintext);
}
