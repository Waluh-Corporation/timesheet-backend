"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import dynamic from "next/dynamic";
import {
  Upload,
  Save,
  Loader2,
  Trash2,
  MousePointerClick,
  Tag,
  Star,
  Sparkles,
  Wand2,
  Building2,
  CheckCircle2,
} from "lucide-react";
import { api, API_BASE, getToken } from "@/lib/api";
import { useToast } from "@/components/Toast";
import type { Template, CellMapping, MappingField, MappingScope } from "@/lib/types";

// Handsontable manipulates the DOM on import, so load the wrapper client-only.
const HotGrid = dynamic(() => import("@/components/HotGrid"), { ssr: false });

// Field catalogue the admin can assign to cells/columns.
const FIELD_OPTIONS: { value: MappingField; label: string; scope: MappingScope; color: string }[] = [
  { value: "date", label: "Date (per day)", scope: "daily_column", color: "bg-blue-100 text-blue-800 border-blue-300" },
  { value: "time_in", label: "Time In (per day)", scope: "daily_column", color: "bg-emerald-100 text-emerald-800 border-emerald-300" },
  { value: "time_out", label: "Time Out (per day)", scope: "daily_column", color: "bg-teal-100 text-teal-800 border-teal-300" },
  { value: "status", label: "Status (per day)", scope: "daily_column", color: "bg-amber-100 text-amber-800 border-amber-300" },
  { value: "activity", label: "Activity Remark (per day)", scope: "daily_column", color: "bg-purple-100 text-purple-800 border-purple-300" },
  { value: "project_name", label: "Project Name (per day)", scope: "daily_column", color: "bg-indigo-100 text-indigo-800 border-indigo-300" },
  { value: "project_id", label: "Project ID (per day)", scope: "daily_column", color: "bg-cyan-100 text-cyan-800 border-cyan-300" },
  { value: "app_impacted", label: "App Impacted (per day)", scope: "daily_column", color: "bg-pink-100 text-pink-800 border-pink-300" },
  { value: "meta_name", label: "Employee Name (cell)", scope: "cell", color: "bg-rose-100 text-rose-800 border-rose-300" },
  { value: "meta_mii_id", label: "MII / NIP ID (cell)", scope: "cell", color: "bg-sky-100 text-sky-800 border-sky-300" },
  { value: "meta_division", label: "Division (cell)", scope: "cell", color: "bg-violet-100 text-violet-800 border-violet-300" },
  { value: "meta_site", label: "Site / Location (cell)", scope: "cell", color: "bg-orange-100 text-orange-800 border-orange-300" },
  { value: "meta_month", label: "Month (cell)", scope: "cell", color: "bg-yellow-100 text-yellow-800 border-yellow-300" },
  { value: "meta_year", label: "Year (cell)", scope: "cell", color: "bg-gray-100 text-gray-800 border-gray-300" },
];

// Company Presets for MII, SDD, NTT, and Adidata
const COMPANY_PRESETS: { name: string; company: string; mappings: CellMapping[] }[] = [
  {
    name: "MII Timesheet",
    company: "MII",
    mappings: [
      { field: "date", scope: "daily_column", column: "A", start_row: 9, fillable: false },
      { field: "time_in", scope: "daily_column", column: "B", start_row: 9, fillable: true },
      { field: "time_out", scope: "daily_column", column: "C", start_row: 9, fillable: true },
      { field: "status", scope: "daily_column", column: "E", start_row: 9, fillable: true },
      { field: "activity", scope: "daily_column", column: "K", start_row: 9, fillable: true },
      { field: "app_impacted", scope: "daily_column", column: "N", start_row: 9, fillable: true },
      { field: "meta_name", scope: "cell", cell_ref: "C3", fillable: false },
      { field: "meta_mii_id", scope: "cell", cell_ref: "C4", fillable: false },
      { field: "meta_division", scope: "cell", cell_ref: "C2", fillable: false },
      { field: "meta_site", scope: "cell", cell_ref: "C5", fillable: false },
    ],
  },
  {
    name: "SDD Timesheet",
    company: "SDD",
    mappings: [
      { field: "date", scope: "daily_column", column: "B", start_row: 12, fillable: false },
      { field: "time_in", scope: "daily_column", column: "C", start_row: 12, fillable: true },
      { field: "time_out", scope: "daily_column", column: "D", start_row: 12, fillable: true },
      { field: "status", scope: "daily_column", column: "F", start_row: 12, fillable: true },
      { field: "project_name", scope: "daily_column", column: "K", start_row: 12, fillable: true },
      { field: "project_id", scope: "daily_column", column: "L", start_row: 12, fillable: true },
      { field: "meta_name", scope: "cell", cell_ref: "F2", fillable: false },
      { field: "meta_mii_id", scope: "cell", cell_ref: "F3", fillable: false },
      { field: "meta_division", scope: "cell", cell_ref: "F4", fillable: false },
    ],
  },
  {
    name: "NTT Timesheet",
    company: "NTT",
    mappings: [
      { field: "date", scope: "daily_column", column: "A", start_row: 10, fillable: false },
      { field: "time_in", scope: "daily_column", column: "B", start_row: 10, fillable: true },
      { field: "time_out", scope: "daily_column", column: "C", start_row: 10, fillable: true },
      { field: "status", scope: "daily_column", column: "E", start_row: 10, fillable: true },
      { field: "activity", scope: "daily_column", column: "H", start_row: 10, fillable: true },
      { field: "meta_name", scope: "cell", cell_ref: "C3", fillable: false },
      { field: "meta_mii_id", scope: "cell", cell_ref: "C4", fillable: false },
    ],
  },
  {
    name: "Adidata Timesheet",
    company: "Adidata",
    mappings: [
      { field: "date", scope: "daily_column", column: "A", start_row: 8, fillable: false },
      { field: "time_in", scope: "daily_column", column: "B", start_row: 8, fillable: true },
      { field: "time_out", scope: "daily_column", column: "C", start_row: 8, fillable: true },
      { field: "status", scope: "daily_column", column: "D", start_row: 8, fillable: true },
      { field: "activity", scope: "daily_column", column: "F", start_row: 8, fillable: true },
      { field: "meta_name", scope: "cell", cell_ref: "B3", fillable: false },
      { field: "meta_division", scope: "cell", cell_ref: "B4", fillable: false },
    ],
  },
];

// Convert a 0-based column index to spreadsheet letters (0 -> A, 26 -> AA).
function colToLetter(col: number): string {
  let s = "";
  let n = col;
  do {
    s = String.fromCharCode(65 + (n % 26)) + s;
    n = Math.floor(n / 26) - 1;
  } while (n >= 0);
  return s;
}

// Auto-detect header fields from 2D Excel cell values.
function autoDetectMappings(grid: any[][]): CellMapping[] {
  const detected: CellMapping[] = [];
  if (!grid || !grid.length) return detected;

  let headerRowIndex = -1;
  // 1. Find daily column header row
  for (let r = 0; r < Math.min(grid.length, 20); r++) {
    const rowStr = grid[r].map((c) => String(c || "").toLowerCase()).join(" ");
    if (
      rowStr.includes("tgl") ||
      rowStr.includes("tanggal") ||
      rowStr.includes("date") ||
      rowStr.includes("masuk") ||
      rowStr.includes("activity")
    ) {
      headerRowIndex = r;
      break;
    }
  }

  const startRow = headerRowIndex !== -1 ? headerRowIndex + 2 : 9;

  if (headerRowIndex !== -1) {
    grid[headerRowIndex].forEach((val, colIndex) => {
      const text = String(val || "").trim().toLowerCase();
      const letter = colToLetter(colIndex);

      if (
        (text.includes("tgl") || text.includes("tanggal") || text.includes("date")) &&
        !detected.some((m) => m.field === "date")
      ) {
        detected.push({ field: "date", scope: "daily_column", column: letter, start_row: startRow, fillable: false });
      } else if (
        (text.includes("masuk") || text.includes("time in") || text.includes("start")) &&
        !detected.some((m) => m.field === "time_in")
      ) {
        detected.push({ field: "time_in", scope: "daily_column", column: letter, start_row: startRow, fillable: true });
      } else if (
        (text.includes("keluar") || text.includes("time out") || text.includes("end")) &&
        !detected.some((m) => m.field === "time_out")
      ) {
        detected.push({ field: "time_out", scope: "daily_column", column: letter, start_row: startRow, fillable: true });
      } else if (
        (text.includes("status") || text.includes("kehadiran")) &&
        !detected.some((m) => m.field === "status")
      ) {
        detected.push({ field: "status", scope: "daily_column", column: letter, start_row: startRow, fillable: true });
      } else if (
        (text.includes("aktivitas") || text.includes("activity") || text.includes("uraian") || text.includes("keterangan") || text.includes("task")) &&
        !detected.some((m) => m.field === "activity")
      ) {
        detected.push({ field: "activity", scope: "daily_column", column: letter, start_row: startRow, fillable: true });
      } else if (
        (text.includes("project") || text.includes("proyek")) &&
        !text.includes("id") &&
        !detected.some((m) => m.field === "project_name")
      ) {
        detected.push({ field: "project_name", scope: "daily_column", column: letter, start_row: startRow, fillable: true });
      } else if (
        (text.includes("aplikasi") || text.includes("app")) &&
        !detected.some((m) => m.field === "app_impacted")
      ) {
        detected.push({ field: "app_impacted", scope: "daily_column", column: letter, start_row: startRow, fillable: true });
      }
    });
  }

  // 2. Find static metadata header cells
  for (let r = 0; r < Math.min(grid.length, 10); r++) {
    grid[r].forEach((val, c) => {
      const text = String(val || "").trim().toLowerCase();
      const targetRef = `${colToLetter(c + 1)}${r + 1}`; // Cell immediately to the right

      if (
        (text.includes("nama") || text.includes("employee name")) &&
        !text.includes("proyek") &&
        !detected.some((m) => m.field === "meta_name")
      ) {
        detected.push({ field: "meta_name", scope: "cell", cell_ref: targetRef, fillable: false });
      } else if (
        (text.includes("mii id") || text.includes("nip") || text.includes("nik")) &&
        !detected.some((m) => m.field === "meta_mii_id")
      ) {
        detected.push({ field: "meta_mii_id", scope: "cell", cell_ref: targetRef, fillable: false });
      } else if (
        (text.includes("divisi") || text.includes("division") || text.includes("departemen")) &&
        !detected.some((m) => m.field === "meta_division")
      ) {
        detected.push({ field: "meta_division", scope: "cell", cell_ref: targetRef, fillable: false });
      } else if (
        (text.includes("site") || text.includes("lokasi") || text.includes("location")) &&
        !detected.some((m) => m.field === "meta_site")
      ) {
        detected.push({ field: "meta_site", scope: "cell", cell_ref: targetRef, fillable: false });
      }
    });
  }

  return detected;
}

export default function TemplateBuilderPage() {
  const { notify } = useToast();
  const [templates, setTemplates] = useState<Template[]>([]);
  const [activeId, setActiveId] = useState<number | null>(null);
  const [grid, setGrid] = useState<any[][]>([]);
  const [merges, setMerges] = useState<
    { row: number; col: number; rowspan: number; colspan: number }[]
  >([]);
  const [colWidths, setColWidths] = useState<number[] | undefined>(undefined);
  const [mappings, setMappings] = useState<CellMapping[]>([]);
  const [selection, setSelection] = useState<{ row: number; col: number } | null>(null);
  const [field, setField] = useState<MappingField>("date");
  const [fillable, setFillable] = useState(true);
  const [loadingGrid, setLoadingGrid] = useState(false);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);

  const loadTemplates = useCallback(async () => {
    try {
      const list = await api<Template[]>("/api/v1/templates");
      setTemplates(list);
      if (list.length && activeId === null) setActiveId(list[0].id);
    } catch (err: any) {
      notify(err.message, "error");
    }
  }, [notify, activeId]);

  useEffect(() => {
    loadTemplates();
  }, [loadTemplates]);

  // Load grid + existing mappings when active template changes.
  useEffect(() => {
    if (activeId === null) return;
    (async () => {
      setLoadingGrid(true);
      try {
        const [gridRes, tmpl] = await Promise.all([
          api<{
            grid: any[][];
            merges?: { row: number; col: number; rowspan: number; colspan: number }[];
            col_widths?: number[];
          }>(`/api/v1/templates/${activeId}/grid`),
          api<Template[]>("/api/v1/templates").then((all) => all.find((t) => t.id === activeId)),
        ]);
        setGrid(gridRes.grid || []);
        setMerges(gridRes.merges || []);
        setColWidths(gridRes.col_widths);
        setMappings(tmpl?.cell_mappings || []);
      } catch (err: any) {
        notify(err.message, "error");
      } finally {
        setLoadingGrid(false);
      }
    })();
  }, [activeId, notify]);

  const autoDetected = useMemo(() => autoDetectMappings(grid), [grid]);

  const applyAutoMapping = () => {
    if (!autoDetected.length) {
      notify("No header patterns detected automatically", "info");
      return;
    }
    setMappings((prev) => {
      const updated = [...prev];
      autoDetected.forEach((d) => {
        const idx = updated.findIndex((m) => m.field === d.field);
        if (idx >= 0) {
          updated[idx] = d;
        } else {
          updated.push(d);
        }
      });
      return updated;
    });
    notify(`Auto-applied ${autoDetected.length} detected mappings!`, "success");
  };

  const applyPreset = (preset: typeof COMPANY_PRESETS[0]) => {
    setMappings(preset.mappings);
    notify(`Applied ${preset.name} preset mapping`, "success");
  };

  const selectedField = useMemo(
    () => FIELD_OPTIONS.find((f) => f.value === field)!,
    [field]
  );

  const addMappingForSelection = (sel: { row: number; col: number }, selectedF: typeof selectedField) => {
    const { row, col } = sel;
    const letter = colToLetter(col);
    const rowNum = row + 1;

    const scope = selectedF.scope;
    const mapping: CellMapping =
      scope === "daily_column"
        ? { field: selectedF.value, scope, column: letter, start_row: rowNum, fillable }
        : { field: selectedF.value, scope, cell_ref: `${letter}${rowNum}`, fillable: false };

    setMappings((prev) => [...prev.filter((m) => m.field !== selectedF.value), mapping]);
    notify(`Mapped ${selectedF.value} → ${letter}${rowNum}`, "success");
  };

  const addMapping = () => {
    if (!selection) {
      notify("Click a cell in the grid first", "info");
      return;
    }
    addMappingForSelection(selection, selectedField);
  };

  const removeMapping = (f: MappingField) =>
    setMappings((prev) => prev.filter((m) => m.field !== f));

  const save = async () => {
    if (activeId === null) return;
    setSaving(true);
    try {
      await api(`/api/v1/admin/templates/${activeId}/mappings`, {
        method: "POST",
        body: JSON.stringify({ mappings }),
      });
      notify("Mapping saved successfully", "success");
      loadTemplates();
    } catch (err: any) {
      notify(err.message, "error");
    } finally {
      setSaving(false);
    }
  };

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const fd = new FormData();
      fd.append("file", file);
      fd.append("name", file.name.replace(/\.xlsx$/i, ""));
      const res = await fetch(`${API_BASE}/api/v1/admin/templates`, {
        method: "POST",
        headers: { Authorization: `Bearer ${getToken()}` },
        body: fd,
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(data.error);
      }
      const tmpl: Template = await res.json();
      notify("Template uploaded", "success");
      await loadTemplates();
      setActiveId(tmpl.id);
    } catch (err: any) {
      notify(err.message || "Upload failed", "error");
    } finally {
      setUploading(false);
      e.target.value = "";
    }
  };

  // Highlight mapped cells/columns in the grid.
  const mappedCells = useMemo(() => {
    const map = new Map<string, MappingField>();
    mappings.forEach((m) => {
      if (m.scope === "cell" && m.cell_ref) {
        map.set(m.cell_ref, m.field);
      }
    });
    return map;
  }, [mappings]);

  const mappedColumns = useMemo(() => {
    const map = new Map<string, MappingField>();
    mappings.forEach((m) => {
      if (m.scope === "daily_column" && m.column) map.set(m.column, m.field);
    });
    return map;
  }, [mappings]);

  const cells = useCallback(
    (row: number, col: number) => {
      const letter = colToLetter(col);
      const ref = `${letter}${row + 1}`;
      const meta: any = { readOnly: true };
      if (mappedCells.has(ref)) {
        meta.className = "ht-mapped-cell";
      } else if (mappedColumns.has(letter)) {
        meta.className = "ht-mapped-col";
      }
      return meta;
    },
    [mappedCells, mappedColumns]
  );

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-extrabold flex items-center gap-2">
            <Building2 className="text-mr-purple" size={28} /> Template Builder
          </h1>
          <p className="text-sm text-mr-muted">
            Upload company templates (MII, SDD, NTT, Adidata), then auto-detect or click cells to map fields.
          </p>
        </div>
        <label className="btn-accent cursor-pointer flex items-center gap-2">
          {uploading ? <Loader2 size={18} className="animate-spin" /> : <Upload size={18} />}
          Upload .xlsx Template
          <input type="file" accept=".xlsx" className="hidden" onChange={handleUpload} />
        </label>
      </header>

      {/* Template tabs & presets */}
      <div className="flex flex-wrap items-center justify-between gap-4 bg-mr-surface p-4 border border-mr-ink rounded-lg shadow-sm">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs font-bold uppercase tracking-wider text-mr-muted mr-1">Templates:</span>
          {templates.map((t) => (
            <button
              key={t.id}
              onClick={() => setActiveId(t.id)}
              className={`chip px-3 py-1.5 text-sm font-semibold flex items-center gap-1.5 transition-all ${
                activeId === t.id
                  ? "bg-mr-purple text-white shadow-sm"
                  : "bg-mr-surface2 text-mr-ink border border-mr-ink hover:bg-mr-purple/10"
              }`}
            >
              <Tag size={13} /> {t.name}
              {t.is_default && <span className="ml-1 text-[10px] bg-yellow-400 text-black px-1.5 py-0.5 rounded font-bold">DEFAULT</span>}
            </button>
          ))}
        </div>

        {/* Company Quick Presets */}
        {activeId !== null && (
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-xs font-bold uppercase tracking-wider text-mr-muted flex items-center gap-1">
              <Sparkles size={13} className="text-amber-500" /> Presets:
            </span>
            {COMPANY_PRESETS.map((p) => (
              <button
                key={p.company}
                onClick={() => applyPreset(p)}
                className="btn-ghost text-xs py-1 px-2.5 bg-white border border-mr-ink hover:bg-mr-yellow"
                title={`Apply 1-click mapping for ${p.name}`}
              >
                {p.company} Preset
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Auto-Detect Smart Banner */}
      {grid.length > 0 && autoDetected.length > 0 && (
        <div className="bg-gradient-to-r from-purple-500/10 via-indigo-500/10 to-blue-500/10 border-2 border-mr-purple p-4 rounded-xl flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-mr-purple text-white rounded-lg">
              <Wand2 size={20} />
            </div>
            <div>
              <h3 className="text-sm font-extrabold text-mr-ink flex items-center gap-1.5">
                Smart Auto-Detect Found {autoDetected.length} Header Fields!
              </h3>
              <p className="text-xs text-mr-muted">
                Detected: {autoDetected.map((d) => d.field).join(", ")}
              </p>
            </div>
          </div>
          <button onClick={applyAutoMapping} className="btn-accent text-xs font-bold py-2 px-4 flex items-center gap-1.5">
            <Sparkles size={14} /> Apply 1-Click Auto-Mapping
          </button>
        </div>
      )}

      {/* Active template actions */}
      {activeId !== null && templates.length > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            {!templates.find((t) => t.id === activeId)?.is_default && (
              <button
                onClick={async () => {
                  try {
                    await api(`/api/v1/admin/templates/${activeId}/default`, {
                      method: "POST",
                      body: JSON.stringify({}),
                    });
                    notify("Set as default template", "success");
                    loadTemplates();
                  } catch (err: any) {
                    notify(err.message, "error");
                  }
                }}
                className="btn-ghost text-xs py-1.5"
              >
                <Star size={14} /> Set as Default Template
              </button>
            )}
          </div>
          <button
            onClick={async () => {
              const t = templates.find((x) => x.id === activeId);
              if (!t || !confirm(`Delete template "${t.name}"? This removes its mappings too.`)) return;
              try {
                await api(`/api/v1/admin/templates/${activeId}`, { method: "DELETE" });
                notify("Template deleted", "success");
                setActiveId(null);
                loadTemplates();
              } catch (err: any) {
                notify(err.message, "error");
              }
            }}
            className="btn-danger text-xs py-1.5"
          >
            <Trash2 size={14} /> Delete Template
          </button>
        </div>
      )}

      {/* Visual Field Palette Bar */}
      {grid.length > 0 && (
        <div className="bg-mr-surface p-3 border border-mr-ink rounded-lg flex flex-col gap-2">
          <span className="text-xs font-extrabold uppercase tracking-wider text-mr-muted flex items-center gap-1.5">
            <MousePointerClick size={14} className="text-mr-purple" /> Quick Visual Palette (Click a field to select, then click cell in grid):
          </span>
          <div className="flex flex-wrap gap-1.5">
            {FIELD_OPTIONS.map((f) => {
              const isSelected = field === f.value;
              const isMapped = mappings.some((m) => m.field === f.value);
              return (
                <button
                  key={f.value}
                  onClick={() => {
                    setField(f.value);
                    if (selection) {
                      addMappingForSelection(selection, f);
                    }
                  }}
                  className={`text-xs px-2.5 py-1 rounded-md border transition-all flex items-center gap-1 font-medium ${f.color} ${
                    isSelected ? "ring-2 ring-mr-purple font-bold scale-105 shadow-sm" : "opacity-85 hover:opacity-100"
                  }`}
                >
                  {isMapped && <CheckCircle2 size={12} className="text-emerald-600" />}
                  {f.label}
                </button>
              );
            })}
          </div>
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-4">
        {/* Handsontable Grid */}
        <div className="card p-4 lg:col-span-3">
          {loadingGrid ? (
            <div className="flex justify-center py-16">
              <Loader2 className="animate-spin text-mr-purple" />
            </div>
          ) : grid.length ? (
            <HotGrid
              data={grid}
              readOnly
              height={560}
              cells={cells}
              mergeCells={merges.length ? merges : undefined}
              colWidths={colWidths}
              afterSelectionEnd={(r, c) => setSelection({ row: r, col: c })}
            />
          ) : (
            <div className="flex flex-col items-center justify-center gap-2 py-16 text-center text-mr-muted">
              <MousePointerClick size={32} />
              <p>Upload a template or select an existing template to begin mapping.</p>
            </div>
          )}
        </div>

        {/* Mapping & Controls Panel */}
        <div className="flex flex-col gap-4 lg:col-span-1">
          <div className="card p-5">
            <h2 className="mb-3 text-lg font-bold">Assign Selected Field</h2>
            <div className="mb-3 bg-mr-surface2 p-3 text-sm rounded border border-mr-ink/20">
              {selection ? (
                <span className="font-semibold text-mr-purple">
                  Selected Cell: {colToLetter(selection.col)}
                  {selection.row + 1}
                </span>
              ) : (
                <span className="text-mr-muted">Click any cell in grid to select…</span>
              )}
            </div>

            <label className="mb-1 block text-sm font-semibold">Target Variable</label>
            <select
              className="input mb-3"
              value={field}
              onChange={(e) => setField(e.target.value as MappingField)}
            >
              {FIELD_OPTIONS.map((f) => (
                <option key={f.value} value={f.value}>
                  {f.label}
                </option>
              ))}
            </select>

            {selectedField.scope === "daily_column" && (
              <label className="mb-3 flex items-center gap-2 text-sm font-medium">
                <input
                  type="checkbox"
                  checked={fillable}
                  onChange={(e) => setFillable(e.target.checked)}
                  className="h-4 w-4 rounded accent-mr-purple"
                />
                Users may edit this column
              </label>
            )}

            <p className="mb-3 text-xs text-mr-muted">
              {selectedField.scope === "daily_column"
                ? "Anchors day 1 at the selected row; subsequent days fill downward."
                : "Maps a single fixed cell (header metadata)."}
            </p>

            <button onClick={addMapping} className="btn-primary w-full flex items-center justify-center gap-2">
              <MousePointerClick size={16} /> Map Selected Cell
            </button>
          </div>

          <div className="card p-5">
            <div className="mb-3 flex items-center justify-between">
              <h2 className="text-lg font-bold">Active Mappings</h2>
              <span className="chip bg-mr-purple text-white px-2 py-0.5 font-bold text-xs">{mappings.length}</span>
            </div>
            <div className="flex flex-col gap-2 max-h-[320px] overflow-y-auto pr-1">
              {mappings.length === 0 && (
                <p className="text-sm text-mr-muted">No mappings defined yet.</p>
              )}
              {mappings.map((m) => (
                <div
                  key={m.field}
                  className="flex items-center justify-between bg-mr-surface2 px-3 py-2 text-sm rounded border border-mr-ink/10"
                >
                  <div>
                    <p className="font-semibold text-mr-ink">{m.field}</p>
                    <p className="text-xs text-mr-muted">
                      {m.scope === "daily_column"
                        ? `Col ${m.column} · Row ${m.start_row}${m.fillable ? " · fillable" : ""}`
                        : `Cell ${m.cell_ref}`}
                    </p>
                  </div>
                  <button
                    onClick={() => removeMapping(m.field)}
                    className="p-1 text-mr-muted hover:bg-mr-pink hover:text-white rounded transition-all"
                    title="Remove mapping"
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              ))}
            </div>

            <button
              onClick={save}
              disabled={saving || activeId === null}
              className="btn-primary mt-4 w-full flex items-center justify-center gap-2"
            >
              {saving ? <Loader2 size={16} className="animate-spin" /> : <Save size={16} />}
              Save All Mappings
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

