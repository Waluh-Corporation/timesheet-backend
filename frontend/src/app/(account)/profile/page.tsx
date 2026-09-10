"use client";

import { useCallback, useEffect, useState } from "react";
import { Loader2, UserRound, Clock, KeyRound, Plus, Trash2, Fingerprint } from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useToast } from "@/components/Toast";
import { registerPasskey, passkeysSupported } from "@/lib/webauthn";
import type { ProfileChangeRequest, Passkey } from "@/lib/types";

// Account page for any authenticated user (user OR admin): profile details
// (edits require admin approval) and self-service passkey management.
export default function ProfilePage() {
  const { user, refresh } = useAuth();
  const { notify } = useToast();

  const [form, setForm] = useState({ name: "", mii_id: "", division: "", site: "" });
  const [changes, setChanges] = useState<ProfileChangeRequest[]>([]);
  const [passkeys, setPasskeys] = useState<Passkey[]>([]);
  const [saving, setSaving] = useState(false);
  const [addingKey, setAddingKey] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (user) {
      setForm({
        name: user.name || "",
        mii_id: user.mii_id || "",
        division: user.division || "",
        site: user.site || "",
      });
    }
  }, [user]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [ch, pk] = await Promise.all([
        api<ProfileChangeRequest[]>("/api/v1/profile/changes").catch(() => []),
        api<Passkey[]>("/api/v1/passkeys").catch(() => []),
      ]);
      setChanges(ch || []);
      setPasskeys(pk || []);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const set = (k: keyof typeof form, v: string) => setForm({ ...form, [k]: v });

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await api("/api/v1/profile/change", { method: "POST", body: JSON.stringify(form) });
      notify("Change requested — waiting for admin approval.", "success");
      load();
      refresh();
    } catch (err: any) {
      notify(err.message || "Request failed", "error");
    } finally {
      setSaving(false);
    }
  };

  const addPasskey = async () => {
    setAddingKey(true);
    try {
      await registerPasskey(`${user?.username}'s device`);
      notify("Passkey added", "success");
      load();
    } catch (err: any) {
      notify(
        err?.name === "NotAllowedError"
          ? "Passkey setup was cancelled or timed out."
          : err.message || "Could not add passkey",
        "error"
      );
    } finally {
      setAddingKey(false);
    }
  };

  const removePasskey = async (pk: Passkey) => {
    if (!confirm(`Remove passkey "${pk.friendly_name || "Passkey"}"?`)) return;
    try {
      await api(`/api/v1/passkeys/${pk.id}`, { method: "DELETE" });
      notify("Passkey removed", "success");
      load();
    } catch (err: any) {
      notify(err.message || "Remove failed", "error");
    }
  };

  const statusChip = (s: ProfileChangeRequest["status"]) => {
    const map: Record<string, string> = {
      pending: "bg-mr-yellow text-black",
      approved: "bg-mr-cyan text-black",
      rejected: "bg-mr-pink text-white",
    };
    return `chip ${map[s] || "bg-mr-surface2 text-mr-muted"}`;
  };

  const hasPending = changes.some((c) => c.status === "pending");

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <div className="flex items-center gap-3">
        <div className="grid h-11 w-11 place-items-center border-2 border-mr-ink bg-mr-yellow text-black">
          <UserRound size={22} />
        </div>
        <div>
          <h1 className="text-2xl font-extrabold">My Profile</h1>
          <p className="text-sm text-mr-muted">Manage your account, passkeys, and profile details.</p>
        </div>
      </div>

      {/* Account (read-only) */}
      <div className="card p-6">
        <h2 className="mb-4 text-lg font-bold">Account</h2>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <p className="text-xs font-bold uppercase text-mr-muted">Username</p>
            <p className="font-semibold">{user?.username}</p>
          </div>
          <div>
            <p className="text-xs font-bold uppercase text-mr-muted">Email</p>
            <p className="font-semibold">{user?.email}</p>
          </div>
          <div>
            <p className="text-xs font-bold uppercase text-mr-muted">Role</p>
            <span className="chip bg-mr-purple text-white">{user?.role}</span>
          </div>
        </div>
      </div>

      {/* Passkeys (self-service) */}
      <div className="card p-6">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <KeyRound size={18} className="text-mr-purple" />
            <h2 className="text-lg font-bold">Passkeys</h2>
            <span className="chip bg-mr-surface2 text-mr-muted">{passkeys.length}</span>
          </div>
          {passkeysSupported() && (
            <button onClick={addPasskey} disabled={addingKey} className="btn-primary text-sm">
              {addingKey ? <Loader2 size={14} className="animate-spin" /> : <Plus size={14} />}
              Add passkey
            </button>
          )}
        </div>
        {loading ? (
          <div className="flex justify-center py-6">
            <Loader2 className="animate-spin text-mr-purple" />
          </div>
        ) : passkeys.length === 0 ? (
          <p className="text-sm text-mr-muted">
            No passkeys yet. Add one to sign in without a password.
          </p>
        ) : (
          <div className="flex flex-col gap-2">
            {passkeys.map((pk) => (
              <div
                key={pk.id}
                className="flex items-center justify-between gap-3 border-2 border-mr-ink bg-mr-surface2 px-4 py-2"
              >
                <div className="flex items-center gap-3">
                  <Fingerprint size={18} className="text-mr-purple" />
                  <div>
                    <p className="text-sm font-semibold">{pk.friendly_name || "Passkey"}</p>
                    <p className="text-xs text-mr-muted">
                      Added {new Date(pk.created_at).toLocaleDateString()}
                    </p>
                  </div>
                </div>
                <button
                  onClick={() => removePasskey(pk)}
                  className="border-2 border-mr-ink p-2 text-mr-muted hover:bg-mr-pink hover:text-white"
                  title="Remove passkey"
                >
                  <Trash2 size={15} />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Editable profile → pending request */}
      <form onSubmit={submit} className="card flex flex-col gap-4 p-6">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-bold">Profile details</h2>
          {hasPending && (
            <span className="chip bg-mr-yellow text-black">
              <Clock size={12} /> Pending approval
            </span>
          )}
        </div>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label className="mb-1 block text-xs font-bold uppercase">Full name</label>
            <input className="input" value={form.name} onChange={(e) => set("name", e.target.value)} />
          </div>
          <div>
            <label className="mb-1 block text-xs font-bold uppercase">MII ID</label>
            <input className="input" value={form.mii_id} onChange={(e) => set("mii_id", e.target.value)} />
          </div>
          <div>
            <label className="mb-1 block text-xs font-bold uppercase">Division</label>
            <input className="input" value={form.division} onChange={(e) => set("division", e.target.value)} />
          </div>
          <div>
            <label className="mb-1 block text-xs font-bold uppercase">Site</label>
            <input className="input" value={form.site} onChange={(e) => set("site", e.target.value)} />
          </div>
        </div>
        <div className="flex justify-end">
          <button type="submit" className="btn-primary" disabled={saving}>
            {saving ? <Loader2 size={16} className="animate-spin" /> : null}
            Request change
          </button>
        </div>
      </form>

      {/* Request history */}
      <div className="card p-6">
        <h2 className="mb-4 text-lg font-bold">Change requests</h2>
        {loading ? (
          <div className="flex justify-center py-6">
            <Loader2 className="animate-spin text-mr-purple" />
          </div>
        ) : changes.length === 0 ? (
          <p className="text-sm text-mr-muted">No change requests yet.</p>
        ) : (
          <div className="flex flex-col gap-2">
            {changes.map((c) => (
              <div
                key={c.id}
                className="flex flex-wrap items-center justify-between gap-2 border-2 border-mr-ink bg-mr-surface2 px-4 py-2"
              >
                <div className="text-sm">
                  <p className="font-semibold">
                    {[c.name, c.mii_id, c.division, c.site].filter(Boolean).join(" · ") || "—"}
                  </p>
                  <p className="text-xs text-mr-muted">
                    {new Date(c.created_at).toLocaleString()}
                  </p>
                </div>
                <span className={statusChip(c.status)}>{c.status}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
