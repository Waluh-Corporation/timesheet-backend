// Browser-side WebAuthn helpers. The backend speaks the standard
// PublicKeyCredentialCreationOptions / RequestOptions JSON shape, which the
// browser's credential API needs as ArrayBuffers — so we base64url-decode the
// challenge/id fields on the way in and re-encode on the way out.

import { api, setToken } from "./api";
import type { User } from "./types";

function bufToBase64url(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf);
  let str = "";
  bytes.forEach((b) => (str += String.fromCharCode(b)));
  return btoa(str).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function base64urlToBuf(value: string): ArrayBuffer {
  const pad = "=".repeat((4 - (value.length % 4)) % 4);
  const base64 = (value + pad).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  const bytes = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
  return bytes.buffer;
}

export function passkeysSupported(): boolean {
  return typeof window !== "undefined" && !!window.PublicKeyCredential;
}

// registerPasskey runs a registration ceremony for the logged-in user.
export async function registerPasskey(friendlyName = "My device") {
  const { session_id, options } = await api<{ session_id: string; options: any }>(
    "/api/v1/passkey/register/begin",
    { method: "POST", body: JSON.stringify({}) }
  );

  const publicKey = options.publicKey;
  publicKey.challenge = base64urlToBuf(publicKey.challenge);
  publicKey.user.id = base64urlToBuf(publicKey.user.id);
  if (publicKey.excludeCredentials) {
    publicKey.excludeCredentials = publicKey.excludeCredentials.map((c: any) => ({
      ...c,
      id: base64urlToBuf(c.id),
    }));
  }

  const credential = (await navigator.credentials.create({ publicKey })) as PublicKeyCredential;
  const response = credential.response as AuthenticatorAttestationResponse;

  const body = {
    id: credential.id,
    rawId: bufToBase64url(credential.rawId),
    type: credential.type,
    response: {
      attestationObject: bufToBase64url(response.attestationObject),
      clientDataJSON: bufToBase64url(response.clientDataJSON),
    },
  };

  await api(
    `/api/v1/passkey/register/finish?session_id=${session_id}&name=${encodeURIComponent(friendlyName)}`,
    { method: "POST", body: JSON.stringify(body) }
  );
}

// loginWithPasskey runs an assertion ceremony and returns the authenticated
// user. With no identifier it performs a usernameless (discoverable) login —
// the browser offers whatever resident passkey it holds for this site.
export async function loginWithPasskey(identifier = ""): Promise<User> {
  const { session_id, options } = await api<{ session_id: string; options: any }>(
    "/api/v1/auth/passkey/login/begin",
    { method: "POST", auth: false, body: JSON.stringify({ identifier }) }
  );

  const publicKey = options.publicKey;
  publicKey.challenge = base64urlToBuf(publicKey.challenge);
  if (publicKey.allowCredentials) {
    publicKey.allowCredentials = publicKey.allowCredentials.map((c: any) => ({
      ...c,
      id: base64urlToBuf(c.id),
    }));
  }

  const assertion = (await navigator.credentials.get({ publicKey })) as PublicKeyCredential;
  const response = assertion.response as AuthenticatorAssertionResponse;

  const body = {
    id: assertion.id,
    rawId: bufToBase64url(assertion.rawId),
    type: assertion.type,
    response: {
      authenticatorData: bufToBase64url(response.authenticatorData),
      clientDataJSON: bufToBase64url(response.clientDataJSON),
      signature: bufToBase64url(response.signature),
      userHandle: response.userHandle ? bufToBase64url(response.userHandle) : null,
    },
  };

  const res = await api<{ token: string; user: User }>(
    `/api/v1/auth/passkey/login/finish?session_id=${session_id}`,
    { method: "POST", auth: false, body: JSON.stringify(body) }
  );
  setToken(res.token);
  return res.user;
}
