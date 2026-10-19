const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('kairo', {
	listProviders: () => ipcRenderer.invoke('kairo:list-providers'),
	listModels: (provider) => ipcRenderer.invoke('kairo:list-models', provider),
	deleteProvider: (id) => ipcRenderer.invoke('kairo:delete-provider', id),
	connectProvider: (payload) =>
		ipcRenderer.invoke('kairo:connect-provider', payload),
	listChats: () => ipcRenderer.invoke('kairo:list-chats'),
	createChat: (payload) => ipcRenderer.invoke('kairo:create-chat', payload),
	getChatMessages: (id) => ipcRenderer.invoke('kairo:get-chat-messages', id),
	sendMessage: (payload) => ipcRenderer.invoke('kairo:send-message', payload),
	openExternal: (url) => ipcRenderer.invoke('kairo:open-external', url),
	copyText: (text) => ipcRenderer.invoke('kairo:copy-text', text),
	// Subscribe to streaming reply chunks; returns an unsubscribe function.
	onStreamChunk: (callback) => {
		const listener = (_event, payload) => callback(payload);

		ipcRenderer.on('kairo:stream-chunk', listener);

		return () => ipcRenderer.removeListener('kairo:stream-chunk', listener);
	},
});
