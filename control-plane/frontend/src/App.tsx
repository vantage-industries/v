import { QRCodeSVG } from "qrcode.react";
import {
	type FormEvent,
	useCallback,
	useEffect,
	useMemo,
	useRef,
	useState,
} from "react";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import {
	type ChartConfig,
	ChartContainer,
	ChartLegend,
	ChartLegendContent,
	ChartTooltip,
	ChartTooltipContent,
} from "@/components/ui/chart";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { getJson, postJson } from "./api";
import SetupWizard from "./SetupWizard";
import type {
	AdminStatus,
	BootstrapResponse,
	ConfigRevision,
	Credential,
	DataPoint,
	Device,
	Network,
	PendingRollback,
	RecoveryState,
	StatusResponse,
	SuricataResponse,
	SuricataSnapshot,
	TrafficSnapshot,
} from "./types";

const initialForm = { name: "", network_slug: "main", include_fallback: true };
const TAP_TOTAL = 10;

function formatDate(value: string | undefined): string {
	if (!value) return "never";
	return new Date(value).toLocaleString();
}

interface PinholeVizProps {
	pressCount: number;
	requiredPresses: number;
	stage: RecoveryState["stage"];
	windowExpiresAt?: string;
}

function PinholeViz({
	pressCount,
	requiredPresses,
	stage,
	windowExpiresAt,
}: PinholeVizProps) {
	const remaining = useMemo(() => {
		if (!windowExpiresAt) return "";
		const diff = new Date(windowExpiresAt).getTime() - Date.now();
		if (diff <= 0) return "0s";
		return `${Math.ceil(diff / 1000)}s`;
	}, [windowExpiresAt]);

	const dotIds = useMemo(
		() => Array.from({ length: requiredPresses }, (_, idx) => `dot-${idx}`),
		[requiredPresses],
	);

	return (
		<div className="flex flex-col items-center gap-4">
			<div className="relative flex justify-center">
				<svg
					viewBox="0 0 160 120"
					className="w-40 h-30"
					role="img"
					aria-label="Router pinhole reset button illustration"
				>
					<rect
						x="10"
						y="20"
						width="140"
						height="80"
						rx="12"
						fill="none"
						stroke="var(--border)"
						strokeWidth="2"
					/>
					<circle
						cx="80"
						cy="60"
						r="8"
						fill={
							stage === "active" ? "var(--warn)" : "var(--muted-foreground)"
						}
						className="transition-colors duration-300"
					/>
					<rect
						x="25"
						y="35"
						width="20"
						height="14"
						rx="3"
						fill="none"
						stroke="var(--border)"
						strokeWidth="1.5"
					/>
					<rect
						x="50"
						y="35"
						width="20"
						height="14"
						rx="3"
						fill="none"
						stroke="var(--border)"
						strokeWidth="1.5"
					/>
					<rect
						x="75"
						y="35"
						width="60"
						height="14"
						rx="3"
						fill="none"
						stroke="var(--border)"
						strokeWidth="1.5"
					/>
					<path
						d="M 30 60 L 130 60"
						stroke="var(--border)"
						strokeWidth="1"
						opacity="0.4"
					/>
				</svg>
				{stage === "active" && (
					<span className="absolute -bottom-1 left-1/2 -translate-x-1/2 text-[11px] font-bold uppercase tracking-wider whitespace-nowrap rounded-full bg-warn/15 text-warn px-2.5 py-1">
						Recovery active
					</span>
				)}
				{stage === "listening" && (
					<span className="absolute -bottom-1 left-1/2 -translate-x-1/2 text-[11px] font-bold uppercase tracking-wider whitespace-nowrap rounded-full bg-muted text-muted-foreground px-2.5 py-1">
						Tapping detected
					</span>
				)}
			</div>

			<div className="flex flex-col items-center gap-2.5">
				<p className="text-sm text-muted-foreground text-center max-w-xs">
					{stage === "active"
						? "Recovery mode is active. Set a new password below."
						: "Press the pinhole reset button on your router 10 times quickly"}
				</p>

				<div className="flex gap-2">
					{dotIds.map((id, idx) => (
						<div
							key={id}
							className={`w-5 h-5 rounded-full border transition-all duration-200 ${
								idx < pressCount
									? "bg-primary border-primary shadow-[0_0_8px_rgba(79,140,255,0.4)]"
									: "bg-muted border-border"
							}`}
						/>
					))}
				</div>

				<div className="flex items-center gap-2.5 text-muted-foreground">
					<span className="text-lg font-bold">
						{pressCount} / {requiredPresses}
					</span>
					{stage === "listening" && remaining && (
						<span className="text-sm font-normal text-warn">
							{remaining} remaining
						</span>
					)}
				</div>
			</div>
		</div>
	);
}

interface AuthScreenProps {
	auth: AdminStatus | null;
	onSessionUpdate: () => Promise<AdminStatus | null>;
}

function AuthScreen({ auth, onSessionUpdate }: AuthScreenProps) {
	const [password, setPassword] = useState("");
	const [confirm, setConfirm] = useState("");
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");
	const recovery = auth?.recovery;
	const isRecovery = auth?.recovery?.active;

	const handleSubmit = async (e: FormEvent) => {
		e.preventDefault();
		setError("");
		if (!isRecovery) {
			setBusy(true);
			try {
				await postJson("/api/v1/auth/login", { password });
				onSessionUpdate();
			} catch (err) {
				setError((err as Error).message);
			} finally {
				setBusy(false);
			}
			return;
		}
		if (password !== confirm) {
			setError("Passwords do not match");
			return;
		}
		if (password.length < 4) {
			setError("Password must be at least 4 characters");
			return;
		}
		setBusy(true);
		try {
			await postJson("/api/v1/recovery/reset", { password });
			onSessionUpdate();
		} catch (err) {
			setError((err as Error).message);
		} finally {
			setBusy(false);
		}
	};

	return (
		<div className="grid place-items-center min-h-screen p-6">
			<Card className="w-full max-w-md p-6 sm:p-8 rounded-2xl">
				<CardHeader className="px-0 pt-0">
					<div className="text-xs uppercase tracking-widest text-muted-foreground mb-2">
						Router appliance
					</div>
					<CardTitle className="text-3xl">
						<span className="text-[color:var(--accent-2,#e53935)]">
							Vantage
						</span>
						<span className="text-foreground">OS</span>
					</CardTitle>
				</CardHeader>

				<CardContent className="px-0">
					{isRecovery && (
						<div className="mb-4">
							<PinholeViz
								pressCount={recovery?.press_count ?? 0}
								requiredPresses={recovery?.required_presses ?? TAP_TOTAL}
								stage={recovery?.stage ?? "idle"}
								windowExpiresAt={recovery?.recovery_expires_at}
							/>
							<Separator className="my-4" />
						</div>
					)}

					{!isRecovery && (
						<CardDescription className="mb-4">
							Enter your admin password to access the control plane.
						</CardDescription>
					)}

					<form className="flex flex-col gap-3.5" onSubmit={handleSubmit}>
						<div className="grid gap-1.5">
							<Label htmlFor="password">
								{isRecovery
									? "New password"
									: "Password"}
							</Label>
							<Input
								id="password"
								type="password"
								value={password}
								onChange={(e) => setPassword(e.target.value)}
								placeholder={isRecovery ? "At least 4 characters" : ""}
								autoFocus
							/>
						</div>

						{isRecovery && (
							<div className="grid gap-1.5">
								<Label htmlFor="confirm">Confirm password</Label>
								<Input
									id="confirm"
									type="password"
									value={confirm}
									onChange={(e) => setConfirm(e.target.value)}
									placeholder="Repeat password"
								/>
							</div>
						)}

						{error && (
							<div className="rounded-lg bg-destructive/10 border border-destructive/20 p-3 text-sm text-destructive">
								{error}
							</div>
						)}

						<Button
							type="submit"
							disabled={busy || !password}
							className="mt-1 w-full"
						>
							{busy
								? "Please wait..."
								: isRecovery
									? "Reset password"
									: "Login"}
						</Button>
					</form>

					{!isRecovery && (
						<div className="mt-6 pt-5 border-t border-border">
							<p className="text-sm text-muted-foreground mb-3">
								Forgot your password?
							</p>
							<PinholeViz
								pressCount={recovery?.press_count ?? 0}
								requiredPresses={recovery?.required_presses ?? TAP_TOTAL}
								stage={recovery?.stage ?? "idle"}
								windowExpiresAt={recovery?.press_window_expires_at}
							/>
						</div>
					)}
				</CardContent>
			</Card>
		</div>
	);
}

interface SetupChecklistProps {
	setup: BootstrapResponse["setup"] | undefined;
	status: StatusResponse | null;
	networks: Network[];
	devices: Device[];
	onApply: () => void;
	onRollback: () => void;
	onEnableTailscale: () => void;
	onOpenSetup: () => void;
}

function SetupChecklist({
	setup,
	status,
	networks,
	devices,
	onApply,
	onRollback,
	onEnableTailscale,
	onOpenSetup,
}: SetupChecklistProps) {
	if (!setup) return null;

	const completed = setup.checklist.filter((item) => item.done).length;
	const total = setup.checklist.length;

	return (
		<Card className={`${setup.needs_setup ? "ring-primary/30" : ""}`}>
			<CardHeader>
				<div className="flex items-start justify-between gap-4">
					<div>
						<CardTitle>
							{setup.needs_setup ? "First-run setup" : "Setup status"}
						</CardTitle>
						<CardDescription>{setup.next_action}</CardDescription>
					</div>
					<Badge variant={setup.needs_setup ? "destructive" : "default"}>
						{completed}/{total} done
					</Badge>
				</div>
			</CardHeader>

			<CardContent>
				<div className="flex justify-between items-end gap-6 p-4 rounded-xl bg-primary/[0.06] border border-primary/[0.12] mb-4">
					<div>
						<div className="text-xs text-muted-foreground">Current mode</div>
						<div className="text-2xl font-bold mt-1">
							{setup.state_label ?? setup.state}
						</div>
						<div className="text-sm text-muted-foreground mt-1.5">
							{networks.length} networks, {devices.length} devices, remote admin{" "}
							{status?.tailscale?.state ?? "unknown"}, last transition{" "}
							{formatDate(setup.last_transition_at)}
						</div>
					</div>

					<div className="min-w-[280px] max-w-full grid gap-2">
						<div className="h-3 rounded-full bg-muted overflow-hidden">
							<div
								className="h-full rounded-full bg-gradient-to-r from-primary to-[color:var(--accent-2,#7cdbb6)] transition-all"
								style={{ width: `${(completed / total) * 100}%` }}
							/>
						</div>
						<div className="text-xs text-muted-foreground">
							{Math.round((completed / total) * 100)}% ready
						</div>
					</div>
				</div>

				{setup.needs_setup && (
					<div className="rounded-lg bg-warn/10 border border-warn/20 p-3 text-sm text-muted-foreground mb-4">
						Connect to the VantageOS Wi-Fi, then open{" "}
						<code className="text-foreground">http://vantageos.local/</code> or{" "}
						<code className="text-foreground">http://192.168.8.1/</code>.
					</div>
				)}

				<div className="flex flex-col gap-2.5">
					{setup.checklist.map((item) => (
						<div
							key={item.key}
							className={`flex justify-between items-center gap-3.5 p-3.5 rounded-xl border ${
								item.done
									? "border-ok/20 bg-background/60"
									: "border-warn/20 bg-background/60"
							}`}
						>
							<div>
								<div className="font-semibold">{item.label}</div>
								<div className="text-sm text-muted-foreground">
									{item.done ? "Complete" : "Pending"}
								</div>
							</div>
							<Badge variant={item.done ? "default" : "destructive"}>
								{item.done ? "done" : "pending"}
							</Badge>
						</div>
					))}
				</div>

				<div className="flex flex-wrap gap-2.5 mt-4">
					<Button variant="secondary" onClick={onOpenSetup}>
						Open guided setup
					</Button>
					<Button variant="secondary" onClick={onEnableTailscale}>
						Enable Tailscale
					</Button>
					<Button onClick={onApply}>Apply current config</Button>
					<Button
						variant="secondary"
						onClick={onRollback}
						disabled={!status?.active_revision}
					>
						Roll back
					</Button>
				</div>
			</CardContent>
		</Card>
	);
}

interface ConfigHistoryProps {
	revisions: ConfigRevision[];
}

function ConfigHistory({ revisions }: ConfigHistoryProps) {
	return (
		<Card>
			<CardHeader>
				<div>
					<CardTitle>Configuration history</CardTitle>
					<CardDescription>
						Applied revisions and the current active snapshot.
					</CardDescription>
				</div>
			</CardHeader>

			<CardContent>
				<div className="flex flex-col gap-3">
					{revisions.length ? (
						revisions.map((revision) => (
							<article
								key={revision.id}
								className={`p-3.5 rounded-xl border bg-background/60 ${revision.active ? "border-ok/30" : "border-border"}`}
							>
								<div className="flex justify-between items-start gap-3">
									<div>
										<div className="font-bold">
											Revision {revision.revision}: {revision.title}
										</div>
										<div className="text-sm text-muted-foreground mt-1">
											{revision.note}
										</div>
									</div>
									<Badge variant={revision.active ? "default" : "secondary"}>
										{revision.status}
									</Badge>
								</div>

								<div className="flex flex-wrap gap-2.5 mt-3 text-sm text-muted-foreground">
									<span>{formatDate(revision.created_at)}</span>
									<span>
										{revision.snapshot.networks?.length ?? 0} networks
									</span>
									<span>{revision.snapshot.devices?.length ?? 0} devices</span>
									<span>
										{revision.snapshot.credentials?.length ?? 0} credentials
									</span>
								</div>
							</article>
						))
					) : (
						<div className="text-sm text-muted-foreground py-2">
							No configuration revisions yet.
						</div>
					)}
				</div>
			</CardContent>
		</Card>
	);
}

interface DeviceProvisioningProps {
	networks: Network[];
	onProvisioned: (result: DeviceProvisioningResult) => void;
}

interface DeviceProvisioningResult {
	device: Device;
	credentials: Credential[];
}

function DeviceProvisioning({
	networks,
	onProvisioned,
}: DeviceProvisioningProps) {
	const [form, setForm] = useState(initialForm);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");

	const canSubmit = form.name.trim().length > 0 && !busy;

	async function handleSubmit(event: FormEvent) {
		event.preventDefault();
		setError("");
		setBusy(true);

		try {
			const result = await postJson<DeviceProvisioningResult>(
				"/api/v1/devices",
				{
					name: form.name.trim(),
					network_slug: form.network_slug,
					include_fallback: form.include_fallback,
				},
			);

			onProvisioned(result);
			setForm(initialForm);
		} catch (err) {
			setError((err as Error).message);
		} finally {
			setBusy(false);
		}
	}

	return (
		<Card id="guided-setup">
			<CardHeader>
				<div>
					<CardTitle>Add device</CardTitle>
					<CardDescription>
						Generate a secure onboarding secret and optional fallback
						credential.
					</CardDescription>
				</div>
			</CardHeader>

			<CardContent>
				<form className="flex flex-col gap-3" onSubmit={handleSubmit}>
					<div className="grid gap-1.5">
						<Label htmlFor="device-name">Device name</Label>
						<Input
							id="device-name"
							value={form.name}
							onChange={(event) =>
								setForm({ ...form, name: event.target.value })
							}
							placeholder="Kitchen camera"
						/>
					</div>

					<div className="grid gap-1.5">
						<Label htmlFor="network-select">Network</Label>
						<Select
							value={form.network_slug}
							onValueChange={(v) => v && setForm({ ...form, network_slug: v })}
						>
							<SelectTrigger id="network-select" className="w-full">
								<SelectValue placeholder="Select network" />
							</SelectTrigger>
							<SelectContent>
								{networks.map((network) => (
									<SelectItem key={network.slug} value={network.slug}>
										{network.name} - {network.ssid}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>

					<Label className="flex items-center gap-2.5 text-sm font-medium">
						<input
							type="checkbox"
							checked={form.include_fallback}
							onChange={(event) =>
								setForm({ ...form, include_fallback: event.target.checked })
							}
							className="w-4.5 h-4.5"
						/>
						Include temporary insecure fallback credential
					</Label>

					<Button type="submit" disabled={!canSubmit} className="w-fit mt-1">
						{busy ? "Generating..." : "Generate access"}
					</Button>
				</form>

				<div className="rounded-lg bg-warn/10 border border-warn/20 p-3 text-sm text-muted-foreground mt-3">
					Fallback credentials are for onboarding only. Revoke after the device
					proves a stable join.
				</div>

				{error && (
					<div className="rounded-lg bg-destructive/10 border border-destructive/20 p-3 text-sm text-destructive mt-3">
						{error}
					</div>
				)}
			</CardContent>
		</Card>
	);
}

interface CredentialViewProps {
	payload: DeviceProvisioningResult | null;
}

function CredentialView({ payload }: CredentialViewProps) {
	if (!payload) return null;

	return (
		<Card className="ring-primary/30">
			<CardHeader>
				<div>
					<CardTitle>Provisioning result</CardTitle>
					<CardDescription>
						Show this QR once, then store the secure secret in the backend.
					</CardDescription>
				</div>
				<Badge variant="default">Pending enrollment</Badge>
			</CardHeader>

			<CardContent>
				<div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
					{payload.credentials.map((credential) => (
						<article
							key={credential.id}
							className={`p-4 rounded-xl border bg-background/70 ${credential.kind === "secure" ? "border-ok/20" : "border-warn/20"}`}
						>
							<div className="flex justify-between items-center gap-2.5 mb-4">
								<h3 className="font-semibold text-sm">
									{credential.kind === "secure"
										? "Secure credential"
										: "Fallback credential"}
								</h3>
								<Badge
									variant={
										credential.kind === "secure" ? "default" : "destructive"
									}
								>
									{credential.kind}
								</Badge>
							</div>

							<div className="grid place-items-center my-2.5 mb-4 p-3.5 rounded-xl bg-white">
								<QRCodeSVG
									value={credential.qr_payload}
									size={196}
									bgColor="#ffffff"
									fgColor="#0d1220"
								/>
							</div>

							<div className="grid gap-2 mb-3">
								<div className="text-xs uppercase tracking-wider text-muted-foreground">
									Password
								</div>
								<code className="block overflow-x-auto whitespace-pre-wrap break-all rounded-lg p-3 bg-muted text-sm">
									{credential.secret}
								</code>
							</div>

							<div className="text-xs uppercase tracking-wider text-muted-foreground mb-1">
								QR payload
							</div>
							<pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-lg p-3 bg-muted text-sm text-foreground">
								{credential.qr_payload}
							</pre>
						</article>
					))}
				</div>
			</CardContent>
		</Card>
	);
}

interface SecurityEventListProps {
	events?: StatusResponse["latest_events"];
}

function SecurityEventList({ events }: SecurityEventListProps) {
	const securityEvents =
		events?.filter((e) => e.kind?.startsWith("security.")) ?? [];
	if (!securityEvents.length) return null;

	return (
		<Card>
			<CardHeader>
				<div>
					<CardTitle>Security events</CardTitle>
					<CardDescription>Recent security-related activity</CardDescription>
				</div>
			</CardHeader>

			<CardContent>
				<div className="flex flex-col gap-1.5">
					{securityEvents
						.slice(-8)
						.reverse()
						.map((ev) => (
							<div
								key={ev.id}
								className="flex items-center justify-between gap-2.5 py-1"
							>
								<Badge variant="destructive">
									{ev.kind.replace("security.", "")}
								</Badge>
								<span className="text-xs text-muted-foreground">
									{formatDate(ev.created_at)}
								</span>
							</div>
						))}
				</div>
			</CardContent>
		</Card>
	);
}

interface ConfirmRollbackBannerProps {
	pendingRollback: PendingRollback;
	onConfirm: () => void;
}

function ConfirmRollbackBanner({
	pendingRollback,
	onConfirm,
}: ConfirmRollbackBannerProps) {
	const [remaining, setRemaining] = useState(
		Math.ceil(pendingRollback.expires_in_ms / 1000),
	);

	useEffect(() => {
		if (remaining <= 0) return;
		const interval = setInterval(() => {
			setRemaining((r) => Math.max(r - 1, 0));
		}, 1000);
		return () => clearInterval(interval);
	}, [remaining]);

	return (
		<div className="rounded-lg bg-ok/10 border border-ok/20 p-4 text-sm mb-4">
			<div className="flex items-start justify-between gap-4">
				<div>
					<p className="font-semibold text-foreground mb-1">
						New configuration applied
					</p>
					<p className="text-muted-foreground">
						If everything is working, confirm to keep this configuration.
						{remaining > 0 && (
							<span className="ml-1">
								Auto-rollback in <strong>{remaining}s</strong>.
							</span>
						)}
					</p>
				</div>
				<Button
					onClick={onConfirm}
					disabled={remaining <= 0}
					className="shrink-0"
				>
					{remaining > 0 ? "Confirm" : "Expired"}
				</Button>
			</div>
		</div>
	);
}

interface SuricataCardProps {
	suricata?: SuricataResponse;
	onRefresh: () => void;
}

function SuricataCard({ suricata, onRefresh }: SuricataCardProps) {
	const [busy, setBusy] = useState(false);

	async function toggle() {
		setBusy(true);
		try {
			if (suricata?.enabled) {
				await postJson("/api/v1/suricata/disable");
			} else {
				await postJson("/api/v1/suricata/enable");
			}
			onRefresh();
		} catch (err) {
			console.warn("suricata toggle failed:", err);
		} finally {
			setBusy(false);
		}
	}

	return (
		<Card>
			<CardHeader>
				<div className="flex items-start justify-between gap-4">
					<div>
						<CardTitle>Suricata IDS</CardTitle>
						<CardDescription>
							Intrusion detection and prevention system
						</CardDescription>
					</div>
					<Badge variant={suricata?.enabled ? "default" : "secondary"}>
						{suricata?.state ?? "unknown"}
					</Badge>
				</div>
			</CardHeader>
			<CardContent>
				<div className="grid grid-cols-2 gap-4 mb-4">
					<div>
						<div className="text-xs uppercase tracking-wider text-muted-foreground">
							Packets
						</div>
						<div className="text-2xl font-bold mt-1">
							{suricata?.packets_total?.toLocaleString() ?? "—"}
						</div>
					</div>
					<div>
						<div className="text-xs uppercase tracking-wider text-muted-foreground">
							Alerts
						</div>
						<div className="text-2xl font-bold mt-1">
							{suricata?.alerts_total?.toLocaleString() ?? "—"}
						</div>
					</div>
				</div>
				<Button
					onClick={toggle}
					disabled={busy}
					variant={suricata?.enabled ? "destructive" : "default"}
					className="w-fit"
				>
					{busy
						? "Updating..."
						: suricata?.enabled
							? "Disable Suricata"
							: "Enable Suricata"}
				</Button>
			</CardContent>
		</Card>
	);
}

const alertsConfig = {
	alerts: { label: "Alerts", color: "#e53935" },
} satisfies ChartConfig;

function AlertsChart({ data }: { data: DataPoint[] }) {
	if (!data.length) return null;
	const formatted = data.map((d) => ({
		time: new Date(d.timestamp * 1000).toLocaleTimeString(),
		alerts: d.value,
	}));
	return (
		<Card>
			<CardHeader>
				<div>
					<CardTitle>Suricata alerts</CardTitle>
					<CardDescription>Alerts over the last hour</CardDescription>
				</div>
			</CardHeader>
			<CardContent>
				<ChartContainer config={alertsConfig} className="h-48 w-full">
					<AreaChart data={formatted}>
						<defs>
							<linearGradient id="alertGrad" x1="0" y1="0" x2="0" y2="1">
								<stop offset="5%" stopColor="#e53935" stopOpacity={0.3} />
								<stop offset="95%" stopColor="#e53935" stopOpacity={0} />
							</linearGradient>
						</defs>
						<CartesianGrid strokeDasharray="3 3" />
						<XAxis dataKey="time" tick={{ fontSize: 10 }} />
						<YAxis allowDecimals={false} tick={{ fontSize: 10 }} />
						<ChartTooltip content={<ChartTooltipContent indicator="line" />} />
						<Area
							type="monotone"
							dataKey="alerts"
							name="alerts"
							stroke="var(--color-alerts)"
							fill="url(#alertGrad)"
							strokeWidth={2}
						/>
					</AreaChart>
				</ChartContainer>
			</CardContent>
		</Card>
	);
}

const trafficConfig = {
	rx: { label: "RX", color: "#3b82f6" },
	tx: { label: "TX", color: "#22c55e" },
} satisfies ChartConfig;

function TrafficChart({ data }: { data: TrafficSnapshot }) {
	if (!data.rx_series.length && !data.tx_series.length) return null;
	const maxLen = Math.max(data.rx_series.length, data.tx_series.length);
	const merged = [];
	for (let i = 0; i < maxLen; i++) {
		const rx = data.rx_series[i];
		const tx = data.tx_series[i];
		merged.push({
			time: rx
				? new Date(rx.timestamp * 1000).toLocaleTimeString()
				: tx
					? new Date(tx.timestamp * 1000).toLocaleTimeString()
					: "",
			rx: rx ? Math.round(rx.value) : 0,
			tx: tx ? Math.round(tx.value) : 0,
		});
	}
	return (
		<Card>
			<CardHeader>
				<div>
					<CardTitle>Network traffic</CardTitle>
					<CardDescription>RX / TX bytes per second</CardDescription>
				</div>
			</CardHeader>
			<CardContent>
				<ChartContainer config={trafficConfig} className="h-48 w-full">
					<AreaChart data={merged}>
						<defs>
							<linearGradient id="rxGrad" x1="0" y1="0" x2="0" y2="1">
								<stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3} />
								<stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
							</linearGradient>
							<linearGradient id="txGrad" x1="0" y1="0" x2="0" y2="1">
								<stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
								<stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
							</linearGradient>
						</defs>
						<CartesianGrid strokeDasharray="3 3" />
						<XAxis dataKey="time" tick={{ fontSize: 10 }} />
						<YAxis tick={{ fontSize: 10 }} />
						<ChartTooltip content={<ChartTooltipContent indicator="line" />} />
						<Area
							type="monotone"
							dataKey="rx"
							name="rx"
							stroke="var(--color-rx)"
							fill="url(#rxGrad)"
							strokeWidth={2}
						/>
						<Area
							type="monotone"
							dataKey="tx"
							name="tx"
							stroke="var(--color-tx)"
							fill="url(#txGrad)"
							strokeWidth={2}
						/>
						<ChartLegend content={<ChartLegendContent />} />
					</AreaChart>
				</ChartContainer>
			</CardContent>
		</Card>
	);
}

interface DashboardProps {
	auth: AdminStatus | null;
	onLogout: () => void;
}

function Dashboard({ auth, onLogout }: DashboardProps) {
	const [bootstrap, setBootstrap] = useState<BootstrapResponse | null>(null);
	const [status, setStatus] = useState<StatusResponse | null>(null);
	const [networks, setNetworks] = useState<Network[]>([]);
	const [devices, setDevices] = useState<Device[]>([]);
	const [history, setHistory] = useState<ConfigRevision[]>([]);
	const [payload, setPayload] = useState<DeviceProvisioningResult | null>(null);
	const [suricataHistory, setSuricataHistory] =
		useState<SuricataSnapshot | null>(null);
	const [trafficHistory, setTrafficHistory] = useState<TrafficSnapshot | null>(
		null,
	);
	const [error, setError] = useState("");
	const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
	const [offline, setOffline] = useState(false);
	const [loading, setLoading] = useState(true);

	const summary = useMemo(() => {
		const services = status?.services ?? [];
		return {
			totalServices: services.length,
			healthyServices: services.filter((s) => s.state === "healthy").length,
		};
	}, [status]);

	const refresh = useCallback(async () => {
		try {
			const [
				bootstrapData,
				statusData,
				networksData,
				devicesData,
				historyData,
			] = await Promise.all([
				getJson<BootstrapResponse>("/api/v1/bootstrap"),
				getJson<StatusResponse>("/api/v1/status"),
				getJson<{ items: Network[] }>("/api/v1/networks"),
				getJson<{ items: Device[] }>("/api/v1/devices"),
				getJson<{ items: ConfigRevision[] }>("/api/v1/config/history"),
			]);

			setBootstrap(bootstrapData);
			setStatus(statusData);
			setNetworks(networksData.items);
			setDevices(devicesData.items);
			setHistory(historyData.items);

			try {
				const [suricataSnap, trafficSnap] = await Promise.all([
					getJson<SuricataSnapshot>("/api/v1/suricata/history"),
					getJson<TrafficSnapshot>("/api/v1/traffic/history"),
				]);
				setSuricataHistory(suricataSnap);
				setTrafficHistory(trafficSnap);
			} catch {
				// charts data is optional
			}
			setError("");
			setLastUpdated(new Date());
			setOffline(false);
			setLoading(false);
		} catch (err) {
			setError((err as Error).message);
			setOffline(true);
			setLoading(false);
		}
	}, []);

	async function applyConfig() {
		try {
			await postJson("/api/v1/config/apply", {
				title: "Applied from control plane",
				note: "Captured during guided setup or operator review",
			});
			await refresh();
		} catch (err) {
			setError((err as Error).message);
		}
	}

	async function confirmConfig() {
		try {
			await postJson("/api/v1/config/confirm");
			await refresh();
		} catch (err) {
			setError((err as Error).message);
		}
	}

	async function rollbackConfig() {
		try {
			const target = history[1]?.revision;
			await postJson(
				"/api/v1/config/rollback",
				target ? { revision: target } : {},
			);
			await refresh();
		} catch (err) {
			setError((err as Error).message);
		}
	}

	async function enableTailscale() {
		try {
			await postJson("/api/v1/tailscale/enroll", {});
			await refresh();
		} catch (err) {
			setError((err as Error).message);
		}
	}

	function openGuidedSetup() {
		document
			.getElementById("guided-setup")
			?.scrollIntoView({ behavior: "smooth", block: "start" });
	}

	useEffect(() => {
		refresh();
		const timer = window.setInterval(refresh, 10000);
		return () => window.clearInterval(timer);
	}, [refresh]);

	useEffect(() => {
		if (!bootstrap?.setup?.needs_setup) return;
		document
			.getElementById("guided-setup")
			?.scrollIntoView({ behavior: "smooth", block: "start" });
	}, [bootstrap?.setup?.needs_setup]);

	if (loading) {
		return (
			<div className="grid place-items-center min-h-screen text-muted-foreground text-lg">
				Loading dashboard...
			</div>
		);
	}

	return (
		<div className="w-full max-w-[1400px] mx-auto px-4 py-6 pb-12">
			<header className="flex justify-between gap-6 items-start mb-5">
				<div>
					<div className="text-xs uppercase tracking-widest text-muted-foreground mb-2">
						Router appliance
					</div>
					<h1 className="text-[clamp(32px,4vw,52px)] leading-none m-0">
						<span className="text-[color:var(--accent-2,#e53935)]">
							Vantage
						</span>
						<span className="text-foreground">OS</span>
					</h1>
					<p className="text-muted-foreground">
						Static SPA frontend, local Go API, hostapd-backed wireless policy.
					</p>
				</div>

				<div className="flex flex-wrap items-center gap-2">
					{offline && <Badge variant="destructive">Disconnected</Badge>}
					<Badge
						variant={bootstrap?.setup?.needs_setup ? "destructive" : "default"}
					>
						{bootstrap?.setup?.state_label ?? "first run"}
					</Badge>
					<Badge variant="default">
						{bootstrap?.launch_band ?? "5 GHz only"}
					</Badge>
					<Badge variant="secondary">{bootstrap?.ui_mode ?? "SPA"}</Badge>
					{auth?.recovery?.active && (
						<Badge variant="destructive">Recovery mode</Badge>
					)}
					<Button
						variant="ghost"
						className="text-destructive text-xs font-bold uppercase tracking-wider h-auto px-3 py-1.5 rounded-full border border-destructive/20 bg-destructive/10 hover:bg-destructive/20"
						onClick={onLogout}
					>
						Logout
					</Button>
					<Badge variant="secondary">
						{lastUpdated
							? `Updated ${lastUpdated.toLocaleTimeString()}`
							: "Loading"}
					</Badge>
				</div>
			</header>

			{offline && (
				<div className="rounded-lg bg-warn/10 border border-warn/20 p-4 text-sm mb-4">
					<p className="font-semibold text-foreground mb-1">
						Router unreachable
					</p>
					<p className="text-muted-foreground">
						Unable to reach the router. Your Wi-Fi connection may have changed.
						If you recently applied a new configuration, reconnect to your
						network and the page will update automatically.
					</p>
				</div>
			)}

			{error && !offline && (
				<div className="rounded-lg bg-destructive/10 border border-destructive/20 p-3 text-sm text-destructive mb-4">
					{error}
				</div>
			)}

			{status?.pending_rollback && (
				<ConfirmRollbackBanner
					pendingRollback={status.pending_rollback}
					onConfirm={confirmConfig}
				/>
			)}

			<SetupChecklist
				setup={bootstrap?.setup}
				status={status}
				networks={networks}
				devices={devices}
				onApply={applyConfig}
				onRollback={rollbackConfig}
				onEnableTailscale={enableTailscale}
				onOpenSetup={openGuidedSetup}
			/>

			{status?.latest_events ? (
				<SecurityEventList events={status.latest_events} />
			) : null}

			<section className="grid grid-cols-2 sm:grid-cols-4 gap-4 my-4">
				<Card>
					<CardContent className="py-4">
						<div className="text-xs uppercase tracking-wider text-muted-foreground">
							Services healthy
						</div>
						<div className="text-3xl font-bold mt-2.5">
							{summary.healthyServices}/{summary.totalServices || 0}
						</div>
						<div className="text-sm text-muted-foreground mt-2">
							Local router stack
						</div>
					</CardContent>
				</Card>
				<Card>
					<CardContent className="py-4">
						<div className="text-xs uppercase tracking-wider text-muted-foreground">
							Networks
						</div>
						<div className="text-3xl font-bold mt-2.5">{networks.length}</div>
						<div className="text-sm text-muted-foreground mt-2">
							Single operational SSID
						</div>
					</CardContent>
				</Card>
				<Card>
					<CardContent className="py-4">
						<div className="text-xs uppercase tracking-wider text-muted-foreground">
							Devices
						</div>
						<div className="text-3xl font-bold mt-2.5">{devices.length}</div>
						<div className="text-sm text-muted-foreground mt-2">
							Per-device PSKs
						</div>
					</CardContent>
				</Card>
				<Card>
					<CardContent className="py-4">
						<div className="text-xs uppercase tracking-wider text-muted-foreground">
							Remote admin
						</div>
						<div className="text-3xl font-bold mt-2.5">
							{status?.tailscale?.state ?? "unknown"}
						</div>
						<div className="text-sm text-muted-foreground mt-2">
							Tailscale control plane
						</div>
					</CardContent>
				</Card>
			</section>

			<div className="grid grid-cols-1 lg:grid-cols-[1.05fr_1.2fr] gap-4 items-start">
				<div className="grid gap-4">
					<DeviceProvisioning networks={networks} onProvisioned={setPayload} />
					<CredentialView payload={payload} />
				</div>

				<div className="grid gap-4">
					<Card>
						<CardHeader>
							<div>
								<CardTitle>Networks</CardTitle>
								<CardDescription>
									Current launch model: one 5 GHz WPA2-PSK operational network.
								</CardDescription>
							</div>
						</CardHeader>

						<CardContent>
							<Table>
								<TableHeader>
									<TableRow>
										<TableHead>Name</TableHead>
										<TableHead>SSID</TableHead>
										<TableHead>Zone</TableHead>
										<TableHead>Band</TableHead>
										<TableHead>Auth</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{networks.map((network) => (
										<TableRow key={network.slug}>
											<TableCell className="font-medium">
												{network.name}
											</TableCell>
											<TableCell>{network.ssid}</TableCell>
											<TableCell>{network.zone}</TableCell>
											<TableCell>{network.band}</TableCell>
											<TableCell>{network.auth_mode}</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
						</CardContent>
					</Card>

					<SuricataCard suricata={status?.suricata} onRefresh={refresh} />
					<AlertsChart data={suricataHistory?.series ?? []} />
					<TrafficChart
						data={trafficHistory ?? { rx_series: [], tx_series: [] }}
					/>

					<Card>
						<CardHeader>
							<div>
								<CardTitle>Devices</CardTitle>
								<CardDescription>
									Track enrollment mode, state, and the assigned network.
								</CardDescription>
							</div>
						</CardHeader>

						<CardContent>
							<Table>
								<TableHeader>
									<TableRow>
										<TableHead>Name</TableHead>
										<TableHead>Network</TableHead>
										<TableHead>State</TableHead>
										<TableHead>Enrolled via</TableHead>
										<TableHead>Last seen</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{devices.map((device) => (
										<TableRow key={device.id}>
											<TableCell className="font-medium">
												{device.name}
											</TableCell>
											<TableCell>{device.network_name}</TableCell>
											<TableCell>{device.join_state}</TableCell>
											<TableCell>{device.enrolled_via ?? "pending"}</TableCell>
											<TableCell>{formatDate(device.last_seen_at)}</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
						</CardContent>
					</Card>

					<Card>
						<CardHeader>
							<div>
								<CardTitle>Platform snapshot</CardTitle>
								<CardDescription>
									Build-time decisions captured in code and docs.
								</CardDescription>
							</div>
						</CardHeader>

						<CardContent>
							<ul className="m-0 pl-4 text-foreground grid gap-2.5 text-sm">
								<li>Frontend: Vite + React + pnpm</li>
								<li>Web server: nginx</li>
								<li>Backend: Go</li>
								<li>Router plane: systemd-networkd + hostapd + nftables</li>
								<li>Remote admin: Tailscale</li>
								<li>Launch: 5 GHz only</li>
							</ul>
						</CardContent>
					</Card>

					<ConfigHistory revisions={history} />

					<Card>
						<CardHeader>
							<div>
								<CardTitle>Config actions</CardTitle>
								<CardDescription>
									Capture a revision or revert to the previous one.
								</CardDescription>
							</div>
						</CardHeader>

						<CardContent>
							<div className="flex flex-wrap gap-2.5">
								<Button onClick={applyConfig}>Apply config</Button>
								<Button
									variant="secondary"
									onClick={rollbackConfig}
									disabled={history.length < 2}
								>
									Roll back
								</Button>
							</div>
						</CardContent>
					</Card>
				</div>
			</div>
		</div>
	);
}

interface SavedSetupData {
	ssid: string;
	psk: string;
}

function SetupCompleteScreen({
	saved,
	onRetry,
}: {
	saved: SavedSetupData;
	onRetry: () => void;
}) {
	const [copied, setCopied] = useState(false);
	const ssid = saved?.ssid || "";
	const psk = saved?.psk || "";

	useEffect(() => {
		const handler = (e: BeforeUnloadEvent) => {
			e.preventDefault();
			e.returnValue = "";
		};
		const onKeyDown = (e: KeyboardEvent) => {
			if (
				e.key === "F5" ||
				((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "r")
			) {
				e.preventDefault();
			}
		};
		window.addEventListener("beforeunload", handler);
		window.addEventListener("keydown", onKeyDown);
		return () => {
			window.removeEventListener("beforeunload", handler);
			window.removeEventListener("keydown", onKeyDown);
		};
	}, []);

	return (
		<div className="grid place-items-center min-h-screen p-6">
			<Card className="w-full max-w-md p-6 sm:p-8 rounded-2xl">
				<CardHeader className="px-0 pt-0">
					<CardTitle className="text-3xl">
						<span className="text-[color:var(--accent-2,#e53935)]">
							Vantage
						</span>
						<span className="text-foreground">OS</span>
					</CardTitle>
					<CardDescription>Setup complete</CardDescription>
				</CardHeader>
				<CardContent className="px-0">
					<div className="rounded-lg bg-ok/10 border border-ok/20 p-4 text-sm mb-4">
						<p className="font-semibold text-foreground mb-1">
							Your router is ready
						</p>
						<p className="text-muted-foreground">
							Connect to <strong>{ssid || "your network"}</strong> using the
							password below, then open{" "}
							<code className="text-foreground">http://vantageos.local/</code>.
						</p>
					</div>
					{ssid && (
						<div className="text-sm mb-3">
							<span className="font-medium">Network:</span> {ssid}
						</div>
					)}
					{psk && (
						<>
							<div className="grid gap-1.5 mb-4">
								<Label>Wi-Fi password</Label>
								<div className="flex gap-2">
									<Input readOnly value={psk} className="font-mono text-sm" />
									<Button
										variant="secondary"
										onClick={() => {
											navigator.clipboard?.writeText(psk);
											setCopied(true);
											setTimeout(() => setCopied(false), 2000);
										}}
									>
										{copied ? "Copied!" : "Copy"}
									</Button>
								</div>
							</div>
							<div className="flex justify-center mb-4">
								<QRCodeSVG
									value={`WIFI:S:${ssid};T:WPA;P:${psk};;`}
									size={160}
									level="M"
								/>
							</div>
						</>
					)}
					<p className="text-xs text-muted-foreground">
						Do not reload this page. Connect to{" "}
						<strong>{ssid || "your network"}</strong>, then the dashboard will
						load automatically.
					</p>
					<Button className="mt-4" variant="secondary" onClick={onRetry}>
						Check connection now
					</Button>
				</CardContent>
			</Card>
		</div>
	);
}

export default function App() {
	const [auth, setAuth] = useState<AdminStatus | null>(null);
	const [authLoading, setAuthLoading] = useState(true);
	const [wizardOpen, setWizardOpen] = useState(false);
	const [savedSetup, setSavedSetup] = useState<SavedSetupData | null>(null);
	const hadValidSession = useRef(false);
	const lastKnownPasswordSet = useRef(false);
	const hasSuccessfulSessionFetch = useRef(false);

	const fetchSession = useCallback(async () => {
		try {
			const session = await getJson<AdminStatus>("/api/v1/auth/session");
			hasSuccessfulSessionFetch.current = true;
			lastKnownPasswordSet.current = session.password_set;
			if (session.password_set && session.authenticated) {
				hadValidSession.current = true;
			}
			setAuth(session);
			setSavedSetup(null);
			return session;
		} catch (err) {
			console.warn("Backend unreachable after setup:", err);
			if (!hadValidSession.current) {
				const done = localStorage.getItem("vantageos_setup_done");
				if (done) {
					setSavedSetup({
						ssid: localStorage.getItem("vantageos_ssid") || "",
						psk:
							localStorage.getItem("vantageos_psk") ||
							localStorage.getItem("vantageos_psk_secure") ||
							localStorage.getItem("vantageos_psk_fallback") ||
							"",
					});
				}
			}

			// Never regress to "password not set" from a failed fetch.
			// If we don't have a confirmed session fetch yet, keep waiting/retrying.
			if (hasSuccessfulSessionFetch.current) {
				setAuth(
					(prev) =>
						prev ?? {
							password_set: lastKnownPasswordSet.current,
							authenticated: false,
							recovery: {
								stage: "idle",
								active: false,
								press_count: 0,
								required_presses: TAP_TOTAL,
							},
						},
				);
			}
			return null;
		} finally {
			setAuthLoading(false);
		}
	}, []);

	useEffect(() => {
		function onUnauthorized() {
			void fetchSession();
		}
		window.addEventListener("vantageos:unauthorized", onUnauthorized);
		return () =>
			window.removeEventListener("vantageos:unauthorized", onUnauthorized);
	}, [fetchSession]);

	useEffect(() => {
		fetchSession();
		const timer = window.setInterval(fetchSession, 5000);
		return () => window.clearInterval(timer);
	}, [fetchSession]);

	useEffect(() => {
		if (!auth?.password_set || !auth?.authenticated) return;
		getJson<BootstrapResponse>("/api/v1/bootstrap")
			.then((data) => {
				if (data?.setup?.needs_setup) setWizardOpen(true);
			})
			.catch((err: unknown) => console.warn("bootstrap poll failed:", err));
	}, [auth?.password_set, auth?.authenticated]);

	useEffect(() => {
		if (authLoading) return;
		if (!auth) return;
		if (!auth.password_set) {
			setWizardOpen(true);
		}
	}, [authLoading, auth]);

	async function handleLogout() {
		await postJson("/api/v1/auth/logout", {});
		void fetchSession();
	}

	if (savedSetup) {
		return (
			<SetupCompleteScreen
				saved={savedSetup}
				onRetry={() => void fetchSession()}
			/>
		);
	}

	if (authLoading) {
		return (
			<div className="grid place-items-center min-h-screen text-muted-foreground text-lg">
				Loading...
			</div>
		);
	}

	if (!hasSuccessfulSessionFetch.current && !savedSetup) {
		return (
			<div className="grid place-items-center min-h-screen p-6">
				<Card className="w-full max-w-md p-6 sm:p-8 rounded-2xl">
					<CardHeader className="px-0 pt-0">
						<CardTitle className="text-3xl">
							<span className="text-[color:var(--accent-2,#e53935)]">
								Vantage
							</span>
							<span className="text-foreground">OS</span>
						</CardTitle>
						<CardDescription>Reconnecting to router...</CardDescription>
					</CardHeader>
					<CardContent className="px-0 text-sm text-muted-foreground">
						Waiting for the router control plane to come online. This can take a
						few seconds right after Wi-Fi changes.
					</CardContent>
				</Card>
			</div>
		);
	}

	const authenticated = auth?.password_set && auth?.authenticated;

	if (wizardOpen) {
		return (
			<SetupWizard
				auth={auth}
				onSessionUpdate={fetchSession}
				onComplete={() => setWizardOpen(false)}
			/>
		);
	}

	return authenticated ? (
		<Dashboard auth={auth} onLogout={handleLogout} />
	) : (
		<AuthScreen auth={auth} onSessionUpdate={fetchSession} />
	);
}
