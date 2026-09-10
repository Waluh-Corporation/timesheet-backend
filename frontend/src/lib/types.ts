export type Role = "admin" | "user";

export interface Company {
  id: number;
  code: string;
  name: string;
  created_at?: string;
}

export interface Department {
  id: number;
  code?: string;
  name: string;
  division?: string;
  company_id?: number;
  is_active: boolean;
}

export interface Project {
  id: number;
  code: string;
  name: string;
  app_impacted?: string;
  company_id?: number;
  is_active: boolean;
}

export interface ActivityStatus {
  code: string;
  name: string;
  description?: string;
  is_working_day: boolean;
  sort_order: number;
}

export interface Holiday {
  id?: number;
  date: string;
  description: string;
  company_id?: number;
  is_joint_leave?: boolean;
}

export interface User {
  id: number;
  username: string;
  email: string;
  role: Role;
  is_active: boolean;
  name: string;
  mii_id: string;
  division: string;
  department?: string;
  department_id?: number;
  department_rel?: Department;
  site: string;
  company?: string;
  company_id?: number;
  company_rel?: Company;
  created_at: string;
}

export interface DailyActivity {
  id?: number;
  date: string; // YYYY-MM-DD
  start_time: string;
  end_time: string;
  status: string;
  activity: string;
  project_name: string;
  project_id: string;
  app_impacted: string;
  project_ref_id?: number;
  project_ref?: Project;
  status_ref?: ActivityStatus;
}

export interface Passkey {
  id: number;
  user_id: number;
  friendly_name: string;
  created_at: string;
}

export interface ProfileChangeRequest {
  id: number;
  user_id: number;
  status: "pending" | "approved" | "rejected";
  name: string;
  mii_id: string;
  division: string;
  department?: string;
  department_id?: number;
  site: string;
  company_id?: number;
  reviewed_by?: number;
  reviewed_at?: string;
  created_at: string;
  user?: User;
}

export interface Approver {
  id: number;
  name: string;
  role_type: "team_leader" | "department_head";
  title?: string;
  is_active: boolean;
}

export interface OvertimeEntry {
  id?: number;
  user_id?: number;
  daily_activity_id?: number;
  date: string;
  start_time: string;
  end_time: string;
  task_description: string;
  team_leader_id?: number;
  department_head_id?: number;
  team_leader?: Approver;
  department_head?: Approver;
}

