/**
 * IPC Handlers
 * 
 * Sets up all IPC communication between Main and Renderer processes.
 */

import { ipcMain, BrowserWindow } from 'electron';
import { ProcessManager } from './process-manager';
import { WebSocketBridge } from './websocket-bridge';
import { createLogger } from './logger';
import type { IpcResponse } from './types';

const logger = createLogger('IPC');

export function setupIpcHandlers(
  processManager: ProcessManager,
  mainWindow: BrowserWindow
): { cleanup: () => void } {
  const wsBridge = new WebSocketBridge(mainWindow);

  // ============ Go Server Control ============

  ipcMain.handle('go-server:start', async (): Promise<IpcResponse> => {
    try {
      await processManager.start();
      return { success: true };
    } catch (err) {
      const error = err instanceof Error ? err.message : String(err);
      logger.error('Failed to start Go Server', error);
      return { success: false, error };
    }
  });

  ipcMain.handle('go-server:stop', (): IpcResponse => {
    processManager.stop();
    return { success: true };
  });

  ipcMain.handle('go-server:restart', async (): Promise<IpcResponse> => {
    try {
      await processManager.restart();
      return { success: true };
    } catch (err) {
      const error = err instanceof Error ? err.message : String(err);
      logger.error('Failed to restart Go Server', error);
      return { success: false, error };
    }
  });

  ipcMain.handle('go-server:status', (): IpcResponse => {
    return { success: true, data: processManager.getStatus() };
  });

  // ============ WebSocket Control ============

  ipcMain.handle('ws:connect', (): IpcResponse => {
    wsBridge.connect();
    return { success: true };
  });

  ipcMain.handle('ws:send', (_, message: unknown): IpcResponse => {
    return wsBridge.sendMessage(message);
  });

  ipcMain.handle('ws:disconnect', (): IpcResponse => {
    wsBridge.disconnect();
    return { success: true };
  });

  ipcMain.handle('ws:status', (): IpcResponse => {
    return { success: true, data: { connected: wsBridge.isConnected() } };
  });

  // ============ Process Manager Events ============

  processManager.on('ready', () => {
    logger.info('Go Server ready, connecting WebSocket');
    wsBridge.connect();
  });

  processManager.on('exit', (info) => {
    logger.info('Go Server exited', info);
    wsBridge.disconnect();
  });

  processManager.on('log', (log) => {
    if (!mainWindow.isDestroyed()) {
      mainWindow.webContents.send('go-server:log', log);
    }
  });

  // ============ Window Events ============

  mainWindow.on('closed', () => {
    wsBridge.destroy();
  });

  // ============ Cleanup Function ============

  const cleanup = () => {
    wsBridge.destroy();
    
    // Remove IPC handlers
    ipcMain.removeHandler('go-server:start');
    ipcMain.removeHandler('go-server:stop');
    ipcMain.removeHandler('go-server:restart');
    ipcMain.removeHandler('go-server:status');
    ipcMain.removeHandler('ws:connect');
    ipcMain.removeHandler('ws:send');
    ipcMain.removeHandler('ws:disconnect');
    ipcMain.removeHandler('ws:status');
  };

  return { cleanup };
}
