"use client";

import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Loader2, ShieldCheck } from "lucide-react";
import { api } from "@/lib/api";
import { useToast } from "@/components/Toast";

// Completes both the admin-invite setup flow and the forgot-password flow: both
// deliver a token in the query string that authorizes setting a new password.
function ResetPasswordInner() {
  const params = useSearchParams();
  const router = useRouter();
  const { notify } = useToast();

  const [token, setToken] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setToken(params.get("token") || "");
  }, [params]);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (password !== confirm) {
      notify("Passwords do not match", "error");
      return;
    }
    setBusy(true);
    try {
      await api("/api/v1/auth/reset-password", {
        method: "POST",
        auth: false,
        body: JSON.stringify({ token, password }),
      });
      notify("Password updated — please sign in.", "success");
      router.replace("/login");
    } catch (err: any) {
      notify(err.message || "Reset failed", "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex min-h-screen items-center justify-center p-4">
      <div className="w-full max-w-md">
        <div className="mb-6 flex flex-col items-center text-center">
          <div className="mb-3 grid h-16 w-16 place-items-center  bg-mr-purple shadow-hard">
            <ShieldCheck size={30} className="text-white" />
          </div>
          <h1 className="text-2xl font-extrabold">Set your password</h1>
          <p className="mt-1 text-sm text-mr-muted">Choose a strong new password.</p>
        </div>

        <form onSubmit={submit} className="card flex flex-col gap-4 p-6">
          {!token && (
            <p className="border-2 border-mr-ink bg-mr-dangerBg px-4 py-2 text-sm font-semibold text-mr-dangerFg">
              Missing reset token. Use the link from your email.
            </p>
          )}
          <div>
            <label className="mb-1 block text-sm font-semibold">New password</label>
            <input
              className="input"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              minLength={8}
              required
            />
          </div>
          <div>
            <label className="mb-1 block text-sm font-semibold">Confirm password</label>
            <input
              className="input"
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              minLength={8}
              required
            />
          </div>
          <button type="submit" className="btn-primary w-full" disabled={busy || !token}>
            {busy ? <Loader2 size={18} className="animate-spin" /> : null}
            Update password
          </button>
        </form>
      </div>
    </div>
  );
}

export default function ResetPasswordPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center">
          <Loader2 className="animate-spin text-mr-purple" />
        </div>
      }
    >
      <ResetPasswordInner />
    </Suspense>
  );
}
