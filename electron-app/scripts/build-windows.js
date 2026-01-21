#!/usr/bin/env node

/**
 * Windows Build Script for ECPay POS
 * 
 * Usage:
 *   npm run build:win                    # Default build (x64, production)
 *   npm run build:win -- --debug         # Debug mode (keep source maps)
 *   npm run build:win -- --arm64         # ARM64 architecture
 *   npm run build:win -- --all           # Build both x64 and ARM64
 *   npm run build:win -- --clean         # Clean build
 */

const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const args = process.argv.slice(2);
const options = {
  debug: args.includes('--debug'),
  arm64: args.includes('--arm64'),
  x64: !args.includes('--arm64') || args.includes('--all'),
  all: args.includes('--all'),
  clean: args.includes('--clean'),
};

const projectRoot = path.join(__dirname, '..');
const releaseDir = path.join(projectRoot, 'release');
const isWindows = process.platform === 'win32';

console.log('🔨 ECPay POS Windows Build Script');
console.log('================================\n');

// Helper: remove directory cross-platform
function rmDir(dir) {
  if (fs.existsSync(dir)) {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

// Clean if requested
if (options.clean) {
  console.log('🧹 Cleaning previous builds...');
  rmDir(releaseDir);
  rmDir(path.join(projectRoot, 'dist'));
  console.log('✓ Clean complete\n');
}

// Build Go server for Windows x64
console.log('📦 Building Go server (Windows x64)...');
try {
  execSync('npm run build:go:win', { cwd: projectRoot, stdio: 'inherit' });
  console.log('✓ Go server built\n');
} catch (error) {
  console.error('✗ Failed to build Go server');
  process.exit(1);
}

// Build TypeScript and Renderer
console.log('🏗️  Building TypeScript and Renderer...');
try {
  execSync('npm run build', { cwd: projectRoot, stdio: 'inherit' });
  console.log('✓ Build complete\n');
} catch (error) {
  console.error('✗ Failed to build');
  process.exit(1);
}

// Determine architectures to build
const archs = [];
if (options.x64 || options.all) archs.push({ arch: 'x64', name: 'x64 (Intel/AMD)' });
if (options.arm64 || options.all) archs.push({ arch: 'arm64', name: 'ARM64' });

console.log(`📦 Building Windows installer: ${archs.map(a => a.name).join(', ')}\n`);

for (const { arch, name } of archs) {
  console.log(`🔨 Building for ${name}...`);
  
  try {
    const builderArgs = ['electron-builder', '--win', `--${arch}`, '-p', 'never'];
    
    execSync(`npx ${builderArgs.join(' ')}`, { 
      cwd: projectRoot, 
      stdio: 'inherit',
      env: { ...process.env, CSC_IDENTITY_AUTO_DISCOVERY: 'false' }
    });
    
    console.log(`✓ ${name} build complete\n`);
  } catch (error) {
    console.error(`✗ Failed to build for ${name}`);
    process.exit(1);
  }
}

// Summary
console.log('================================');
console.log('✅ Build Complete!\n');

if (fs.existsSync(releaseDir)) {
  const files = fs.readdirSync(releaseDir).filter(f => f.endsWith('.exe'));
  if (files.length > 0) {
    console.log('📁 Output:');
    files.forEach(file => {
      const stats = fs.statSync(path.join(releaseDir, file));
      console.log(`   ${file} (${(stats.size / 1024 / 1024).toFixed(1)} MB)`);
    });
  }
}

console.log(`\n📍 Location: ${releaseDir}`);
console.log(`⚙️  Mode: ${options.debug ? 'Debug' : 'Production'}`);
