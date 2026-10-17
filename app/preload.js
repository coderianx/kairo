const { contextBridge, ipcRenderer } = require('electron');

contextBridge.exposeInMainWorld('kairo', {
	listProviders: () => ipcRenderer.invoke('kairo:list-providers'),
	connectProvider: (payload) =>
		ipcRenderer.invoke('kairo:connect-provider', payload),
});
