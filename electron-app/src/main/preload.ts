/**
 * Preload Script
 * 
 * Exposes a safe, limited API to the Renderer process.
 * This runs in a sandboxed context with access to Node.js APIs.
 */

import { contextBridge, ipcRenderer, IpcRendererEvent } from 'electron';

/**
 * Type definitions for the exposed API
 */
export interface ElectronAPI {
  goServer: {
    start: () => Promise<{ success: boolean; error?: string }>;
    stop: () => Promise<{ success: boolean }>;
    restart: () => Promise<{ success: boolean; error?: string }>;
    status: () => Promise<{ success: boolean; data?: { running: boolean; pid?: number; uptime?: number } }>;
    onLog: (callback: (log: { level: string; message: string }) => void) => () => void;
  };
  ws: {
    connect: () => Promise<{ success: boolean }>;
    send: (message: unknown) => Promise<{ success: boolean; error?: string }>;
    disconnect: () => Promise<{ success: boolean }>;
    status: () => Promise<{ success: boolean; data?: { connected: boolean } }>;
    onConnected: (callback: () => void) => () => void;
    onDisconnected: (callback: () => void) => () => void;
    onMessage: (callback: (data: unknown) => void) => () => void;
    onError: (callback: (error: string) => void) => () => void;
  };
  app: {
    platform: NodeJS.Platform;
    isPackaged: boolean;
  };
}

/**
 * Helper to create IPC event listeners with cleanup
 */
function createListener<T>(
  channel: string,
  callback: (data: T) => void
): () => void {
  const handler = (_event: IpcRendererEvent, data: T) => callback(data);
  ipcRenderer.on(channel, handler);
  return () => ipcRenderer.removeListener(channel, handler);
}

/**
 * Helper to create IPC event listeners without data
 */
function createSimpleListener(
  channel: string,
  callback: () => void
): () => void {
  const handler = () => callback();
  ipcRenderer.on(channel, handler);
  return () => ipcRenderer.removeListener(channel, handler);
}

/**
 * The API exposed to the Renderer process
 */
const electronAPI: ElectronAPI = {
  // Go Server control
  goServer: {
    start: () => ipcRenderer.invoke('go-server:start'),
    stop: () => ipcRenderer.invoke('go-server:stop'),
    restart: () => ipcRenderer.invoke('go-server:restart'),
    status: () => ipcRenderer.invoke('go-server:status'),
    onLog: (callback) => createListener('go-server:log', callback),
  },

  // WebSocket communication
  ws: {
    connect: () => ipcRenderer.invoke('ws:connect'),
    send: (message) => ipcRenderer.invoke('ws:send', message),
    disconnect: () => ipcRenderer.invoke('ws:disconnect'),
    status: () => ipcRenderer.invoke('ws:status'),
    onConnected: (callback) => createSimpleListener('ws:connected', callback),
    onDisconnected: (callback) => createSimpleListener('ws:disconnected', callback),
    onMessage: (callback) => createListener('ws:message', callback),
    onError: (callback) => createListener('ws:error', callback),
  },

  // App info
  app: {
    platform: process.platform,
    isPackaged: !process.defaultApp,
  },
};

// Expose the API to the Renderer process
contextBridge.exposeInMainWorld('electronAPI', electronAPI);

// Type declaration for global window object
declare global {
  interface Window {
    electronAPI: ElectronAPI;
  }
}
