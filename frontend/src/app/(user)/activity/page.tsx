"use client";

import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { CalendarDays, Loader2, ArrowLeft } from "lucide-react";
import { api } from "@/lib/api";
import { useToast } from "@/components/Toast";
import type { DailyActivity } from "@/lib/types";

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

// Dedicated daily-entry page (replaces the old modal, which overlapped the
// Handsontable grid). Reached from the dashboard button and from the 17:00 WIB
// reminder notification (which deep-links to /activity).
function ActivityInner() {
  const router = useRouter();
  const params = useSearchParams();
  const { notify } = useToast();

  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<DailyActivity>({
    date: params.get("date") || todayISO(),
    start_time: "08:00",
    end_time: "17:00",
    status: "P",
    activity: "",
    project_name: "",
    project_id: "",
    app_impacted: "",
  });

  const set = (k: keyof DailyActivity, v: string) => setForm({ ...form, [k]: v });

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await api("/api/v1/activities", { method: "POST", body: JSON.stringify(form) });
      notify("Activity saved", "success");
      router.push("/dashboard");
    } catch (err: any) {
      notify(err.message || "Save failed", "error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <div className="flex items-center gap-3">
        <button onClick={() => router.push("/dashboard")} className="btn-ghost px-3">
          <ArrowLeft size={16} />
        </button>
        <div className="flex items-center gap-3">
          <div className="grid h-11 w-11 place-items-center border-2 border-mr-ink bg-mr-yellow text-black">
            <CalendarDays size={22} />
          </div>
          <div>
            <h1 className="text-2xl font-extrabold">Daily Activity</h1>
            <p className="text-sm text-mr-muted">Fill today or pick a past date.</p>
          </div>
        </div>
      </div>

      <form onSubmit={submit} className="card flex flex-col gap-4 p-6">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label className="mb-1 block text-xs font-bold uppercase">Date</label>
            <input
              className="input"
              type="date"
              value={form.date}
              max={todayISO()}
              onChange={(e) => set("date", e.target.value)}
              required
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-bold uppercase">Status</label>
            <select
              className="input"
              value={form.status}
              onChange={(e) => set("status", e.target.value)}
            >
              <option value="P">Present</option>
              <option value="S">Sick</option>
              <option value="PM">Permission</option>
              <option value="V">Leave</option>
              <option value="BT">Business Trip</option>
              <option value="X">Off</option>
            </select>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="mb-1 block text-xs font-bold uppercase">Time In</label>
            <input
              className="input"
              type="time"
              value={form.start_time}
              onChange={(e) => set("start_time", e.target.value)}
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-bold uppercase">Time Out</label>
            <input
              className="input"
              type="time"
              value={form.end_time}
              onChange={(e) => set("end_time", e.target.value)}
            />
          </div>
        </div>

        <div>
          <label className="mb-1 block text-xs font-bold uppercase">Activity</label>
          <textarea
            className="input min-h-[96px]"
            value={form.activity}
            onChange={(e) => set("activity", e.target.value)}
            placeholder="What did you work on?"
          />
        </div>


        <div>
          <label className="mb-1 block text-xs font-semibold">
            Aplikasi Terdampak
          </label>
          <select
            className="input"
            value={form.app_impacted}
            onChange={(e) => set("app_impacted", e.target.value)}
          >
            <option value="">— Pilih —</option>
            <option value="Bisnis">Bisnis</option>
            <option value="Cash">Cash</option>
            <option value="Overseas">Overseas</option>
          </select>
        </div>

        <div className="mt-2 flex justify-end gap-2">
          <button type="button" onClick={() => router.push("/dashboard")} className="btn-ghost">
            Cancel
          </button>
          <button type="submit" className="btn-primary" disabled={saving}>
            {saving ? <Loader2 size={16} className="animate-spin" /> : null}
            Save activity
          </button>
        </div>
      </form>
    </div>
  );
}

export default function ActivityPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[40vh] items-center justify-center">
          <Loader2 className="animate-spin text-mr-purple" />
        </div>
      }
    >
      <ActivityInner />
    </Suspense>
  );
}
