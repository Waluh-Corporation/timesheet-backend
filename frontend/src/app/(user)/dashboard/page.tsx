"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import dynamic from "next/dynamic";
import { useRouter } from "next/navigation";
import {
  Plus,
  Download,
  Bell,
  BellOff,
  Fingerprint,
  Loader2,
  CalendarRange,
} from "lucide-react";
import { api, downloadFile } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useToast } from "@/components/Toast";
import {
  enablePush,
  disablePush,
  pushSupported,
  registerServiceWorker,
} from "@/lib/push";
import { registerPasskey, passkeysSupported } from "@/lib/webauthn";
import type { DailyActivity } from "@/lib/types";

const HotGrid = dynamic(() => import("@/components/HotGrid"), { ssr: false });

const MONTHS = [
  "January", "February", "March", "April", "May", "June",
  "July", "August", "September", "October", "November", "December",
];

// Columns shown in the monthly grid. `fillableFields` (from the default
// template mapping) decides which of these the user may actually edit.
const GRID_COLUMNS: { key: keyof DailyActivity; label: string; field: string }[] = [
  { key: "start_time", label: "Time In", field: "time_in" },
  { key: "end_time", label: "Time Out", field: "time_out" },
  { key: "status", label: "Status", field: "status" },
  { key: "activity", label: "Activity", field: "activity" },
  { key: "app_impacted", label: "App Impacted", field: "app_impacted" },
];
// Allowed values for the "Aplikasi Terdampak" (app impacted) column.
const APP_IMPACTED_OPTIONS = ["Bisnis", "Cash", "Overseas"];

function daysInMonth(year: number, month: number) {
  return new Date(year, month, 0).getDate();
}

// Columns available in the monthly Handsontable grid.
const GRID_COLUMNS = [
  { field: "day", title: "Day", readOnly: true, width: 55 },
  { field: "date", title: "Date", readOnly: true, width: 95 },
  { field: "day_name", title: "Day Name", readOnly: true, width: 85 },
  { field: "start_time", title: "Start", readOnly: false, width: 70 },
  { field: "end_time", title: "End", readOnly: false, width: 70 },
  { field: "status", title: "Status", readOnly: false, width: 65 },
  { field: "activity", title: "Activity", readOnly: false, width: 220 },
  { field: "project_name", title: "Project", readOnly: false, width: 140 },
  { field: "project_id", title: "Project ID", readOnly: false, width: 100 },
  { field: "app_impacted", title: "App Impacted", readOnly: false, width: 120 },
] as const;

export default function DashboardPage() {
  const { user } = useAuth();
  const { notify } = useToast();

  const router = useRouter();
  const now = new Date();
  const [year, setYear] = useState(now.getFullYear());
  const [month, setMonth] = useState(now.getMonth() + 1); // 1-based
  const [activities, setActivities] = useState<DailyActivity[]>([]);
  const [holidays, setHolidays] = useState<Record<number, string>>({});
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [pushBusy, setPushBusy] = useState(false);
  const [pushOn, setPushOn] = useState(false);

  // All columns are fillable in the interactive grid
  const fillableFields = useMemo(() => new Set(GRID_COLUMNS.map((c) => c.field)), []);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [acts, hols] = await Promise.all([
        api<DailyActivity[]>(`/api/v1/activities?year=${year}&month=${month}`),
        api<{ date: string; description: string }[]>(
          `/api/v1/holidays?year=${year}&month=${month}`
        ).catch(() => []),
      ]);
      setActivities(acts || []);
      const hmap: Record<number, string> = {};
      (hols || []).forEach((h) => {
        const d = parseInt(h.date.split("-")[2], 10);
        if (!Number.isNaN(d)) hmap[d] = h.description;
      });
      setHolidays(hmap);
    } catch (err: any) {
      notify(err.message, "error");
    } finally {
      setLoading(false);
    }
  }, [year, month, notify]);

  useEffect(() => {
    load();
  }, [load]);

  // Register the service worker on mount and reflect current permission state.
  useEffect(() => {
    if (pushSupported()) {
      registerServiceWorker().catch(() => {});
      setPushOn(Notification.permission === "granted");
    }
  }, []);

  // Build a day-indexed grid: one row per calendar day.
  const totalDays = daysInMonth(year, month);
  const byDay = useMemo(() => {
    const map = new Map<number, DailyActivity>();
    activities.forEach((a) => {
      const d = new Date(a.date).getDate();
      map.set(d, a);
    });
    return map;
  }, [activities]);

  const gridData = useMemo(() => {
    const DOW = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
    const activityIdx = GRID_COLUMNS.findIndex((c) => c.field === "activity");
    const statusIdx = GRID_COLUMNS.findIndex((c) => c.field === "status");
    const rows: any[][] = [];
    for (let day = 1; day <= totalDays; day++) {
      const date = new Date(year, month - 1, day);
      const dow = date.getDay();
      const isWeekend = dow === 0 || dow === 6;
      const holiday = holidays[day];
      const act = byDay.get(day);
      const cols = GRID_COLUMNS.map((c) => (act ? (act[c.key] as string) || "" : ""));
      // For non-working days without an entry, show the reason as a hint.
      if (!act && (isWeekend || holiday)) {
        if (activityIdx >= 0) cols[activityIdx] = holiday || "Weekend";
        if (statusIdx >= 0) cols[statusIdx] = "X";
      }
      rows.push([`${MONTHS[month - 1].slice(0, 3)} ${day} · ${DOW[dow]}`, ...cols]);
    }
    return rows;
  }, [totalDays, byDay, month, year, holidays]);

  const colHeaders = useMemo(
    () => ["Day", ...GRID_COLUMNS.map((c) => c.label)],
    []
  );

  // Only mapped-fillable columns are editable; the Day column is always locked.
  const cells = useCallback(
    (row: number, col: number) => {
      const day = row + 1;
      const date = new Date(year, month - 1, day);
      const dow = date.getDay();
      const holiday = holidays[day];
      const nonWorking = dow === 0 || dow === 6 || !!holiday;
      const nonWorkingClass = holiday ? "ht-holiday" : "ht-weekend";

      if (col === 0) {
        return { readOnly: true, className: nonWorking ? nonWorkingClass : "ht-day-col" };
      }
      if (nonWorking) {
        // Weekends and holidays are non-editable and visually flagged.
        return { readOnly: true, className: nonWorkingClass };
      }
      const column = GRID_COLUMNS[col - 1];
      const editable = fillableFields.size === 0 || fillableFields.has(column.field);
      // Constrain the "Aplikasi Terdampak" column to a fixed dropdown.
      if (column.field === "app_impacted" && editable) {
        return {
          type: "dropdown",
          source: APP_IMPACTED_OPTIONS,
          allowInvalid: false,
          className: "",
        };
      }
      return { readOnly: !editable, className: editable ? "" : "ht-locked" };
    },
    [fillableFields, holidays, year, month]
  );

  // Persist grid edits back to the backend (upsert per day).
  const onAfterChange = useCallback(
    async (changes: any, source: string) => {
      if (source === "loadData" || !changes) return;
      const affectedDays = new Set<number>();
      changes.forEach(([rowIndex]: [number]) => affectedDays.add(rowIndex + 1));

      for (const day of Array.from(affectedDays)) {
        const rowIndex = day - 1;
        const row = gridData[rowIndex];
        if (!row) continue;
        const payload: DailyActivity = {
          date: `${year}-${String(month).padStart(2, "0")}-${String(day).padStart(2, "0")}`,
          start_time: row[1] || "",
          end_time: row[2] || "",
          status: row[3] || "",
          activity: row[4] || "",
          project_name: "",
          project_id: "",
          app_impacted: row[5] || "",
        };
        try {
          await api("/api/v1/activities", { method: "POST", body: JSON.stringify(payload) });
        } catch (err: any) {
          notify(err.message, "error");
        }
      }
    },
    [gridData, year, month, notify]
  );

  const generate = async () => {
    setGenerating(true);
    try {
      await downloadFile(
        "/api/v1/timesheet/generate",
        { month, year },
        `Timesheet_${month}_${year}.xlsx`
      );
      notify("Timesheet downloaded & emailed to you 📧", "success");
    } catch (err: any) {
      notify(err.message || "Generation failed", "error");
    } finally {
      setGenerating(false);
    }
  };

  const handleTogglePush = async () => {
    setPushBusy(true);
    try {
      if (pushOn) {
        await disablePush();
        setPushOn(false);
        notify("Daily reminders turned off", "info");
      } else {
        await enablePush();
        setPushOn(true);
        notify("Daily reminders enabled at 17:00 WIB", "success");
      }
    } catch (err: any) {
      notify(err.message || "Could not update reminders", "error");
    } finally {
      setPushBusy(false);
    }
  };

  const handleAddPasskey = async () => {
    try {
      await registerPasskey(`${user?.username}'s device`);
      notify("Passkey registered — you can now sign in without a password.", "success");
    } catch (err: any) {
      notify(err.message || "Passkey registration failed", "error");
    }
  };

  const filledCount = activities.length;

  return (
    <div className="flex flex-col gap-6">
      {/* Greeting hero */}
      <div className="card overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-4 bg-mr-yellow p-6 text-black">
          <div>
            <p className="text-sm font-bold opacity-70">Halo,</p>
            <h1 className="text-2xl font-extrabold">{user?.name || user?.username} 👋</h1>
            <p className="mt-1 text-sm font-semibold opacity-70">
              {filledCount} day{filledCount === 1 ? "" : "s"} filled this month.
            </p>
          </div>
          <button onClick={() => router.push("/activity")} className="btn-primary">
            <Plus size={18} /> Today&apos;s Activity
          </button>
        </div>
      </div>

      {/* Action row */}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <button
          onClick={() => router.push("/activity")}
          className="card flex items-center gap-3 p-4 text-left transition hover:shadow-hard"
        >
          <div className="grid h-10 w-10 place-items-center bg-mr-yellow text-black">
            <Plus size={18} />
          </div>
          <div>
            <p className="text-sm font-bold">Daily entry</p>
            <p className="text-xs text-mr-muted">Add today or a past date</p>
          </div>
        </button>

        <button
          onClick={generate}
          disabled={generating}
          className="card flex items-center gap-3 p-4 text-left transition hover:shadow-hard"
        >
          <div className="grid h-10 w-10 place-items-center  bg-mr-cyan text-mr-ink">
            {generating ? <Loader2 size={18} className="animate-spin" /> : <Download size={18} />}
          </div>
          <div>
            <p className="text-sm font-bold">Generate</p>
            <p className="text-xs text-mr-muted">Download & email .xlsx</p>
          </div>
        </button>

        <button
          onClick={handleTogglePush}
          disabled={pushBusy || !pushSupported()}
          className="card flex items-center gap-3 p-4 text-left transition hover:shadow-hard disabled:opacity-60"
        >
          <div className="grid h-10 w-10 place-items-center bg-mr-purple text-white">
            {pushBusy ? (
              <Loader2 size={18} className="animate-spin" />
            ) : pushOn ? (
              <BellOff size={18} />
            ) : (
              <Bell size={18} />
            )}
          </div>
          <div>
            <p className="text-sm font-bold">{pushOn ? "Disable reminders" : "Enable reminders"}</p>
            <p className="text-xs text-mr-muted">Daily push at 17:00 WIB</p>
          </div>
        </button>

        <button
          onClick={handleAddPasskey}
          disabled={!passkeysSupported()}
          className="card flex items-center gap-3 p-4 text-left transition hover:shadow-hard disabled:opacity-60"
        >
          <div className="grid h-10 w-10 place-items-center  bg-mr-pink text-white">
            <Fingerprint size={18} />
          </div>
          <div>
            <p className="text-sm font-bold">Add passkey</p>
            <p className="text-xs text-mr-muted">Passwordless sign-in</p>
          </div>
        </button>
      </div>

      {/* Month selector + grid */}
      <div className="card p-5">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <CalendarRange size={18} className="text-mr-purple" />
            <h2 className="text-lg font-bold">Monthly timesheet</h2>
          </div>
          <div className="flex gap-2">
            <select
              className="input w-auto"
              value={month}
              onChange={(e) => setMonth(Number(e.target.value))}
            >
              {MONTHS.map((m, i) => (
                <option key={m} value={i + 1}>
                  {m}
                </option>
              ))}
            </select>
            <select
              className="input w-auto"
              value={year}
              onChange={(e) => setYear(Number(e.target.value))}
            >
              {[year - 1, year, year + 1].map((y) => (
                <option key={y} value={y}>
                  {y}
                </option>
              ))}
            </select>
          </div>
        </div>

        {loading ? (
          <div className="flex justify-center py-12">
            <Loader2 className="animate-spin text-mr-purple" />
          </div>
        ) : (
          <HotGrid
            data={gridData}
            colHeaders={colHeaders}
            cells={cells}
            afterChange={onAfterChange}
            height={480}
          />
        )}
        <p className="mt-3 text-xs text-mr-muted">
          Only columns your admin marked as fillable are editable. Weekends and
          holidays are locked. Edits save automatically.
        </p>
      </div>
    </div>
  );
}
