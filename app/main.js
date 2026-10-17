const { app, BrowserWindow } = require('electron');
const { spawn } = require('node:child_process');
const path = require('node:path');
const { registerBackendIpc } = require('./ipc');

const UI_PATH = path.join(__dirname, '..', 'ui', 'index.html');

let backend;

// In development the backend binary sits in the project root; when packaged
// electron-builder places it under extraResources.
function backendPath() {
	return app.isPackaged
		? path.join(process.resourcesPath, 'kairo-server')
		: path.join(__dirname, '..', 'kairo-server');
}

function createWindow() {
	const win = new BrowserWindow({
		width: 960,
		height: 640,
		minWidth: 480,
		minHeight: 360,
		title: 'Kairo',
		frame: false,
		maximizable: false,
		backgroundColor: '#0b0d10',
		webPreferences: {
			preload: path.join(__dirname, 'preload.js'),
			contextIsolation: true,
			nodeIntegration: false,
		},
	});

	win.loadFile(UI_PATH);
}

registerBackendIpc();

app.whenReady().then(() => {
	backend = spawn(backendPath(), [], {
		stdio: 'inherit',
	});

	backend.on('error', (err) => {
		console.error('Failed to start Go backend:', err);
	});

	createWindow();

	// macOS: re-create a window when the dock icon is clicked and none is open.
	app.on('activate', () => {
		if (BrowserWindow.getAllWindows().length === 0) {
			createWindow();
		}
	});
});

app.on('before-quit', () => {
	backend?.kill();
});

app.on('window-all-closed', () => {
	if (process.platform !== 'darwin') {
		app.quit();
	}
});
