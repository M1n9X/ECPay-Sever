/**
 * Go Server Process Manager
 * 
 * Manages the lifecycle of the Go Server child process including:
 * - Starting and stopping the server
 * - Auto-restart on crash
 * - Health checking
 * - Log forwarding
 */

import { spawn, ChildProcess } from 'child_process';
import { app } from 'electron';
import path from 'path';
import fs from 'fs';
import net from 'net';
import { EventEmitter } from 'events';
import { config } from './config';
import { createLogger } from './logger';
import type { GoServerExitInfo, GoServerLog, ProcessManagerEvents } from './types';

const logger = createLogger('ProcessManager');

export class ProcessManager extends EventEmitter {
  private goServer: ChildProcess | null = null;
  private restartCount = 0;
  private isShuttingDown = false;
  private startTime: number | null = null;

  constructor() {
    super();
  }

  // Type-safe event emitter
  override emit<K extends keyof ProcessManagerEvents>(
    event: K,
    ...args: Parameters<ProcessManagerEvents[K]>
  ): boolean {
    return super.emit(event, ...args);
  }

  override on<K extends keyof ProcessManagerEvents>(
    event: K,
    listener: ProcessManagerEvents[K]
  ): this {
    return super.on(event, listener);
  }

  /**
   * Get the path to the Go Server executable
   */
  getServerPath(): string {
    const ext = process.platform === 'win32' ? '.exe' : '';
    const binaryName = `ecpay-server${ext}`;

    if (app.isPackaged) {
      // Production: resources/bin/ecpay-server
      return path.join(process.resourcesPath, 'bin', binaryName);
    }

    // Development: check multiple locations
    const candidates = [
      path.join(__dirname, '../../resources/bin', binaryName),
      path.join(__dirname, '../../../resources/bin', binaryName),
      path.join(__dirname, '../../../server', binaryName),
    ];

    for (const candidate of candidates) {
      if (fs.existsSync(candidate)) {
        return candidate;
      }
    }

    // Return first candidate path for error message
    return candidates[0];
  }

  /**
   * Start the Go Server
   */
  async start(): Promise<void> {
    if (this.goServer && !this.goServer.killed) {
      logger.info('Go Server already running');
      return;
    }

    this.isShuttingDown = false;
    const serverPath = this.getServerPath();
    
    logger.info('Starting Go Server', { path: serverPath });

    // Verify executable exists
    if (!fs.existsSync(serverPath)) {
      const error = new Error(`Go Server executable not found: ${serverPath}`);
      logger.error('Executable not found', { path: serverPath });
      throw error;
    }

    // Spawn the process
    this.goServer = spawn(serverPath, [], {
      stdio: ['ignore', 'pipe', 'pipe'],
      windowsHide: true,
      cwd: path.dirname(serverPath),
    });

    this.startTime = Date.now();

    // Handle stdout
    this.goServer.stdout?.on('data', (data: Buffer) => {
      const lines = data.toString().trim().split('\n');
      for (const line of lines) {
        if (line) {
          logger.debug(`[Go] ${line}`);
          this.emit('log', { level: 'INFO', message: line });
        }
      }
    });

    // Handle stderr
    this.goServer.stderr?.on('data', (data: Buffer) => {
      const lines = data.toString().trim().split('\n');
      for (const line of lines) {
        if (line) {
          logger.warn(`[Go Error] ${line}`);
          this.emit('log', { level: 'ERROR', message: line });
        }
      }
    });

    // Handle exit
    this.goServer.on('exit', (code, signal) => {
      const uptime = this.startTime ? Date.now() - this.startTime : 0;
      logger.info('Go Server exited', { code, signal, uptime: `${uptime}ms` });
      
      this.goServer = null;
      this.startTime = null;
      this.emit('exit', { code, signal });

      // Auto-restart logic
      this.handleAutoRestart(code);
    });

    // Handle spawn error
    this.goServer.on('error', (err) => {
      logger.error('Failed to spawn Go Server', err);
      this.emit('error', err);
    });

    // Wait for server to be ready
    await this.waitForReady();
    this.restartCount = 0;
    logger.info('Go Server is ready');
    this.emit('ready');
  }

  /**
   * Handle auto-restart logic
   */
  private handleAutoRestart(exitCode: number | null): void {
    if (this.isShuttingDown) {
      logger.debug('Shutdown in progress, skipping auto-restart');
      return;
    }

    if (exitCode === 0) {
      logger.debug('Clean exit, skipping auto-restart');
      return;
    }

    if (this.restartCount >= config.goServer.maxRestarts) {
      logger.error('Max restart attempts reached', { 
        attempts: this.restartCount,
        max: config.goServer.maxRestarts 
      });
      return;
    }

    this.restartCount++;
    logger.info('Scheduling auto-restart', {
      attempt: this.restartCount,
      max: config.goServer.maxRestarts,
      delay: config.goServer.restartDelay,
    });

    setTimeout(() => {
      if (!this.isShuttingDown) {
        this.start().catch(err => {
          logger.error('Auto-restart failed', err);
        });
      }
    }, config.goServer.restartDelay);
  }

  /**
   * Wait for Go Server to be ready (port is listening)
   */
  private async waitForReady(): Promise<void> {
    const { host, port, startupTimeout } = config.goServer;
    const start = Date.now();

    while (Date.now() - start < startupTimeout) {
      const isReady = await this.checkPort(host, port);
      if (isReady) {
        return;
      }
      await this.sleep(200);
    }

    logger.warn('Server ready check timed out, continuing anyway');
  }

  /**
   * Check if a port is accepting connections
   */
  private checkPort(host: string, port: number): Promise<boolean> {
    return new Promise((resolve) => {
      const socket = new net.Socket();
      socket.setTimeout(500);

      socket.on('connect', () => {
        socket.destroy();
        resolve(true);
      });

      socket.on('error', () => {
        socket.destroy();
        resolve(false);
      });

      socket.on('timeout', () => {
        socket.destroy();
        resolve(false);
      });

      socket.connect(port, host);
    });
  }

  /**
   * Stop the Go Server
   */
  stop(): void {
    this.isShuttingDown = true;

    if (!this.goServer) {
      logger.debug('Go Server not running');
      return;
    }

    logger.info('Stopping Go Server', { pid: this.goServer.pid });

    if (process.platform === 'win32') {
      // Windows: use taskkill for clean shutdown
      if (this.goServer.pid) {
        spawn('taskkill', ['/pid', String(this.goServer.pid), '/f', '/t']);
      }
    } else {
      // Unix: send SIGTERM for graceful shutdown
      this.goServer.kill('SIGTERM');
      
      // Force kill after timeout
      setTimeout(() => {
        if (this.goServer && !this.goServer.killed) {
          logger.warn('Force killing Go Server');
          this.goServer.kill('SIGKILL');
        }
      }, 5000);
    }

    this.goServer = null;
  }

  /**
   * Restart the Go Server
   */
  async restart(): Promise<void> {
    logger.info('Restarting Go Server');
    this.stop();
    await this.sleep(1000);
    this.restartCount = 0;
    await this.start();
  }

  /**
   * Check if Go Server is running
   */
  isRunning(): boolean {
    return this.goServer !== null && !this.goServer.killed;
  }

  /**
   * Get server status
   */
  getStatus(): { running: boolean; pid?: number; uptime?: number } {
    if (!this.isRunning()) {
      return { running: false };
    }

    return {
      running: true,
      pid: this.goServer?.pid,
      uptime: this.startTime ? Date.now() - this.startTime : undefined,
    };
  }

  private sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }
}
