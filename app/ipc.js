const { ipcMain } = require('electron');

const BACKEND_URL = 'http://127.0.0.1:8080';

// Fetch the stored providers from the Go backend. The backend is spawned
// right before the window, so a few short retries cover the startup race.
async function fetchProviders() {
	for (let attempt = 0; attempt < 5; attempt++) {
		try {
			const res = await fetch(`${BACKEND_URL}/providers`);

			if (!res.ok) {
				return { ok: false, providers: [] };
			}

			const data = await res.json();

			return { ok: true, providers: data.providers || [] };
		} catch {
			await new Promise((resolve) => setTimeout(resolve, 150));
		}
	}

	return { ok: false, providers: [] };
}

// Forward the provider connection to the Go backend over HTTP. Runs in the
// main process so the renderer never talks to the network itself.
function registerBackendIpc() {
	ipcMain.handle('kairo:list-providers', async () => {
		return fetchProviders();
	});

	ipcMain.handle('kairo:connect-provider', async (_event, payload) => {
		const { provider, apiKey } = payload || {};

		if (!provider || !apiKey) {
			return { ok: false, error: 'provider and api key are required' };
		}

		try {
			const res = await fetch(`${BACKEND_URL}/providers`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name: provider, api_key: apiKey }),
			});

			// 201 = newly connected, 409 = already connected; both are success
			// from the user's point of view.
			return { ok: res.ok || res.status === 409, status: res.status };
		} catch {
			return { ok: false, error: 'Backend is not reachable.' };
		}
	});
}

module.exports = { registerBackendIpc };
