/**
 * WebSocket Bridge
 * 
 * Manages WebSocket connection to Go Server and bridges
 * messages between Main Process and Renderer via IPC.
 */

import WebSocket from 'ws';
import { BrowserWindow } from 'electron';
import { config } from './config';
import { createLogger } from './logger';

const logger = createLogger('WebSocketBridge');

export class WebSocketBridge {
  private ws: WebSocket | null = null;
  private reconnectTimer: NodeJS.Timeout | null = null;
  private isConnecting = false;
  private shouldReconnect = true;
  private mainWindow: BrowserWindow;

  constructor(mainWindow: BrowserWindow) {
    this.mainWindow = mainWindow;
  }

  /**
   * Connect to Go Server WebSocket
   */
  connect(): void {
    if (this.isConnecting) {
      logger.debug('Connection already in progress');
      return;
    }

    if (this.ws?.readyState === WebSocket.OPEN) {
      logger.debug('Already connected');
      return;
    }

    this.cleanup();
    this.isConnecting = true;
    this.shouldReconnect = true;

    logger.info('Connecting to Go Server', { url: config.websocket.url });

    this.ws = new WebSocket(config.websocket.url);

    this.ws.on('open', () => {
      this.isConnecting = false;
      logger.info('WebSocket connected');
      this.send('ws:connected');
      this.clearReconnectTimer();
    });

    this.ws.on('message', (data: WebSocket.Data) => {
      try {
        const message = JSON.parse(data.toString());
        this.send('ws:message', message);
      } catch (err) {
        logger.error('Failed to parse message', err);
      }
    });

    this.ws.on('close', (code, reason) => {
      this.isConnecting = false;
      logger.info('WebSocket disconnected', { code, reason: reason.toString() });
      this.ws = null;
      this.send('ws:disconnected');
      this.scheduleReconnect();
    });

    this.ws.on('error', (err) => {
      this.isConnecting = false;
      logger.error('WebSocket error', err.message);
      this.send('ws:error', err.message);
    });
  }

  /**
   * Send message to Go Server
   */
  sendMessage(message: unknown): { success: boolean; error?: string } {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      return { success: false, error: 'Not connected' };
    }

    try {
      this.ws.send(JSON.stringify(message));
      return { success: true };
    } catch (err) {
      const error = err instanceof Error ? err.message : String(err);
      logger.error('Failed to send message', error);
      return { success: false, error };
    }
  }

  /**
   * Disconnect from Go Server
   */
  disconnect(): void {
    this.shouldReconnect = false;
    this.clearReconnectTimer();
    this.cleanup();
    logger.info('WebSocket disconnected (manual)');
  }

  /**
   * Check if connected
   */
  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  /**
   * Clean up WebSocket connection
   */
  private cleanup(): void {
    if (this.ws) {
      this.ws.removeAllListeners();
      if (this.ws.readyState === WebSocket.OPEN || 
          this.ws.readyState === WebSocket.CONNECTING) {
        this.ws.close();
      }
      this.ws = null;
    }
  }

  /**
   * Schedule reconnection attempt
   */
  private scheduleReconnect(): void {
    if (!this.shouldReconnect) {
      return;
    }

    if (this.reconnectTimer) {
      return;
    }

    logger.debug('Scheduling reconnect', { delay: config.websocket.reconnectDelay });

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      if (this.shouldReconnect) {
        this.connect();
      }
    }, config.websocket.reconnectDelay);
  }

  /**
   * Clear reconnect timer
   */
  private clearReconnectTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  /**
   * Send IPC message to renderer
   */
  private send(channel: string, data?: unknown): void {
    if (!this.mainWindow.isDestroyed()) {
      if (data !== undefined) {
        this.mainWindow.webContents.send(channel, data);
      } else {
        this.mainWindow.webContents.send(channel);
      }
    }
  }

  /**
   * Destroy the bridge
   */
  destroy(): void {
    this.disconnect();
  }
}
