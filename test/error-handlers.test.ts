import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createRejectionThreshold } from '../src/server.js';

describe('createRejectionThreshold', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.spyOn(console, 'error').mockImplementation(() => {});
    vi.spyOn(process, 'exit').mockImplementation(() => undefined as never);
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('single rejection increments count', () => {
    const { handler, getCount } = createRejectionThreshold(3, 60000);
    expect(getCount()).toBe(0);
    handler(new Error('test error'));
    expect(getCount()).toBe(1);
  });

  it('count decrements after window expires', () => {
    const { handler, getCount } = createRejectionThreshold(3, 60000);
    handler(new Error('test error'));
    expect(getCount()).toBe(1);
    vi.advanceTimersByTime(60000);
    expect(getCount()).toBe(0);
  });

  it('3 rejections in window triggers exit', () => {
    const { handler, getCount } = createRejectionThreshold(3, 60000);
    handler(new Error('error 1'));
    handler(new Error('error 2'));
    expect(process.exit).not.toHaveBeenCalled();
    handler(new Error('error 3'));
    expect(getCount()).toBe(3);
    expect(process.exit).toHaveBeenCalledWith(1);
  });

  it('rejections spread across windows do not trigger exit', () => {
    const { handler, getCount } = createRejectionThreshold(3, 60000);
    handler(new Error('error 1'));
    handler(new Error('error 2'));
    vi.advanceTimersByTime(60000);
    expect(getCount()).toBe(0);
    handler(new Error('error 3'));
    handler(new Error('error 4'));
    expect(getCount()).toBe(2);
    expect(process.exit).not.toHaveBeenCalled();
  });
});
