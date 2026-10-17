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
});
