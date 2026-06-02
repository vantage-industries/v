import { QRCodeSVG } from "qrcode.react";
import { type FormEvent, useCallback, useEffect, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { getJson, postJson } from "./api";
import type { AdminStatus, Credential, Network } from "./types";

const STEPS = ["welcome", "password", "network", "onboard", "apply"] as const;
type Step = (typeof STEPS)[number];

const STEP_TITLES: Record<Step, string> = {
	welcome: "Welcome",
	password: "Admin password",
	network: "Network name",
	onboard: "First device",
	apply: "Apply configuration",
};

interface SetupWizardProps {
	auth: AdminStatus | null;
	onSessionUpdate: () => Promise<AdminStatus | null>;
	onComplete: () => void;
}

interface CreateDeviceResponse {
	device: { id: string; join_state: string };
	credentials: Credential[];
}

export default function SetupWizard({
	auth,
	onSessionUpdate,
	onComplete,
}: SetupWizardProps) {
	const [step, setStep] = useState<Step>("welcome");
	const [networks, setNetworks] = useState<Network[]>([]);
	const [busy, setBusy] = useState(false);
	const [error, setError] = useState("");

	const [password, setPassword] = useState("");
	const [confirm, setConfirm] = useState("");
	const [ssid, setSsid] = useState("");
	const [deviceName, setDeviceName] = useState("");
	const [payload, setPayload] = useState<CreateDeviceResponse | null>(null);
	const [applied, setApplied] = useState(false);
	const [copiedIdx, setCopiedIdx] = useState(-1);
	const [appliedSSID, setAppliedSSID] = useState("");

	// Block accidental reloads during/after apply
	useEffect(() => {
		if (!(step === "apply" && (busy || applied))) return;
		const handler = (e: BeforeUnloadEvent) => {
			e.preventDefault();
			e.returnValue = "";
		};
		window.addEventListener("beforeunload", handler);
		return () => window.removeEventListener("beforeunload", handler);
	}, [step, busy, applied]);

	function saveCreds(
		creds: Credential[] | undefined,
		ssidName: string | undefined,
	) {
		try {
			const networkSSID = (ssidName || ssid || "").trim();
			if (networkSSID) {
				localStorage.setItem("vantageos_ssid", networkSSID);
			}

			const secure = creds?.find((c) => c.kind === "secure" && c.secret);
			const fallback = creds?.find((c) => c.kind === "fallback" && c.secret);
			const psk = (secure?.secret || fallback?.secret || "").trim();
			if (psk) {
				localStorage.setItem("vantageos_psk", psk);
			}
			if (secure?.secret) {
				localStorage.setItem("vantageos_psk_secure", secure.secret);
			}
			if (fallback?.secret) {
				localStorage.setItem("vantageos_psk_fallback", fallback.secret);
			}
			localStorage.setItem("vantageos_setup_done", "1");
		} catch (err) {
			console.warn(
				"localStorage save failed (PSK will not survive refresh):",
				err,
			);
		}
	}

	const fetchBootstrap = useCallback(async () => {
		try {
			const nets = await getJson<{ items: Network[] }>("/api/v1/networks");
			setNetworks(nets.items || []);
			const main = (nets.items || []).find((n) => n.slug === "main");
			if (main?.ssid) {
				setAppliedSSID(main.ssid);
			}
		} catch (err) {
			console.warn("fetchBootstrap failed:", err);
		}
	}, []);

	useEffect(() => {
		fetchBootstrap();
	}, [fetchBootstrap]);

	const activeStepIdx = STEPS.indexOf(step);
	const totalSteps = STEPS.length;

	function goNext() {
		const nextIdx = activeStepIdx + 1;
		if (nextIdx < totalSteps) setStep(STEPS[nextIdx]);
	}

	function goBack() {
		const prevIdx = activeStepIdx - 1;
		if (prevIdx >= 0) setStep(STEPS[prevIdx]);
	}

	async function handleSetPassword(e: FormEvent) {
		e.preventDefault();
		setError("");
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
			await postJson("/api/v1/auth/setup", { password });
			await onSessionUpdate();
			goNext();
		} catch (err) {
			setError((err as Error).message);
		} finally {
			setBusy(false);
		}
	}

	async function handleSaveNetwork(e: FormEvent) {
		e.preventDefault();
		setError("");
		const trimmed = ssid.trim();
		if (!trimmed) {
			setError("Network name (SSID) is required");
			return;
		}
		if (trimmed.length < 2) {
			setError("Network name must be at least 2 characters");
			return;
		}
		setBusy(true);
		try {
			await postJson("/api/v1/networks", {
				slug: "main",
				name: ssid || "Main Network",
				ssid: ssid || "VantageOS",
				zone: "trusted",
				band: "5ghz",
				auth_mode: "wpa2-psk",
			});
			await fetchBootstrap();
			goNext();
		} catch (err) {
			setError((err as Error).message);
		} finally {
			setBusy(false);
		}
	}

	async function handleOnboard(e: FormEvent) {
		e.preventDefault();
		setError("");
		if (!deviceName.trim()) {
			setError("Device name is required");
			return;
		}
		setBusy(true);
		try {
			const result = await postJson<CreateDeviceResponse>("/api/v1/devices", {
				name: deviceName.trim(),
				network_slug: "main",
				include_fallback: true,
			});
			setPayload(result);
			saveCreds(result.credentials, ssid.trim() || networks[0]?.ssid);
			await fetchBootstrap();
			goNext();
		} catch (err) {
			setError((err as Error).message);
		} finally {
			setBusy(false);
		}
	}

	async function handleApply(e: FormEvent) {
		e.preventDefault();
		setError("");
		setBusy(true);
		try {
			const result = await postJson<{
				status: string;
				main_ssid?: string;
			}>("/api/v1/config/apply", {
				title: "Initial setup",
				note: "Applied during guided setup wizard",
			});
			setAppliedSSID(result.main_ssid || "");
			saveCreds(payload?.credentials, result.main_ssid);
			setApplied(true);
		} catch (err) {
			setError((err as Error).message);
		} finally {
			setBusy(false);
		}
	}

	const currentStepIdx = STEPS.indexOf(step) + 1;
	const mainNetworkSSID =
		appliedSSID ||
		ssid.trim() ||
		networks.find((n) => n.slug === "main")?.ssid ||
		"";

	return (
		<div className="grid place-items-center min-h-screen p-6">
			<Card className="w-full max-w-lg p-6 sm:p-8 rounded-2xl">
				<CardHeader className="px-0 pt-0">
					<div className="text-xs uppercase tracking-widest text-[color:var(--accent-2,#e53935)] mb-2">
						Step {currentStepIdx} of {totalSteps} &middot; {STEP_TITLES[step]}
					</div>
					<CardTitle className="text-3xl">
						<span className="text-[color:var(--accent-2,#e53935)]">
							Vantage
						</span>
						<span className="text-foreground">OS</span>
					</CardTitle>
					<CardDescription>Router setup wizard</CardDescription>
				</CardHeader>

				<CardContent className="px-0">
					<div className="flex items-center gap-1 mb-5">
						{STEPS.map((s, i) => (
							<div key={s} className="flex items-center gap-1 flex-1">
								<div
									className={`h-2 flex-1 rounded-full transition-colors ${
										i <= STEPS.indexOf(step)
											? "bg-gradient-to-r from-[color:var(--accent-2,#e53935)] to-[color:var(--accent-3,#ff7043)]"
											: "bg-muted"
									}`}
								/>
							</div>
						))}
					</div>

					{error && (
						<div className="rounded-lg bg-destructive/10 border border-destructive/20 p-3 text-sm text-destructive mb-4">
							{error}
						</div>
					)}

					{step === "welcome" && (
						<div className="flex flex-col gap-4">
							<p className="text-sm text-muted-foreground">
								Welcome to{" "}
								<span className="text-[color:var(--accent-2,#e53935)] font-bold">
									Vantage
								</span>
								OS. This guided setup will walk you through configuring your
								router.
							</p>
							<ul className="text-sm text-muted-foreground list-disc pl-4 grid gap-1.5">
								<li>Set an admin password</li>
								<li>Name your Wi-Fi network</li>
								<li>Generate a device credential</li>
								<li>
									Apply the configuration &mdash; your secured network will
									appear
								</li>
							</ul>
							<p className="text-sm text-muted-foreground">
								You are currently connected to the open setup network. After
								applying, the router will switch to your secured network and you
								will need to reconnect.
							</p>
							<Button onClick={goNext} className="w-fit mt-2">
								Let's go
							</Button>
						</div>
					)}

					{step === "password" && (
						<div>
							{auth?.password_set ? (
								<div className="flex flex-col gap-4">
									<p className="text-sm text-muted-foreground">
										Admin password is already set.
									</p>
									<Badge variant="default" className="w-fit">
										Complete
									</Badge>
									<Button onClick={goNext} className="w-fit mt-2">
										Continue
									</Button>
								</div>
							) : (
								<form
									className="flex flex-col gap-3.5"
									onSubmit={handleSetPassword}
								>
									<p className="text-sm text-muted-foreground">
										Create an admin password to secure your router's control
										plane.
									</p>
									<div className="grid gap-1.5">
										<Label htmlFor="wiz-password">Password</Label>
										<Input
											id="wiz-password"
											type="password"
											value={password}
											onChange={(e) => setPassword(e.target.value)}
											placeholder="At least 4 characters"
											autoFocus
										/>
									</div>
									<div className="grid gap-1.5">
										<Label htmlFor="wiz-confirm">Confirm password</Label>
										<Input
											id="wiz-confirm"
											type="password"
											value={confirm}
											onChange={(e) => setConfirm(e.target.value)}
											placeholder="Repeat password"
										/>
									</div>
									<Button
										type="submit"
										disabled={busy || !password || !confirm}
										className="w-fit mt-1"
									>
										{busy ? "Setting..." : "Set password"}
									</Button>
								</form>
							)}
						</div>
					)}

					{step === "network" && (
						<form
							className="flex flex-col gap-3.5"
							onSubmit={handleSaveNetwork}
						>
							<p className="text-sm text-muted-foreground">
								Choose a name for your Wi-Fi network. This will appear as a
								secured WPA2-PSK network after you apply the configuration.
							</p>
							<div className="grid gap-1.5">
								<Label htmlFor="wiz-ssid">Network name (SSID)</Label>
								<Input
									id="wiz-ssid"
									value={ssid}
									onChange={(e) => setSsid(e.target.value)}
									placeholder="My Network"
									autoFocus
								/>
							</div>
							<div className="grid gap-1.5">
								<div className="rounded-lg border bg-muted/50 p-3 text-sm text-muted-foreground">
									Single-network mode is enabled. Your SSID will be applied to
									the main network.
								</div>
							</div>
							<Button type="submit" disabled={busy} className="w-fit mt-1">
								{busy ? "Saving..." : "Save and continue"}
							</Button>
						</form>
					)}

					{step === "onboard" && (
						<div>
							{payload ? (
								<div className="flex flex-col gap-4">
									<p className="text-sm text-muted-foreground">
										<strong>Save this Wi-Fi password before continuing.</strong>{" "}
										After you apply the configuration, your network will restart
										and you'll need it to reconnect.
									</p>
									{payload.credentials?.map((cred, i) => (
										<div
											key={cred.id}
											className="rounded-lg bg-muted p-3 text-sm grid gap-2"
										>
											<div className="flex items-center justify-between">
												<Badge
													variant={
														cred.kind === "secure" ? "default" : "secondary"
													}
													className="w-fit"
												>
													{cred.kind === "secure" ? "Wi-Fi password" : "Backup"}
												</Badge>
												<Button
													variant="ghost"
													size="sm"
													onClick={() => {
														navigator.clipboard?.writeText(cred.secret);
														setCopiedIdx(i);
														setTimeout(() => setCopiedIdx(-1), 2000);
													}}
												>
													{copiedIdx === i ? "Copied!" : "Copy"}
												</Button>
											</div>
											<div>
												<code className="text-sm break-all font-mono select-all">
													{cred.secret}
												</code>
											</div>
											{cred.kind === "secure" && (
												<div className="flex justify-center pt-1">
													<QRCodeSVG
														value={`WIFI:S:${mainNetworkSSID};T:WPA;P:${cred.secret};;`}
														size={128}
														level="M"
													/>
												</div>
											)}
										</div>
									))}
									<Button onClick={goNext} className="w-fit">
										Continue to apply
									</Button>
								</div>
							) : (
								<form
									className="flex flex-col gap-3.5"
									onSubmit={handleOnboard}
								>
									<p className="text-sm text-muted-foreground">
										Generate a Wi-Fi password for your first device. Save it
										&mdash; you will need it to reconnect after the
										configuration is applied.
									</p>
									<div className="grid gap-1.5">
										<Label htmlFor="wiz-device-name">Device name</Label>
										<Input
											id="wiz-device-name"
											value={deviceName}
											onChange={(e) => setDeviceName(e.target.value)}
											placeholder="Kitchen camera"
											autoFocus
										/>
									</div>
									<div className="rounded-lg border bg-muted/50 p-3 text-sm text-muted-foreground">
										Credentials will be generated for your main Wi-Fi network.
									</div>
									<Button
										type="submit"
										disabled={busy || !deviceName.trim()}
										className="w-fit mt-1"
									>
										{busy ? "Generating..." : "Generate credentials"}
									</Button>
								</form>
							)}
						</div>
					)}

					{step === "apply" && (
						<div className="flex flex-col gap-4">
							{applied ? (
								<div className="flex flex-col gap-4">
									<div className="rounded-lg bg-ok/10 border border-ok/20 p-4 text-sm">
										<p className="font-semibold text-foreground mb-1">
											Configuration applied
										</p>
										<p className="text-muted-foreground">
											Your network is now active. Your Wi-Fi connection may drop
											momentarily as the router switches from the open setup
											network to your secured network.
										</p>
									</div>
									<div className="rounded-lg bg-warn/10 border border-warn/20 p-4 text-sm">
										<p className="font-semibold text-foreground mb-1">
											Next step
										</p>
										<p className="text-muted-foreground">
											Connect to your new network{" "}
											<strong>{mainNetworkSSID || "your SSID"}</strong> using
											the Wi-Fi password you saved earlier, then open{" "}
											<code className="text-foreground">
												http://vantageos.local/
											</code>{" "}
											to access the dashboard.
										</p>
										{payload?.credentials && (
											<div className="mt-2 text-xs text-muted-foreground">
												Your saved password:{" "}
												<code className="text-foreground">
													{payload.credentials[0]?.secret}
												</code>
											</div>
										)}
									</div>
									<ul className="text-sm text-muted-foreground list-disc pl-4 grid gap-1">
										{auth?.password_set && <li>Admin password set</li>}
										{mainNetworkSSID && (
											<li>Wi-Fi network "{mainNetworkSSID}" created</li>
										)}
										{payload && <li>Device provisioned</li>}
									</ul>
									<Button onClick={onComplete} className="w-fit">
										Go to dashboard
									</Button>
								</div>
							) : (
								<div className="flex flex-col gap-4">
									<p className="text-sm text-muted-foreground">
										This will write all configuration files and restart the
										Wi-Fi access point with your secured network. The open setup
										network will disappear.
									</p>
									<div className="rounded-lg bg-warn/10 border border-warn/20 p-3 text-sm">
										<p className="font-semibold text-foreground mb-1">
											Make sure you have:
										</p>
										<ul className="text-muted-foreground list-disc pl-4 grid gap-1 mt-1">
											<li>Saved the Wi-Fi password from the previous step</li>
											<li>Your device ready to connect to the new network</li>
										</ul>
									</div>
									<Button
										onClick={handleApply}
										disabled={busy}
										className="w-fit"
									>
										{busy ? "Applying..." : "Apply configuration"}
									</Button>
								</div>
							)}
						</div>
					)}

					<div className="flex justify-between items-center mt-6 pt-4 border-t border-border">
						<Button
							variant="ghost"
							onClick={goBack}
							disabled={STEPS.indexOf(step) === 0 || busy || applied}
						>
							Back
						</Button>
						<span className="text-xs text-muted-foreground">
							Step {currentStepIdx} of {totalSteps}
						</span>
					</div>
				</CardContent>
			</Card>
		</div>
	);
}
