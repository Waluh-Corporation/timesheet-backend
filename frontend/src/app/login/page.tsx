"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { CalendarCheck, KeyRound, Fingerprint, Loader2 } from "lucide-react";
import { useAuth } from "@/lib/auth";
import { useToast } from "@/components/Toast";
import ThemeToggle from "@/components/ThemeToggle";
import { loginWithPasskey, passkeysSupported } from "@/lib/webauthn";
import { api, getToken } from "@/lib/api";

// friendlyAuthError maps raw backend/browser errors to a message that's safe
// and useful to show a user, instead of surfacing the raw response text.
function friendlyAuthError(err: any, context: "password" | "passkey"): string {
  const raw = (err?.message || "").toLowerCase();
  if (err?.name === "NotAllowedError" || raw.includes("timed out") || raw.includes("not allowed")) {
    return "Passkey sign-in was cancelled or timed out. Please try again.";
  }
  if (raw.includes("failed to fetch") || raw.includes("networkerror") || raw.includes("load failed")) {
    return "Can't reach the server. Check your connection and try again.";
  }
  if (raw.includes("disabled")) {
    return "Your account has been deactivated. Please contact your administrator.";
  }
  if (raw.includes("invalid credentials") || raw.includes("unauthorized")) {
    return context === "passkey"
      ? "No matching passkey was found for this device."
      : "Incorrect username/email or password.";
  }
  return context === "passkey"
    ? "Couldn't sign in with a passkey. Try your password instead."
    : "Sign-in failed. Please try again.";
}

// The sole entry point to the portal. There is NO public sign-up — accounts are
// created by admins. This page offers password login, passkey login, and the
// forgot-password flow.
export default function LoginPage() {
  const { loginWithPassword, loginWithToken } = useAuth();
  const { notify } = useToast();
  const router = useRouter();

  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);
  const [showForgot, setShowForgot] = useState(false);
  const [forgotEmail, setForgotEmail] = useState("");

  const routeForRole = (role: string) =>
    router.replace(role === "admin" ? "/users" : "/dashboard");

  const handlePasswordLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const user = await loginWithPassword(identifier, password);
      notify(`Welcome back, ${user.name || user.username}!`, "success");
      routeForRole(user.role);
    } catch (err: any) {
      notify(friendlyAuthError(err, "password"), "error");
    } finally {
      setBusy(false);
    }
  };

  const handlePasskeyLogin = async () => {
    setBusy(true);
    try {
      // Usernameless: an identifier is optional. If the user typed one we scope
      // to it, otherwise the browser offers its resident passkeys.
      const user = await loginWithPasskey(identifier);
      // loginWithPasskey already persisted the JWT; sync the auth context user.
      loginWithToken(getToken() || "", user);
      notify("Signed in with passkey", "success");
      routeForRole(user.role);
    } catch (err: any) {
      notify(friendlyAuthError(err, "passkey"), "error");
    } finally {
      setBusy(false);
    }
  };

  const handleForgot = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      await api("/api/v1/auth/forgot-password", {
        method: "POST",
        auth: false,
        body: JSON.stringify({ email: forgotEmail }),
      });
      notify("If that email exists, a reset link is on its way.", "success");
      setShowForgot(false);
    } catch (err: any) {
      notify(err.message || "Request failed", "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="relative flex min-h-screen items-center justify-center p-4">
      <div className="absolute right-4 top-4">
        <ThemeToggle />
      </div>
      <div className="w-full max-w-md">
        <div className="mb-6 flex flex-col items-center text-center">
          <div className="mb-3 grid h-16 w-16 place-items-center bg-mr-yellow text-black shadow-hard">
            <CalendarCheck size={30} className="text-black" />
          </div>
          <h1 className="text-2xl font-extrabold">Timesheet Portal</h1>
          <p className="mt-1 text-sm text-mr-muted">
            Sign in to fill today&apos;s activity ✨
          </p>
        </div>

        <div className="card p-6">
          {!showForgot ? (
            <form onSubmit={handlePasswordLogin} className="flex flex-col gap-4">
              <div>
                <label className="mb-1 block text-sm font-semibold">Username or Email</label>
                <input
                  className="input"
                  value={identifier}
                  onChange={(e) => setIdentifier(e.target.value)}
                  placeholder="you@company.com"
                  autoComplete="username"
                  required
                />
              </div>
              <div>
                <label className="mb-1 block text-sm font-semibold">Password</label>
                <input
                  className="input"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••"
                  autoComplete="current-password"
                  required
                />
              </div>

              <button type="submit" className="btn-primary w-full" disabled={busy}>
                {busy ? <Loader2 size={18} className="animate-spin" /> : <KeyRound size={18} />}
                Sign in
              </button>

              {passkeysSupported() && (
                <button
                  type="button"
                  onClick={handlePasskeyLogin}
                  className="btn-accent w-full"
                  disabled={busy}
                >
                  <Fingerprint size={18} /> Sign in with passkey
                </button>
              )}

              <button
                type="button"
                onClick={() => setShowForgot(true)}
                className="text-center text-sm font-semibold text-mr-purple hover:underline"
              >
                Forgot your password?
              </button>
            </form>
          ) : (
            <form onSubmit={handleForgot} className="flex flex-col gap-4">
              <div>
                <label className="mb-1 block text-sm font-semibold">Email</label>
                <input
                  className="input"
                  type="email"
                  value={forgotEmail}
                  onChange={(e) => setForgotEmail(e.target.value)}
                  placeholder="you@company.com"
                  required
                />
                <p className="mt-2 text-xs text-mr-muted">
                  We&apos;ll email you a link to reset your password.
                </p>
              </div>
              <button type="submit" className="btn-primary w-full" disabled={busy}>
                {busy ? <Loader2 size={18} className="animate-spin" /> : null}
                Send reset link
              </button>
              <button
                type="button"
                onClick={() => setShowForgot(false)}
                className="text-center text-sm font-semibold text-mr-purple hover:underline"
              >
                Back to sign in
              </button>
            </form>
          )}
        </div>

        <p className="mt-4 text-center text-xs text-mr-muted">
          Accounts are provisioned by administrators. Contact your admin for access.
        </p>
      </div>
    </div>
  );
}
