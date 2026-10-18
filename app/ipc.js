const { ipcMain } = require('electron');

const BACKEND_URL = 'http://127.0.0.1:8080';

// Generic backend fetch that returns { status, data } and optionally retries
// while the backend process is still booting.
async function fetchJSON(path, options = {}, retries = 0) {
	for (let attempt = 0; ; attempt++) {
		try {
			const res = await fetch(`${BACKEND_URL}${path}`, options);

			const data = await res.json().catch(() => ({}));

			return { status: res.status, data };
		} catch (err) {
			if (attempt >= retries) {
				throw err;
			}

			await new Promise((resolve) => setTimeout(resolve, 150));
		}
	}
}

// Fetch the stored providers from the Go backend.
async function fetchProviders() {
	try {
		const res = await fetchJSON('/providers', {}, 5);

		if (res.status !== 200) {
			return { ok: false, providers: [] };
		}

		return { ok: true, providers: res.data.providers || [] };
	} catch {
		return { ok: false, providers: [] };
	}
}

// Forward the provider connection to the Go backend over HTTP. Runs in the
// main process so the renderer never talks to the network itself.
function registerBackendIpc() {
	ipcMain.handle('kairo:list-providers', async () => {
		return fetchProviders();
	});

	ipcMain.handle('kairo:delete-provider', async (_event, id) => {
		if (!Number.isInteger(id) || id <= 0) {
			return { ok: false, error: 'invalid id' };
		}

		try {
			const res = await fetch(`${BACKEND_URL}/providers/${id}`, {
				method: 'DELETE',
			});

			return { ok: res.ok, status: res.status };
		} catch {
			return { ok: false, error: 'Backend is not reachable.' };
		}
	});

	ipcMain.handle('kairo:list-chats', async () => {
		try {
			const res = await fetchJSON('/chats', {}, 5);

			if (res.status !== 200) {
				return { ok: false, chats: [] };
			}

			return { ok: true, chats: res.data.chats || [] };
		} catch {
			return { ok: false, chats: [] };
		}
	});

	ipcMain.handle('kairo:get-chat-messages', async (_event, id) => {
		if (!Number.isInteger(id) || id <= 0) {
			return { ok: false, messages: [] };
		}

		try {
			const res = await fetchJSON(`/chats/${id}/messages`, {}, 2);

			if (res.status !== 200) {
				return { ok: false, messages: [] };
			}

			return { ok: true, messages: res.data.messages || [] };
		} catch {
			return { ok: false, messages: [] };
		}
	});

	ipcMain.handle('kairo:create-chat', async (_event, payload) => {
		const body = {};

		if (payload && payload.title) {
			body.title = payload.title;
		}

		try {
			const res = await fetchJSON('/chats', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(body),
			});

			if (res.status !== 201) {
				return { ok: false, error: res.data.error || 'Create chat failed.' };
			}

			return { ok: true, chat: res.data };
		} catch {
			return { ok: false, error: 'Backend is not reachable.' };
		}
	});

	ipcMain.handle('kairo:list-models', async (_event, provider) => {
		if (!provider) {
			return { ok: false, models: [], error: 'provider is required' };
		}

		try {
			const res = await fetchJSON(
				`/providers/${encodeURIComponent(provider)}/models`,
				{},
				2,
			);

			if (res.status !== 200) {
				return {
					ok: false,
					models: [],
					error: res.data.error || 'Could not load models.',
				};
			}

			return { ok: true, models: res.data.models || [] };
		} catch {
			return { ok: false, models: [], error: 'Backend is not reachable.' };
		}
	});

	ipcMain.handle('kairo:send-message', async (_event, payload) => {
		const { chatId, provider, model, content } = payload || {};

		if (!Number.isInteger(chatId) || chatId <= 0) {
			return { ok: false, error: 'invalid chat' };
		}

		if (!provider || !model || !content) {
			return { ok: false, error: 'provider, model and message are required' };
		}

		try {
			const res = await fetchJSON(`/chats/${chatId}/messages`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name: provider, model, content }),
			});

			if (res.status !== 201) {
				return {
					ok: false,
					error: res.data.error || 'Could not send the message.',
				};
			}

			return { ok: true, result: res.data };
		} catch {
			return { ok: false, error: 'Backend is not reachable.' };
		}
	});

	ipcMain.handle('kairo:connect-provider', async (_event, payload) => {
		const { provider, apiKey } = payload || {};

		// Ollama runs locally and connects without an API key.
		if (!provider || (!apiKey && provider !== 'ollama')) {
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
