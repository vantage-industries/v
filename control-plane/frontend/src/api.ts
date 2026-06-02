export async function getJson<T = unknown>(path: string): Promise<T> {
	const response = await fetch(path, {
		credentials: "same-origin",
		headers: { Accept: "application/json" },
	});

	if (response.status === 401) {
		window.dispatchEvent(new CustomEvent("vantageos:unauthorized"));
		throw new Error(`GET ${path} failed with 401`);
	}

	if (!response.ok) {
		const text = await response.text();
		let message: string;
		try {
			const parsed = JSON.parse(text);
			message = parsed.error || parsed.message || text;
		} catch {
			message = text;
		}
		throw new Error(message || `GET ${path} failed with ${response.status}`);
	}

	return response.json() as Promise<T>;
}

export async function postJson<T = unknown>(
	path: string,
	body?: unknown,
): Promise<T> {
	const response = await fetch(path, {
		method: "POST",
		credentials: "same-origin",
		headers: {
			"Content-Type": "application/json",
			Accept: "application/json",
		},
		body: body != null ? JSON.stringify(body) : undefined,
	});

	if (response.status === 401) {
		window.dispatchEvent(new CustomEvent("vantageos:unauthorized"));
		throw new Error(`POST ${path} failed with 401`);
	}

	if (!response.ok) {
		const text = await response.text();
		let message: string;
		try {
			const parsed = JSON.parse(text);
			message = parsed.error || parsed.message || text;
		} catch {
			message = text;
		}
		throw new Error(message || `POST ${path} failed with ${response.status}`);
	}

	return response.json() as Promise<T>;
}
