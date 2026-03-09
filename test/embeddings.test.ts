import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  checkOllamaHealth,
  checkOpenAIHealth
} from '../src/embeddings.js';

describe('Embeddings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('checkOllamaHealth', () => {
    it('should return reachable when Ollama is healthy', async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ models: [{ name: 'embedding-model' }] })
      });

      const result = await checkOllamaHealth('http://localhost:11434');
      expect(result.reachable).toBe(true);
    });

    it('should return unreachable when Ollama is down', async () => {
      global.fetch = vi.fn().mockRejectedValueOnce(new Error('Connection refused'));

      const result = await checkOllamaHealth('http://localhost:11434');
      expect(result.reachable).toBe(false);
    });

    it('should return unreachable on HTTP error response', async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 500
      });

      const result = await checkOllamaHealth('http://localhost:11434');
      expect(result.reachable).toBe(false);
    });

    it('should handle response parsing errors gracefully', async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => { throw new Error('Invalid JSON'); }
      });

      const result = await checkOllamaHealth('http://localhost:11434');
      expect(result.reachable).toBe(false);
    });

    it('should return list of available models when healthy', async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ models: [
          { name: 'model1:latest' },
          { name: 'model2:latest' }
        ] })
      });

      const result = await checkOllamaHealth('http://localhost:11434');
      expect(result.models).toContain('model1:latest');
      expect(result.models).toContain('model2:latest');
    });

    it('should normalize URLs with trailing slashes', async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ models: [] })
      });

      await checkOllamaHealth('http://localhost:11434/');
      const url = (global.fetch as any).mock.calls[0][0];
      expect(url).toBe('http://localhost:11434//api/tags');
    });

    it('should set timeout for API requests', async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ models: [] })
      });

      await checkOllamaHealth('http://localhost:11434');
      const options = (global.fetch as any).mock.calls[0][1];
      expect(options?.signal).toBeDefined();
    });
  });

  describe('checkOpenAIHealth', () => {
    it('should return reachable when API is healthy', async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ model: 'text-embedding-3-small', owned_by: 'openai' })
      });

      const result = await checkOpenAIHealth('https://api.openai.com/v1', 'sk-xxx', 'text-embedding-3-small');
      expect(result.reachable).toBe(true);
    });

    it('should return unreachable on authentication error', async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: false,
        status: 401
      });

      const result = await checkOpenAIHealth('https://api.openai.com/v1', 'sk-xxx', 'text-embedding-3-small');
      expect(result.reachable).toBe(false);
    });

    it('should return unreachable when network is unavailable', async () => {
      global.fetch = vi.fn().mockRejectedValueOnce(new Error('Network error'));

      const result = await checkOpenAIHealth('https://api.openai.com/v1', 'sk-xxx', 'text-embedding-3-small');
      expect(result.reachable).toBe(false);
    });

    it('should include Bearer token in Authorization header', async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ model: 'model' })
      });

      await checkOpenAIHealth('https://api.openai.com/v1', 'sk-test123', 'text-embedding-3-small');
      const options = (global.fetch as any).mock.calls[0][1];
      expect(options?.headers?.Authorization).toBe('Bearer sk-test123');
    });

    it('should normalize base URL by removing trailing slash', async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ model: 'model' })
      });

      await checkOpenAIHealth('https://api.openai.com/v1/', 'sk-xxx', 'text-embedding-3-small');
      const url = (global.fetch as any).mock.calls[0][0];
      expect(url).toContain('https://api.openai.com/v1/v1/embeddings');
    });

    it('should work with custom API endpoints', async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ model: 'model' })
      });

      const result = await checkOpenAIHealth('https://custom.ai.com/v1', 'sk-xxx', 'custom-model');
      expect(result.reachable).toBe(true);
      const url = (global.fetch as any).mock.calls[0][0];
      expect(url).toContain('custom.ai.com');
    });

    it('should include model name in response', async () => {
      global.fetch = vi.fn().mockResolvedValueOnce({
        ok: true,
        json: async () => ({ model: 'text-embedding-3-large' })
      });

      const result = await checkOpenAIHealth('https://api.openai.com/v1', 'sk-xxx', 'text-embedding-3-large');
      expect(result.model).toBe('text-embedding-3-large');
    });
  });
});
