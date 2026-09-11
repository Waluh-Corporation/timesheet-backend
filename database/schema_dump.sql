--
-- PostgreSQL database dump
--

\restrict BH3x013drdUpJrEB9UKsnc3WqrKzcPQ2efwSfbM7G250gjtotdIhVDO1GuOBtmH

-- Dumped from database version 16.15
-- Dumped by pg_dump version 16.15

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: activity_statuses; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.activity_statuses (
    code character varying(8) NOT NULL,
    name character varying(64) NOT NULL,
    description character varying(255),
    is_working_day boolean DEFAULT true NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL
);


--
-- Name: approvers; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.approvers (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    name character varying(255) NOT NULL,
    role_type character varying(32) NOT NULL,
    title character varying(128),
    is_active boolean DEFAULT true NOT NULL
);


--
-- Name: approvers_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.approvers_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: approvers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.approvers_id_seq OWNED BY public.approvers.id;


--
-- Name: companies; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.companies (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    code character varying(32) NOT NULL,
    name character varying(255) NOT NULL
);


--
-- Name: companies_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.companies_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: companies_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.companies_id_seq OWNED BY public.companies.id;


--
-- Name: daily_activities; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.daily_activities (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    user_id bigint NOT NULL,
    date date NOT NULL,
    start_time character varying(8),
    end_time character varying(8),
    status character varying(8),
    activity text,
    project_name character varying(255),
    project_id character varying(64),
    project_ref_id bigint
);


--
-- Name: daily_activities_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.daily_activities_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: daily_activities_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.daily_activities_id_seq OWNED BY public.daily_activities.id;


--
-- Name: departments; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.departments (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    company_id bigint,
    code character varying(64),
    name character varying(255) NOT NULL,
    division character varying(255),
    is_active boolean DEFAULT true NOT NULL
);


--
-- Name: departments_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.departments_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: departments_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.departments_id_seq OWNED BY public.departments.id;


--
-- Name: holidays; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.holidays (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    date date NOT NULL,
    description character varying(255) NOT NULL,
    is_joint_leave boolean DEFAULT false NOT NULL,
    is_civic boolean DEFAULT false NOT NULL,
    is_religious boolean DEFAULT false NOT NULL
);


--
-- Name: holidays_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.holidays_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: holidays_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.holidays_id_seq OWNED BY public.holidays.id;


--
-- Name: overtime_entries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.overtime_entries (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    user_id bigint NOT NULL,
    daily_activity_id bigint,
    date date NOT NULL,
    start_time character varying(8),
    end_time character varying(8),
    task_description text NOT NULL,
    team_leader_id bigint,
    department_head_id bigint
);


--
-- Name: overtime_entries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.overtime_entries_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: overtime_entries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.overtime_entries_id_seq OWNED BY public.overtime_entries.id;


--
-- Name: password_reset_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.password_reset_tokens (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    user_id bigint NOT NULL,
    token_hash character varying(64) NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    token_type character varying(32) DEFAULT 'password_reset'::character varying NOT NULL,
    used_at timestamp with time zone,
    created_ip character varying(45),
    used_ip character varying(45)
);


--
-- Name: password_reset_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.password_reset_tokens_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: password_reset_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.password_reset_tokens_id_seq OWNED BY public.password_reset_tokens.id;


--
-- Name: profile_change_requests; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.profile_change_requests (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    user_id bigint NOT NULL,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    name character varying(255),
    bni_id character varying(64),
    employee_id character varying(64),
    division character varying(255),
    department character varying(255),
    group_name character varying(255),
    "position" character varying(128),
    site character varying(128),
    reviewed_by bigint,
    reviewed_at timestamp with time zone,
    company_id bigint,
    department_id bigint
);


--
-- Name: profile_change_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.profile_change_requests_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: profile_change_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.profile_change_requests_id_seq OWNED BY public.profile_change_requests.id;


--
-- Name: projects; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.projects (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    code character varying(64) NOT NULL,
    name character varying(255) NOT NULL,
    app_impacted character varying(255),
    is_active boolean DEFAULT true NOT NULL
);


--
-- Name: projects_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.projects_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: projects_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.projects_id_seq OWNED BY public.projects.id;


--
-- Name: push_subscriptions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.push_subscriptions (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    user_id bigint NOT NULL,
    endpoint character varying(512) NOT NULL,
    p256dh character varying(255) NOT NULL,
    auth character varying(255) NOT NULL
);


--
-- Name: push_subscriptions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.push_subscriptions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: push_subscriptions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.push_subscriptions_id_seq OWNED BY public.push_subscriptions.id;


--
-- Name: schema_migrations; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.schema_migrations (
    version bigint NOT NULL,
    name character varying(255) NOT NULL,
    applied_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: system_settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.system_settings (
    key character varying(64) NOT NULL,
    value text NOT NULL,
    updated_at timestamp with time zone DEFAULT now()
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    username character varying(64) NOT NULL,
    email character varying(255) NOT NULL,
    password_hash character varying(255),
    role character varying(16) DEFAULT 'user'::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    name character varying(255),
    bni_id character varying(64),
    employee_id character varying(64),
    division character varying(255),
    department character varying(255),
    group_name character varying(255),
    "position" character varying(128),
    site character varying(128),
    company character varying(64),
    company_id bigint,
    department_id bigint
);


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: web_authn_credentials; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.web_authn_credentials (
    id bigint NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    user_id bigint NOT NULL,
    credential_id bytea NOT NULL,
    public_key bytea NOT NULL,
    attestation_type character varying(64),
    aaguid bytea,
    sign_count bigint DEFAULT 0 NOT NULL,
    clone_warning boolean DEFAULT false NOT NULL,
    backup_eligible boolean DEFAULT false NOT NULL,
    backup_state boolean DEFAULT false NOT NULL,
    transports jsonb,
    friendly_name character varying(128)
);


--
-- Name: web_authn_credentials_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.web_authn_credentials_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: web_authn_credentials_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.web_authn_credentials_id_seq OWNED BY public.web_authn_credentials.id;


--
-- Name: approvers id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approvers ALTER COLUMN id SET DEFAULT nextval('public.approvers_id_seq'::regclass);


--
-- Name: companies id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.companies ALTER COLUMN id SET DEFAULT nextval('public.companies_id_seq'::regclass);


--
-- Name: daily_activities id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_activities ALTER COLUMN id SET DEFAULT nextval('public.daily_activities_id_seq'::regclass);


--
-- Name: departments id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.departments ALTER COLUMN id SET DEFAULT nextval('public.departments_id_seq'::regclass);


--
-- Name: holidays id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.holidays ALTER COLUMN id SET DEFAULT nextval('public.holidays_id_seq'::regclass);


--
-- Name: overtime_entries id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.overtime_entries ALTER COLUMN id SET DEFAULT nextval('public.overtime_entries_id_seq'::regclass);


--
-- Name: password_reset_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens ALTER COLUMN id SET DEFAULT nextval('public.password_reset_tokens_id_seq'::regclass);


--
-- Name: profile_change_requests id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profile_change_requests ALTER COLUMN id SET DEFAULT nextval('public.profile_change_requests_id_seq'::regclass);


--
-- Name: projects id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.projects ALTER COLUMN id SET DEFAULT nextval('public.projects_id_seq'::regclass);


--
-- Name: push_subscriptions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_subscriptions ALTER COLUMN id SET DEFAULT nextval('public.push_subscriptions_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: web_authn_credentials id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_authn_credentials ALTER COLUMN id SET DEFAULT nextval('public.web_authn_credentials_id_seq'::regclass);


--
-- Name: activity_statuses activity_statuses_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.activity_statuses
    ADD CONSTRAINT activity_statuses_pkey PRIMARY KEY (code);


--
-- Name: approvers approvers_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.approvers
    ADD CONSTRAINT approvers_pkey PRIMARY KEY (id);


--
-- Name: companies companies_code_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.companies
    ADD CONSTRAINT companies_code_key UNIQUE (code);


--
-- Name: companies companies_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.companies
    ADD CONSTRAINT companies_pkey PRIMARY KEY (id);


--
-- Name: daily_activities daily_activities_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_activities
    ADD CONSTRAINT daily_activities_pkey PRIMARY KEY (id);


--
-- Name: departments departments_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.departments
    ADD CONSTRAINT departments_pkey PRIMARY KEY (id);


--
-- Name: holidays holidays_date_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.holidays
    ADD CONSTRAINT holidays_date_key UNIQUE (date);


--
-- Name: holidays holidays_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.holidays
    ADD CONSTRAINT holidays_pkey PRIMARY KEY (id);


--
-- Name: overtime_entries overtime_entries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.overtime_entries
    ADD CONSTRAINT overtime_entries_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_token_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_token_hash_key UNIQUE (token_hash);


--
-- Name: profile_change_requests profile_change_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profile_change_requests
    ADD CONSTRAINT profile_change_requests_pkey PRIMARY KEY (id);


--
-- Name: projects projects_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_pkey PRIMARY KEY (id);


--
-- Name: push_subscriptions push_subscriptions_endpoint_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_subscriptions
    ADD CONSTRAINT push_subscriptions_endpoint_key UNIQUE (endpoint);


--
-- Name: push_subscriptions push_subscriptions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_subscriptions
    ADD CONSTRAINT push_subscriptions_pkey PRIMARY KEY (id);


--
-- Name: schema_migrations schema_migrations_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.schema_migrations
    ADD CONSTRAINT schema_migrations_pkey PRIMARY KEY (version);


--
-- Name: system_settings system_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.system_settings
    ADD CONSTRAINT system_settings_pkey PRIMARY KEY (key);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: web_authn_credentials web_authn_credentials_credential_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_authn_credentials
    ADD CONSTRAINT web_authn_credentials_credential_id_key UNIQUE (credential_id);


--
-- Name: web_authn_credentials web_authn_credentials_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_authn_credentials
    ADD CONSTRAINT web_authn_credentials_pkey PRIMARY KEY (id);


--
-- Name: idx_approvers_role_type_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_approvers_role_type_active ON public.approvers USING btree (role_type) WHERE (is_active = true);


--
-- Name: idx_daily_activities_project_ref_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_daily_activities_project_ref_id ON public.daily_activities USING btree (project_ref_id);


--
-- Name: idx_daily_activities_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_daily_activities_status ON public.daily_activities USING btree (status);


--
-- Name: idx_daily_activities_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_daily_activities_user_id ON public.daily_activities USING btree (user_id);


--
-- Name: idx_departments_code; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_departments_code ON public.departments USING btree (code);


--
-- Name: idx_departments_company_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_departments_company_active ON public.departments USING btree (company_id, is_active);


--
-- Name: idx_departments_company_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_departments_company_id ON public.departments USING btree (company_id);



--
-- Name: idx_holidays_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_holidays_date ON public.holidays USING btree (date);

--
-- Name: idx_holidays_joint_leave; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_holidays_joint_leave ON public.holidays USING btree (is_joint_leave);


--
-- Name: idx_overtime_entries_user_date; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_overtime_entries_user_date ON public.overtime_entries USING btree (user_id, date);


--
-- Name: idx_overtimes_daily_activity_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_overtimes_daily_activity_id ON public.overtime_entries USING btree (daily_activity_id);


--
-- Name: idx_overtimes_department_head_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_overtimes_department_head_id ON public.overtime_entries USING btree (department_head_id);


--
-- Name: idx_overtimes_team_leader_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_overtimes_team_leader_id ON public.overtime_entries USING btree (team_leader_id);


--
-- Name: idx_overtimes_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_overtimes_user_id ON public.overtime_entries USING btree (user_id);


--
-- Name: idx_password_reset_tokens_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_password_reset_tokens_active ON public.password_reset_tokens USING btree (user_id) WHERE (used_at IS NULL);


--
-- Name: idx_password_reset_tokens_expires_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_password_reset_tokens_expires_at ON public.password_reset_tokens USING btree (expires_at);


--
-- Name: idx_password_reset_tokens_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_password_reset_tokens_user_id ON public.password_reset_tokens USING btree (user_id);


--
-- Name: idx_profile_change_requests_company_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_profile_change_requests_company_id ON public.profile_change_requests USING btree (company_id);


--
-- Name: idx_profile_change_requests_department_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_profile_change_requests_department_id ON public.profile_change_requests USING btree (department_id);


--
-- Name: idx_profile_change_requests_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_profile_change_requests_user_id ON public.profile_change_requests USING btree (user_id);


--
-- Name: idx_projects_code; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_projects_code ON public.projects USING btree (code);


--
-- Name: idx_projects_company_active; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_projects_company_active ON public.projects USING btree (company_id, is_active);


--
-- Name: idx_projects_company_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_projects_company_id ON public.projects USING btree (company_id);


--
-- Name: idx_push_subscriptions_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_push_subscriptions_user_id ON public.push_subscriptions USING btree (user_id);


--
-- Name: idx_user_date; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_user_date ON public.daily_activities USING btree (user_id, date);


--
-- Name: idx_users_company_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_company_id ON public.users USING btree (company_id);


--
-- Name: idx_users_deleted_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_deleted_at ON public.users USING btree (deleted_at);


--
-- Name: idx_users_department_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_department_id ON public.users USING btree (department_id);


--
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_users_email ON public.users USING btree (email) WHERE (deleted_at IS NULL);


--
-- Name: idx_users_username; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_users_username ON public.users USING btree (username) WHERE (deleted_at IS NULL);


--
-- Name: idx_webauthn_credentials_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_webauthn_credentials_user_id ON public.web_authn_credentials USING btree (user_id);


--
-- Name: daily_activities daily_activities_project_ref_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_activities
    ADD CONSTRAINT daily_activities_project_ref_id_fkey FOREIGN KEY (project_ref_id) REFERENCES public.projects(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: departments departments_company_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.departments
    ADD CONSTRAINT departments_company_id_fkey FOREIGN KEY (company_id) REFERENCES public.companies(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: daily_activities fk_daily_activities_status; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_activities
    ADD CONSTRAINT fk_daily_activities_status FOREIGN KEY (status) REFERENCES public.activity_statuses(code) ON UPDATE CASCADE ON DELETE RESTRICT;


--
-- Name: daily_activities fk_daily_activities_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.daily_activities
    ADD CONSTRAINT fk_daily_activities_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: overtime_entries fk_overtime_entries_department_head; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.overtime_entries
    ADD CONSTRAINT fk_overtime_entries_department_head FOREIGN KEY (department_head_id) REFERENCES public.approvers(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: overtime_entries fk_overtime_entries_team_leader; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.overtime_entries
    ADD CONSTRAINT fk_overtime_entries_team_leader FOREIGN KEY (team_leader_id) REFERENCES public.approvers(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: profile_change_requests fk_profile_change_requests_company_rel; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profile_change_requests
    ADD CONSTRAINT fk_profile_change_requests_company_rel FOREIGN KEY (company_id) REFERENCES public.companies(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: profile_change_requests fk_profile_change_requests_reviewer; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profile_change_requests
    ADD CONSTRAINT fk_profile_change_requests_reviewer FOREIGN KEY (reviewed_by) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE SET NULL;



--
-- Name: overtime_entries overtime_entries_daily_activity_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.overtime_entries
    ADD CONSTRAINT overtime_entries_daily_activity_id_fkey FOREIGN KEY (daily_activity_id) REFERENCES public.daily_activities(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: overtime_entries overtime_entries_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.overtime_entries
    ADD CONSTRAINT overtime_entries_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: password_reset_tokens password_reset_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: profile_change_requests profile_change_requests_department_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profile_change_requests
    ADD CONSTRAINT profile_change_requests_department_id_fkey FOREIGN KEY (department_id) REFERENCES public.departments(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: profile_change_requests profile_change_requests_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.profile_change_requests
    ADD CONSTRAINT profile_change_requests_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: projects projects_company_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_company_id_fkey FOREIGN KEY (company_id) REFERENCES public.companies(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: push_subscriptions push_subscriptions_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.push_subscriptions
    ADD CONSTRAINT push_subscriptions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- Name: users users_company_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_company_id_fkey FOREIGN KEY (company_id) REFERENCES public.companies(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: users users_department_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_department_id_fkey FOREIGN KEY (department_id) REFERENCES public.departments(id) ON UPDATE CASCADE ON DELETE SET NULL;


--
-- Name: web_authn_credentials web_authn_credentials_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.web_authn_credentials
    ADD CONSTRAINT web_authn_credentials_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON UPDATE CASCADE ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict BH3x013drdUpJrEB9UKsnc3WqrKzcPQ2efwSfbM7G250gjtotdIhVDO1GuOBtmH

