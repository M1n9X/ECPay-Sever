/**
 * Application Configuration
 */

type ParsedAddress = {
  host: string;
  port: number;
};

const envValue = (key: string): string | undefined => {
  const value = process.env[key];
  if (!value) return undefined;
  const trimmed = value.trim();
  return trimmed.length ? trimmed : undefined;
};

const parseNumber = (value: string | undefined): number | undefined => {
  if (!value) return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
};

const parseBool = (value: string | undefined): boolean | undefined => {
  if (!value) return undefined;
  const normalized = value.trim().toLowerCase();
  if (['1', 'true', 'yes', 'on'].includes(normalized)) return true;
  if (['0', 'false', 'no', 'off'].includes(normalized)) return false;
  return undefined;
};

const parseWsAddr = (
  addr: string,
  fallbackHost: string,
  fallbackPort: number
): ParsedAddress => {
  let host = fallbackHost;
  let port = fallbackPort;

  if (addr.startsWith('[')) {
    const closeIdx = addr.indexOf(']');
    if (closeIdx !== -1) {
      host = addr.slice(1, closeIdx) || host;
      const rest = addr.slice(closeIdx + 1);
      if (rest.startsWith(':')) {
        port = parseNumber(rest.slice(1)) ?? port;
      }
    }
    return { host, port };
  }

  if (addr.includes(':')) {
    const [maybeHost, maybePort] = addr.split(':');
    if (maybeHost) host = maybeHost;
    if (maybePort) port = parseNumber(maybePort) ?? port;
    return { host, port };
  }

  if (addr) {
    host = addr;
  }

  return { host, port };
};

const resolveClientHost = (host: string): string => {
  if (host === '0.0.0.0' || host === '::') {
    return '127.0.0.1';
  }
  return host;
};

const isDev = process.env.NODE_ENV === 'development';

const wsHostEnv = envValue('TCB_WS_HOST');
const wsPortEnv = parseNumber(envValue('TCB_WS_PORT'));
const wsAddrEnv = envValue('TCB_WS_ADDR');

let wsHost = wsHostEnv ?? '127.0.0.1';
let wsPort = wsPortEnv ?? 8989;

if (wsAddrEnv && !wsHostEnv && !wsPortEnv) {
  const parsed = parseWsAddr(wsAddrEnv, wsHost, wsPort);
  wsHost = parsed.host;
  wsPort = parsed.port;
}

const wsAddr = wsAddrEnv ?? `${wsHost}:${wsPort}`;
const wsClientHost = resolveClientHost(wsHost);

const serialPort =
  envValue('TCB_SERIAL_PORT') ?? (isDev ? 'tcp://localhost:9999' : '');

const baudRate = parseNumber(envValue('TCB_BAUD')) ?? 115200;
const autoDetect = parseBool(envValue('TCB_AUTODETECT')) ?? false;

export const config = {
  // Go Server 配置
  goServer: {
    port: wsPort,
    host: wsHost,
    healthHost: wsClientHost,
    wsAddr,
    serialPort,
    baudRate,
    autoDetect,
    maxRestarts: 5,
    restartDelay: 3000,
    startupTimeout: 10000,
  },

  // WebSocket 配置
  websocket: {
    reconnectDelay: 3000,
    url: envValue('TCB_WS_URL') ?? `ws://${wsClientHost}:${wsPort}/ws`,
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
