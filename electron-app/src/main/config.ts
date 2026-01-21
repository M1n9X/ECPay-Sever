/**
 * Application Configuration
 */

export const config = {
  // Go Server 配置
  goServer: {
    port: 8989,
    host: '127.0.0.1',
    maxRestarts: 5,
    restartDelay: 3000,
    startupTimeout: 10000,
  },

  // WebSocket 配置
  websocket: {
    reconnectDelay: 3000,
    url: 'ws://127.0.0.1:8989/ws',
  },

  // 窗口配置
  window: {
    width: 1200,
    height: 800,
    minWidth: 800,
    minHeight: 600,
  },

  // 开发服务器
  devServer: {
    url: 'http://localhost:5173',
  },
} as const;

export type Config = typeof config;
