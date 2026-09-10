"use client";

import { Fragment, useEffect, useState, useCallback } from "react";
import {
  UserPlus,
  Loader2,
  Check,
  X,
  Ban,
  ShieldCheck,
  ClipboardList,
  KeyRound,
  Trash2,
  Fingerprint,
} from "lucide-react";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useToast } from "@/components/Toast";
import type { User, Role, ProfileChangeRequest, Passkey } from "@/lib/types";

// Admin console for provisioning accounts (the ONLY registration path) and
// reviewing self-service profile change requests.
export default function UsersPage() {
  const { notify } = useToast();
  const { user: currentUser } = useAuth();
  const [users, setUsers] = useState<User[]>([]);
  const [changes, setChanges] = useState<ProfileChangeRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [passkeysFor, setPasskeysFor] = useState<number | null>(null);
  const [userPasskeys, setUserPasskeys] = useState<Passkey[]>([]);
  const [pkLoading, setPkLoading] = useState(false);

  const emptyForm = {
    username: "",
    email: "",
    role: "user" as Role,
    name: "",
    mii_id: "",
    division: "",
    site: "",
    company: "",
  };
  const [form, setForm] = useState(emptyForm);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [u, c] = await Promise.all([
        api<User[]>("/api/v1/admin/users"),
        api<ProfileChangeRequest[]>("/api/v1/admin/profile-changes?status=pending"),
      ]);
      setUsers(u);
      setChanges(c || []);
    } catch (err: any) {
      notify(err.message || "Failed to load", "error");
    } finally {
      setLoading(false);
    }
  }, [notify]);

  useEffect(() => {
    load();
  }, [load]);

  const createUser = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    try {
      await api("/api/v1/admin/users", { method: "POST", body: JSON.stringify(form) });
      notify("User created — a setup email has been sent.", "success");
      setForm(emptyForm);
      load();
    } catch (err: any) {
      notify(err.message || "Create failed", "error");
    } finally {
      setCreating(false);
    }
  };

  const toggleActive = async (u: User) => {
    try {
      await api(`/api/v1/admin/users/${u.id}`, {
        method: "PATCH",
        body: JSON.stringify({ is_active: !u.is_active }),
      });
      load();
    } catch (err: any) {
      notify(err.message, "error");
    }
  };

  const deactivateUser = async (u: User) => {
    if (!confirm(`Deactivate ${u.username}? They keep their history and can be reactivated later.`)) return;
    try {
      await api(`/api/v1/admin/users/${u.id}`, { method: "DELETE" });
      notify("User deactivated", "success");
      load();
    } catch (err: any) {
      notify(err.message, "error");
    }
  };

  const viewPasskeys = async (u: User) => {
    if (passkeysFor === u.id) {
      setPasskeysFor(null);
      return;
    }
    setPasskeysFor(u.id);
    setPkLoading(true);
    try {
      const pk = await api<Passkey[]>(`/api/v1/admin/users/${u.id}/passkeys`);
      setUserPasskeys(pk || []);
    } catch (err: any) {
      notify(err.message, "error");
      setUserPasskeys([]);
    } finally {
      setPkLoading(false);
    }
  };

  const removeUserPasskey = async (u: User, pk: Passkey) => {
    if (!confirm(`Remove ${u.username}'s passkey "${pk.friendly_name || "Passkey"}"?`)) return;
    try {
      await api(`/api/v1/admin/users/${u.id}/passkeys/${pk.id}`, { method: "DELETE" });
      notify("Passkey removed", "success");
      const pks = await api<Passkey[]>(`/api/v1/admin/users/${u.id}/passkeys`);
      setUserPasskeys(pks || []);
    } catch (err: any) {
      notify(err.message, "error");
    }
  };

  const review = async (c: ProfileChangeRequest, action: "approve" | "reject") => {
    try {
      await api(`/api/v1/admin/profile-changes/${c.id}/review?action=${action}`, {
        method: "POST",
        body: JSON.stringify({}),
      });
      notify(`Request ${action}d`, "success");
      load();
    } catch (err: any) {
      notify(err.message, "error");
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <header>
        <h1 className="text-2xl font-extrabold">Users</h1>
        <p className="text-sm text-mr-muted">
          Provision accounts and approve profile changes.
        </p>
      </header>

      <div className="grid gap-6 lg:grid-cols-3">
        {/* Create user */}
        <div className="card h-fit p-6 lg:col-span-1">
          <div className="mb-4 flex items-center gap-2">
            <div className="grid h-9 w-9 place-items-center bg-mr-yellow text-black">
              <UserPlus size={18} />
            </div>
            <h2 className="text-lg font-bold">New account</h2>
          </div>
          <form onSubmit={createUser} className="flex flex-col gap-3">
            <input
              className="input"
              placeholder="Username"
              value={form.username}
              onChange={(e) => setForm({ ...form, username: e.target.value })}
              required
            />
            <input
              className="input"
              type="email"
              placeholder="Email"
              value={form.email}
              onChange={(e) => setForm({ ...form, email: e.target.value })}
              required
            />
            <input
              className="input"
              placeholder="Full name"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
            />
            <div className="grid grid-cols-2 gap-3">
              <input
                className="input"
                placeholder="MII ID"
                value={form.mii_id}
                onChange={(e) => setForm({ ...form, mii_id: e.target.value })}
              />
              <input
                className="input"
                placeholder="Site"
                value={form.site}
                onChange={(e) => setForm({ ...form, site: e.target.value })}
              />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <input
                className="input"
                placeholder="Division"
                value={form.division}
                onChange={(e) => setForm({ ...form, division: e.target.value })}
              />
              <select
                className="input font-semibold"
                value={form.company}
                onChange={(e) => setForm({ ...form, company: e.target.value })}
                required
              >
                <option value="">— Select Company —</option>
                <option value="MII">MII</option>
                <option value="SDD">SDD</option>
                <option value="NTT">NTT</option>
                <option value="Adidata">Adidata</option>
              </select>
            </div>
            <select
              className="input"
              value={form.role}
              onChange={(e) => setForm({ ...form, role: e.target.value as Role })}
            >
              <option value="user">User</option>
              <option value="admin">Admin</option>
            </select>
            <button type="submit" className="btn-primary w-full" disabled={creating}>
              {creating ? <Loader2 size={18} className="animate-spin" /> : <UserPlus size={18} />}
              Create & send setup link
            </button>
          </form>
        </div>

        {/* Pending profile changes + user list */}
        <div className="flex flex-col gap-6 lg:col-span-2">
          {changes.length > 0 && (
            <div className="card p-6">
              <div className="mb-4 flex items-center gap-2">
                <ClipboardList size={18} className="text-mr-purple" />
                <h2 className="text-lg font-bold">Pending profile changes</h2>
                <span className="chip bg-mr-yellow text-mr-ink">{changes.length}</span>
              </div>
              <div className="flex flex-col gap-3">
                {changes.map((c) => (
                  <div
                    key={c.id}
                    className="flex flex-wrap items-center justify-between gap-3  bg-mr-surface2 p-4"
                  >
                    <div className="text-sm">
                      <p className="font-semibold">{c.user?.username || `User #${c.user_id}`}</p>
                      <p className="text-mr-muted">
                        {[c.name, c.mii_id, c.division, c.site].filter(Boolean).join(" · ")}
                      </p>
                    </div>
                    <div className="flex gap-2">
                      <button
                        onClick={() => review(c, "approve")}
                        className="btn border-2 border-mr-ink bg-mr-cyan text-mr-ink"
                      >
                        <Check size={16} /> Approve
                      </button>
                      <button
                        onClick={() => review(c, "reject")}
                        className="btn border-2 border-mr-ink bg-mr-pink text-white"
                      >
                        <X size={16} /> Reject
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          <div className="card p-6">
            <h2 className="mb-4 text-lg font-bold">All users</h2>
            {loading ? (
              <div className="flex justify-center py-8">
                <Loader2 className="animate-spin text-mr-purple" />
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="text-xs uppercase text-mr-muted">
                      <th className="pb-2">User</th>
                      <th className="pb-2">Company</th>
                      <th className="pb-2">Role</th>
                      <th className="pb-2">Status</th>
                      <th className="pb-2 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {users.map((u) => (
                      <Fragment key={u.id}>
                      <tr className="border-t border-mr-ink">
                        <td className="py-3">
                          <p className="font-semibold">{u.name || u.username}</p>
                          <p className="text-xs text-mr-muted">{u.email}</p>
                        </td>
                        <td className="py-3">
                          <select
                            className={`input py-1 px-2 text-xs font-bold border-mr-ink/30 ${
                              u.company ? "bg-mr-surface" : "bg-amber-100 text-amber-800 border-amber-400"
                            }`}
                            value={u.company || ""}
                            onChange={async (e) => {
                              const newComp = e.target.value;
                              try {
                                await api(`/api/v1/admin/users/${u.id}`, {
                                  method: "PATCH",
                                  body: JSON.stringify({ company: newComp }),
                                });
                                notify(`Assigned ${u.username} to ${newComp || "none"}`, "success");
                                load();
                              } catch (err: any) {
                                notify(err.message, "error");
                              }
                            }}
                          >
                            <option value="">— Unassigned —</option>
                            <option value="MII">MII</option>
                            <option value="SDD">SDD</option>
                            <option value="NTT">NTT</option>
                            <option value="Adidata">Adidata</option>
                          </select>
                        </td>
                        <td className="py-3">
                          <span
                            className={`chip ${
                              u.role === "admin"
                                ? "bg-mr-purple text-white"
                                : "bg-mr-surface2 text-mr-muted"
                            }`}
                          >
                            {u.role === "admin" && <ShieldCheck size={12} />}
                            {u.role}
                          </span>
                        </td>
                        <td className="py-3">
                          <button
                            onClick={() => toggleActive(u)}
                            disabled={currentUser?.id === u.id}
                            title={
                              currentUser?.id === u.id
                                ? "You cannot deactivate your own account"
                                : undefined
                            }
                            className={`chip ${
                              u.is_active
                                ? "bg-mr-cyan text-mr-ink"
                                : "bg-mr-pink text-white"
                            } ${currentUser?.id === u.id ? "cursor-not-allowed opacity-60" : ""}`}
                          >
                            {u.is_active ? "Active" : "Disabled"}
                          </button>
                        </td>
                        <td className="py-3">
                          <div className="flex items-center justify-end gap-2">
                            <button
                              onClick={() => viewPasskeys(u)}
                              className={`border-2 border-mr-ink p-2 ${
                                passkeysFor === u.id
                                  ? "bg-mr-purple text-white"
                                  : "text-mr-muted hover:bg-mr-surface2"
                              }`}
                              title="Manage passkeys"
                            >
                              <KeyRound size={16} />
                            </button>
                            {u.is_active && currentUser?.id !== u.id && (
                              <button
                                onClick={() => deactivateUser(u)}
                                className="border-2 border-mr-ink p-2 text-mr-muted hover:bg-mr-pink hover:text-white"
                                title="Deactivate user"
                              >
                                <Ban size={16} />
                              </button>
                            )}
                          </div>
                        </td>
                      </tr>
                      {passkeysFor === u.id && (
                        <tr>
                          <td colSpan={4} className="border-t-2 border-mr-ink bg-mr-surface2 p-4">
                            <div className="mb-2 flex items-center gap-2 text-xs font-extrabold uppercase text-mr-muted">
                              <KeyRound size={14} /> Passkeys for {u.username}
                            </div>
                            {pkLoading ? (
                              <div className="flex justify-center py-3">
                                <Loader2 size={18} className="animate-spin text-mr-purple" />
                              </div>
                            ) : userPasskeys.length === 0 ? (
                              <p className="text-sm text-mr-muted">
                                This user has no passkeys.
                              </p>
                            ) : (
                              <div className="flex flex-col gap-2">
                                {userPasskeys.map((pk) => (
                                  <div
                                    key={pk.id}
                                    className="flex items-center justify-between gap-3 border-2 border-mr-ink bg-mr-surface px-3 py-2"
                                  >
                                    <div className="flex items-center gap-2">
                                      <Fingerprint size={16} className="text-mr-purple" />
                                      <div>
                                        <p className="text-sm font-semibold">{pk.friendly_name || "Passkey"}</p>
                                        <p className="text-xs text-mr-muted">
                                          Added {new Date(pk.created_at).toLocaleDateString()}
                                        </p>
                                      </div>
                                    </div>
                                    <button
                                      onClick={() => removeUserPasskey(u, pk)}
                                      className="border-2 border-mr-ink p-2 text-mr-muted hover:bg-mr-pink hover:text-white"
                                      title="Remove passkey"
                                    >
                                      <Trash2 size={15} />
                                    </button>
                                  </div>
                                ))}
                              </div>
                            )}
                          </td>
                        </tr>
                      )}
                      </Fragment>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
