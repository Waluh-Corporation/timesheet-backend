"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth";
import ThemeToggle from "@/components/ThemeToggle";
import {
  LayoutDashboard,
  Users,
  LogOut,
  CalendarCheck,
  UserRound,
  type LucideIcon,
} from "lucide-react";

interface NavItem {
  href: string;
  label: string;
  icon: LucideIcon;
}

const adminNav: NavItem[] = [
  { href: "/users", label: "Users", icon: Users },
  { href: "/profile", label: "Profile", icon: UserRound },
];

const userNav: NavItem[] = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/profile", label: "Profile", icon: UserRound },
];

// Shell provides the friendly, rounded sidebar + top bar used by all
// authenticated pages.
export default function Shell({ children }: { children: React.ReactNode }) {
  const { user, logout } = useAuth();
  const pathname = usePathname();
  const router = useRouter();

  const nav = user?.role === "admin" ? adminNav : userNav;

  const handleLogout = () => {
    logout();
    router.replace("/login");
  };

  return (
    <div className="min-h-screen">
      <div className="mx-auto flex max-w-7xl gap-6 p-4 md:p-6">
        {/* Sidebar */}
        <aside className="hidden w-64 shrink-0 md:block">
          <div className="card sticky top-6 p-5">
            <div className="mb-6 flex items-center gap-2">
              <div className="grid h-10 w-10 place-items-center bg-mr-yellow text-black">
                <CalendarCheck size={20} />
              </div>
              <div>
                <p className="text-sm font-extrabold leading-tight">Timesheet</p>
                <p className="text-xs text-mr-muted">Portal</p>
              </div>
            </div>

            <nav className="flex flex-col gap-1">
              {nav.map((item) => {
                const active = pathname === item.href;
                const Icon = item.icon;
                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={`flex items-center gap-3  px-4 py-2.5 text-sm font-semibold transition ${
                      active
                        ? "bg-mr-purple text-white shadow-hard-sm"
                        : "text-mr-ink hover:bg-mr-surface2"
                    }`}
                  >
                    <Icon size={18} />
                    {item.label}
                  </Link>
                );
              })}
            </nav>

            <div className="mt-6 border-t border-mr-ink pt-4">
              <div className="mb-3 flex items-center gap-3 px-2">
                <div className="grid h-9 w-9 place-items-center rounded-full border-2 border-mr-ink bg-mr-yellow text-sm font-bold text-mr-ink">
                  {(user?.name || user?.username || "?").charAt(0).toUpperCase()}
                </div>
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold">{user?.name || user?.username}</p>
                  <p className="truncate text-xs capitalize text-mr-muted">{user?.role}</p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <button onClick={handleLogout} className="btn-ghost flex-1 text-sm">
                  <LogOut size={16} /> Sign out
                </button>
                <ThemeToggle />
              </div>
            </div>
          </div>
        </aside>

        {/* Main */}
        <main className="min-w-0 flex-1">
          {/* Mobile top bar */}
          <div className="card mb-4 flex items-center justify-between p-3 md:hidden">
            <span className="font-extrabold uppercase">Timesheet Portal</span>
            <div className="flex items-center gap-2">
              <ThemeToggle />
              <button onClick={handleLogout} className="btn-ghost text-sm">
                <LogOut size={16} />
              </button>
            </div>
          </div>
          {children}
        </main>
      </div>
    </div>
  );
}
