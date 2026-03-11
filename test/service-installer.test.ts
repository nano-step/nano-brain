import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  detectPlatform,
  generateLaunchdPlist,
  generateSystemdService,
  getDefaultServiceConfig,
  type ServiceConfig,
} from '../src/service-installer.js';

describe('service-installer', () => {
  describe('detectPlatform', () => {
    const originalPlatform = process.platform;

    afterEach(() => {
      vi.stubGlobal('process', { ...process, platform: originalPlatform });
    });

    it('returns macos for darwin', () => {
      vi.stubGlobal('process', { ...process, platform: 'darwin' });
      expect(detectPlatform()).toBe('macos');
    });

    it('returns linux for linux', () => {
      vi.stubGlobal('process', { ...process, platform: 'linux' });
      expect(detectPlatform()).toBe('linux');
    });

    it('returns unsupported for win32', () => {
      vi.stubGlobal('process', { ...process, platform: 'win32' });
      expect(detectPlatform()).toBe('unsupported');
    });
  });

  describe('generateLaunchdPlist', () => {
    const config: ServiceConfig = {
      port: 3100,
      nodePath: '/usr/local/bin/node',
      cliPath: '/path/to/cli.js',
      homeDir: '/Users/testuser',
      logsDir: '/Users/testuser/.nano-brain/logs',
    };

    it('contains Label key', () => {
      const plist = generateLaunchdPlist(config);
      expect(plist).toContain('<key>Label</key>');
      expect(plist).toContain('<string>com.nano-brain.server</string>');
    });

    it('contains KeepAlive key', () => {
      const plist = generateLaunchdPlist(config);
      expect(plist).toContain('<key>KeepAlive</key>');
      expect(plist).toContain('<true/>');
    });

    it('contains ProgramArguments with correct values', () => {
      const plist = generateLaunchdPlist(config);
      expect(plist).toContain('<key>ProgramArguments</key>');
      expect(plist).toContain(`<string>${config.nodePath}</string>`);
      expect(plist).toContain(`<string>${config.cliPath}</string>`);
      expect(plist).toContain('<string>serve</string>');
      expect(plist).toContain('<string>--port</string>');
      expect(plist).toContain(`<string>${config.port}</string>`);
    });

    it('contains RunAtLoad key', () => {
      const plist = generateLaunchdPlist(config);
      expect(plist).toContain('<key>RunAtLoad</key>');
    });

    it('contains StandardOutPath and StandardErrorPath', () => {
      const plist = generateLaunchdPlist(config);
      expect(plist).toContain('<key>StandardOutPath</key>');
      expect(plist).toContain(`<string>${config.logsDir}/server.log</string>`);
      expect(plist).toContain('<key>StandardErrorPath</key>');
      expect(plist).toContain(`<string>${config.logsDir}/server.err</string>`);
    });

    it('contains WorkingDirectory', () => {
      const plist = generateLaunchdPlist(config);
      expect(plist).toContain('<key>WorkingDirectory</key>');
      expect(plist).toContain(`<string>${config.homeDir}</string>`);
    });
  });

  describe('generateSystemdService', () => {
    const config: ServiceConfig = {
      port: 3100,
      nodePath: '/usr/bin/node',
      cliPath: '/path/to/cli.js',
      homeDir: '/home/testuser',
      logsDir: '/home/testuser/.nano-brain/logs',
    };

    it('contains Restart=always directive', () => {
      const service = generateSystemdService(config);
      expect(service).toContain('Restart=always');
    });

    it('contains RestartSec=2 directive', () => {
      const service = generateSystemdService(config);
      expect(service).toContain('RestartSec=2');
    });

    it('contains correct ExecStart', () => {
      const service = generateSystemdService(config);
      expect(service).toContain(`ExecStart=${config.nodePath} ${config.cliPath} serve --port ${config.port}`);
    });

    it('contains Unit section with Description', () => {
      const service = generateSystemdService(config);
      expect(service).toContain('[Unit]');
      expect(service).toContain('Description=nano-brain MCP server');
    });

    it('contains Service section', () => {
      const service = generateSystemdService(config);
      expect(service).toContain('[Service]');
      expect(service).toContain('Type=simple');
    });

    it('contains Install section', () => {
      const service = generateSystemdService(config);
      expect(service).toContain('[Install]');
      expect(service).toContain('WantedBy=default.target');
    });

    it('contains WorkingDirectory', () => {
      const service = generateSystemdService(config);
      expect(service).toContain(`WorkingDirectory=${config.homeDir}`);
    });

    it('contains StandardOutput and StandardError', () => {
      const service = generateSystemdService(config);
      expect(service).toContain(`StandardOutput=append:${config.logsDir}/server.log`);
      expect(service).toContain(`StandardError=append:${config.logsDir}/server.err`);
    });
  });

  describe('getDefaultServiceConfig', () => {
    it('returns config with default port 3100', () => {
      const config = getDefaultServiceConfig();
      expect(config.port).toBe(3100);
    });

    it('returns config with nodePath pointing to npx', () => {
      const config = getDefaultServiceConfig();
      expect(config.nodePath).toContain('npx');
    });

    it('returns config with homeDir from os.homedir()', () => {
      const config = getDefaultServiceConfig();
      expect(config.homeDir).toBeTruthy();
    });

    it('returns config with logsDir under .nano-brain', () => {
      const config = getDefaultServiceConfig();
      expect(config.logsDir).toContain('.nano-brain');
      expect(config.logsDir).toContain('logs');
    });

    it('returns config with cliPath set to nano-brain', () => {
      const config = getDefaultServiceConfig();
      expect(config.cliPath).toBe('nano-brain');
    });
  });
});
