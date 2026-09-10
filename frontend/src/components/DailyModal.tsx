"use client";

import { useState } from "react";
import { X, Loader2, CalendarDays } from "lucide-react";
import { api } from "@/lib/api";
import { useToast } from "@/components/Toast";
import type { DailyActivity } from "@/lib/types";

function todayISO(): string {
  const now = new Date();
  // Render in local time; the backend normalises to Asia/Jakarta.
  return now.toISOString().slice(0, 10);
}

// Friendly modal prompting the user to fill "Today's Activity" or a past date,
// enabling incremental daily entry instead of a whole-month form.
export default function DailyModal({
  onClose,
  onSaved,
}: {
  onClose: () => void;
  onSaved: () => void;
}) {
  const { notify } = useToast();
  const [saving, setSaving] = useState(false);
  const [form, setForm] = useState<DailyActivity>({
    date: todayISO(),
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
      notify("Saved today's activity 🎉", "success");
      onSaved();
      onClose();
    } catch (err: any) {
      notify(err.message || "Save failed", "error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
      <div className="card w-full max-w-lg p-6 shadow-hard">
        <div className="mb-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="grid h-10 w-10 place-items-center bg-mr-yellow text-black">
              <CalendarDays size={20} />
            </div>
            <div>
              <h2 className="text-lg font-extrabold">Today&apos;s Activity</h2>
              <p className="text-xs text-mr-muted">Fill in what you worked on.</p>
            </div>
          </div>
          <button onClick={onClose} className=" p-2 hover:bg-mr-surface2">
            <X size={18} />
          </button>
        </div>

        <form onSubmit={submit} className="flex flex-col gap-3">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="mb-1 block text-xs font-semibold">Date</label>
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
              <label className="mb-1 block text-xs font-semibold">Status</label>
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

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="mb-1 block text-xs font-semibold">Time In</label>
              <input
                className="input"
                type="time"
                value={form.start_time}
                onChange={(e) => set("start_time", e.target.value)}
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-semibold">Time Out</label>
              <input
                className="input"
                type="time"
                value={form.end_time}
                onChange={(e) => set("end_time", e.target.value)}
              />
            </div>
          </div>

          <div>
            <label className="mb-1 block text-xs font-semibold">Activity</label>
            <textarea
              className="input min-h-[80px]"
              value={form.activity}
              onChange={(e) => set("activity", e.target.value)}
              placeholder="What did you work on today?"
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
            <button type="button" onClick={onClose} className="btn-ghost">
              Cancel
            </button>
            <button type="submit" className="btn-primary" disabled={saving}>
              {saving ? <Loader2 size={16} className="animate-spin" /> : null}
              Save activity
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
