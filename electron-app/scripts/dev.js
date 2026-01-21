/**
 * Development script for Electron app
 * 
 * This script:
 * 1. Compiles TypeScript for main process
 * 2. Starts Vite dev server for renderer
 * 3. Starts Electron when Vite is ready
 */

const { spawn } = require('child_process');
const path = require('path');

const isWindows = process.platform === 'win32';
const npm = isWindows ? 'npm.cmd' : 'npm';

console.log('🚀 Starting Electron development environment...\n');

// Step 1: Compile main process TypeScript
console.log('📦 Compiling main process...');
const tsc = spawn(npm, ['run', 'build:main'], {
  stdio: 'inherit',
  shell: true,
});

tsc.on('close', (code) => {
  if (code !== 0) {
    console.error('❌ TypeScript compilation failed');
    process.exit(1);
  }

  console.log('✅ Main process compiled\n');

  // Step 2: Start Vite dev server
  console.log('🌐 Starting Vite dev server...');
  const vite = spawn(npm, ['run', 'dev:renderer'], {
    stdio: 'inherit',
    shell: true,
  });

  // Wait a bit for Vite to start, then launch Electron
  setTimeout(() => {
    console.log('\n⚡ Starting Electron...');
    const electron = spawn(npm, ['run', 'start'], {
      stdio: 'inherit',
      shell: true,
      env: {
        ...process.env,
        NODE_ENV: 'development',
      },
    });

    electron.on('close', () => {
      vite.kill();
      process.exit(0);
    });
  }, 3000);

  // Handle Ctrl+C
  process.on('SIGINT', () => {
    vite.kill();
    process.exit(0);
  });
});
