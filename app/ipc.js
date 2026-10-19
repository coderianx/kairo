const { ipcMain, shell, clipboard } = require('electron');

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

	// Consume an SSE body, invoking onEvent for every `data:` payload and
	// returning when the stream ends with [DONE] (or the body does).
	async function readSSE(body, onEvent) {
		const decoder = new TextDecoder();

		let buffer = '';

		for await (const chunk of body) {
			buffer += decoder.decode(chunk, { stream: true });

			let index;

			while ((index = buffer.indexOf('\n\n')) !== -1) {
				const block = buffer.slice(0, index);

				buffer = buffer.slice(index + 2);

				for (const line of block.split('\n')) {
					if (!line.startsWith('data:')) {
						continue;
					}

					const data = line.slice(5).trim();

					if (data === '[DONE]') {
						return;
					}

					if (data) {
						onEvent(data);
					}
				}
			}
		}
	}

	ipcMain.handle('kairo:send-message', async (event, payload) => {
		const { chatId, provider, model, content } = payload || {};

		if (!Number.isInteger(chatId) || chatId <= 0) {
			return { ok: false, error: 'invalid chat' };
		}

		if (!provider || !model || !content) {
			return { ok: false, error: 'provider, model and message are required' };
		}

		try {
			// The streaming endpoint keeps the request open and emits the
			// reply as it is generated; chunks are forwarded to the renderer
			// over IPC while the final result is the handler's return value.
			const res = await fetch(`${BACKEND_URL}/chats/${chatId}/messages/stream`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name: provider, model, content }),
			});

			if (!res.ok || !res.body) {
				const data = await res.json().catch(() => ({}));

				return {
					ok: false,
					error: data.error || 'Could not send the message.',
				};
			}

			let final = null;
			let streamError = null;

			await readSSE(res.body, (data) => {
				let parsed;

				try {
					parsed = JSON.parse(data);
				} catch {
					return;
				}

				if (parsed.type === 'chunk' && parsed.content) {
					event.sender.send('kairo:stream-chunk', parsed);
				} else if (parsed.type === 'done') {
					final = parsed;
				} else if (parsed.type === 'error') {
					streamError = parsed.error || 'Stream failed.';
				}
			});

			if (streamError) {
				return { ok: false, error: streamError };
			}

			if (!final || !final.assistant_message) {
				return { ok: false, error: 'Stream ended unexpectedly.' };
			}

			return { ok: true, result: final };
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

	// Open markdown links in the system browser, never inside the app window.
	ipcMain.handle('kairo:open-external', async (_event, url) => {
		if (typeof url !== 'string' || !/^https?:\/\//i.test(url)) {
			return { ok: false, error: 'invalid url' };
		}

		try {
			await shell.openExternal(url);

			return { ok: true };
		} catch {
			return { ok: false, error: 'Could not open the link.' };
		}
	});

	// Copy rendered message content (e.g. code blocks) to the clipboard.
	ipcMain.handle('kairo:copy-text', async (_event, text) => {
		if (typeof text !== 'string') {
			return { ok: false, error: 'text is required' };
		}

		clipboard.writeText(text);

		return { ok: true };
	});
}

module.exports = { registerBackendIpc };
