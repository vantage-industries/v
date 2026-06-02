export interface Network {
	slug: string;
	name: string;
	ssid: string;
	zone: string;
	band: string;
	auth_mode: string;
	enabled: boolean;
}

export interface Credential {
	id: string;
	device_id: string;
	kind: string;
	secret: string;
	active: boolean;
	created_at: string;
	revoked_at?: string;
	qr_payload: string;
}

export interface Device {
	id: string;
	name: string;
	network_slug: string;
	network_name: string;
	join_state: string;
	enrolled_via?: string;
	first_seen_at?: string;
	last_seen_at?: string;
	rx_bytes: number;
	tx_bytes: number;
	created_at: string;
	credential_count: number;
}

export interface RecoveryState {
	stage: "idle" | "listening" | "active";
	active: boolean;
	press_count: number;
	required_presses: number;
	press_window_expires_at?: string;
	recovery_expires_at?: string;
}

export interface AdminStatus {
	password_set: boolean;
	authenticated: boolean;
	recovery: RecoveryState;
}

export interface ConfigRevision {
	id: string;
	revision: number;
	title: string;
	note: string;
	status: string;
	active: boolean;
	created_at: string;
	snapshot: {
		networks?: Network[];
		devices?: Device[];
		credentials?: Credential[];
	};
}

export interface ServiceStatus {
	name: string;
	state: string;
}

export interface BootstrapSetup {
	state: string;
	state_label?: string;
	needs_setup: boolean;
	next_action: string;
	last_transition_at?: string;
	checklist: Array<{ key: string; label: string; done: boolean }>;
}

export interface BootstrapResponse {
	product_name: string;
	version: string;
	ui_mode: string;
	launch_band: string;
	setup: BootstrapSetup;
	password_set: boolean;
	recovery: RecoveryState;
}

export interface PendingRollback {
	revision: number;
	expires_in_ms: number;
}

export interface StatusResponse {
	timestamp: number;
	services: ServiceStatus[];
	networks: Network[];
	totals: { device_count: number };
	latest_events?: Array<{ id: string; kind: string; created_at: string }>;
	active_revision?: number;
	setup: BootstrapSetup;
	bootstrap_state: { state: string };
	password_set: boolean;
	recovery: RecoveryState;
	tailscale?: TailscaleResponse;
	suricata?: SuricataResponse;
	pending_rollback?: PendingRollback;
}

export interface TailscaleResponse {
	enabled: boolean;
	state: string;
	hostname?: string;
	advertise_routes?: string[];
	requested_at?: string;
}

export interface SuricataResponse {
	enabled: boolean;
	state: string;
	alerts_total?: number;
	packets_total?: number;
	updated_at?: string;
}

export interface DataPoint {
	timestamp: number;
	value: number;
}

export interface SuricataSnapshot {
	alerts_total: number;
	packets_total: number;
	series: DataPoint[];
}

export interface TrafficSnapshot {
	rx_series: DataPoint[];
	tx_series: DataPoint[];
}

export type Step = "welcome" | "password" | "network" | "onboard" | "apply";
