/**
 * ECPay POS Electron Application
 * 
 * Main Process Entry Point
 */

import { app, BrowserWindow } from 'electron';
import path from 'path';
import { ProcessManager } from './process-manager';
import { setupIpcHandlers } from './ipc-handlers';
import { config } from './config';
import { logger } from './logger';

// Keep references to prevent garbage collection
let mainWindow: BrowserWindow | null = null;
let processManager: ProcessManager | null = null;
let ipcCleanup: (() => void) | null = null;

/**
 * Create the main application window
 */
async function createWindow(): Promise<BrowserWindow> {
  const window = new BrowserWindow({
    width: config.window.width,
    height: config.window.height,
    minWidth: config.window.minWidth,
    minHeight: config.window.minHeight,
    webPreferences: {
      nodeIntegration: false,
      contextIsolation: true,
      sandbox: true,
      preload: path.join(__dirname, 'preload.js'),
    },
    title: 'ECPay POS',
    show: false,
    backgroundColor: '#09090b',
  });

  // Show window when ready to prevent white flash
  window.once('ready-to-show', () => {
    window.show();
  });

  // Load content
  if (!app.isPackaged) {
    // Development: load from Vite dev server
    logger.info('Loading from dev server', { url: config.devServer.url });
    await window.loadURL(config.devServer.url);
    window.webContents.openDevTools();
  } else {
    // Production: load from bundled files
    const indexPath = path.join(__dirname, '../renderer/index.html');
    logger.info('Loading from file', { path: indexPath });
    await window.loadFile(indexPath);
  }

  window.on('closed', () => {
    mainWindow = null;
  });

  return window;
}

/**
 * Initialize the application
 */
async function initialize(): Promise<void> {
  logger.info('Application starting', {
    version: app.getVersion(),
    platform: process.platform,
    arch: process.arch,
    packaged: app.isPackaged,
  });

  // Create process manager
  processManager = new ProcessManager();

  // Start Go Server
  try {
    await processManager.start();
  } catch (err) {
    logger.error('Failed to start Go Server', err);
    // Continue anyway - user can manually restart
  }

  // Create main window
  mainWindow = await createWindow();

  // Setup IPC handlers
  const { cleanup } = setupIpcHandlers(processManager, mainWindow);
  ipcCleanup = cleanup;

  logger.info('Application initialized');
}

/**
 * Cleanup before quit
 */
function cleanup(): void {
  logger.info('Application shutting down');

  if (ipcCleanup) {
    ipcCleanup();
    ipcCleanup = null;
  }

  if (processManager) {
    processManager.stop();
    processManager = null;
  }
}

// ============ App Lifecycle ============

app.whenReady().then(initialize).catch((err) => {
  logger.error('Failed to initialize application', err);
  app.quit();
});

app.on('window-all-closed', () => {
  // On macOS, apps typically stay open until explicitly quit
  if (process.platform !== 'darwin') {
    app.quit();
  }
});

app.on('activate', () => {
  // On macOS, re-create window when dock icon is clicked
  if (BrowserWindow.getAllWindows().length === 0) {
    createWindow().then(window => {
      mainWindow = window;
      if (processManager && mainWindow) {
        const { cleanup } = setupIpcHandlers(processManager, mainWindow);
        ipcCleanup = cleanup;
      }
    });
  }
});

app.on('before-quit', cleanup);

// ============ Error Handling ============

process.on('uncaughtException', (error) => {
  logger.error('Uncaught Exception', error);
});

process.on('unhandledRejection', (reason) => {
  logger.error('Unhandled Rejection', reason);
});

// ============ Security ============

// Prevent new window creation
app.on('web-contents-created', (_, contents) => {
  contents.setWindowOpenHandler(() => {
    return { action: 'deny' };
  });
});
