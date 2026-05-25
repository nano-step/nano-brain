import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import Database from 'better-sqlite3';
import { SqliteEventStore } from '../src/event-store.js';
import type { JSONRPCMessage } from '@modelcontextprotocol/sdk/types.js';

describe('SqliteEventStore', () => {
  let db: Database.Database;
  let eventStore: SqliteEventStore;

  beforeEach(() => {
    db = new Database(':memory:');
    eventStore = new SqliteEventStore(db, 300);
  });

  afterEach(() => {
    db.close();
  });

  describe('storeEvent', () => {
    it('returns unique event IDs', async () => {
      const streamId = 'stream-1';
      const message: JSONRPCMessage = { jsonrpc: '2.0', method: 'test', id: 1 };

      const id1 = await eventStore.storeEvent(streamId, message);
      const id2 = await eventStore.storeEvent(streamId, message);

      expect(id1).toBeDefined();
      expect(id2).toBeDefined();
      expect(id1).not.toBe(id2);
    });

    it('stores events in the database', async () => {
      const streamId = 'stream-1';
      const message: JSONRPCMessage = { jsonrpc: '2.0', method: 'test', id: 1 };

      const eventId = await eventStore.storeEvent(streamId, message);

      const row = db.prepare('SELECT * FROM mcp_events WHERE event_id = ?').get(eventId) as {
        event_id: string;
        stream_id: string;
        message: string;
      };

      expect(row).toBeDefined();
      expect(row.stream_id).toBe(streamId);
      expect(JSON.parse(row.message)).toEqual(message);
    });
  });

  describe('replayEventsAfter', () => {
    it('replays events in order', async () => {
      const streamId = 'stream-1';
      const messages: JSONRPCMessage[] = [
        { jsonrpc: '2.0', method: 'test1', id: 1 },
        { jsonrpc: '2.0', method: 'test2', id: 2 },
        { jsonrpc: '2.0', method: 'test3', id: 3 },
      ];

      const eventIds: string[] = [];
      for (const msg of messages) {
        eventIds.push(await eventStore.storeEvent(streamId, msg));
      }

      const replayed: Array<{ eventId: string; message: JSONRPCMessage }> = [];
      const returnedStreamId = await eventStore.replayEventsAfter(eventIds[0], {
        send: async (eventId, message) => {
          replayed.push({ eventId, message });
        },
      });

      expect(returnedStreamId).toBe(streamId);
      expect(replayed).toHaveLength(2);
      expect(replayed[0].eventId).toBe(eventIds[1]);
      expect(replayed[0].message).toEqual(messages[1]);
      expect(replayed[1].eventId).toBe(eventIds[2]);
      expect(replayed[1].message).toEqual(messages[2]);
    });

    it('throws error for unknown event ID', async () => {
      await expect(
        eventStore.replayEventsAfter('unknown-id', {
          send: async () => {},
        })
      ).rejects.toThrow('Event not found');
    });

    it('returns empty replay for last event', async () => {
      const streamId = 'stream-1';
      const message: JSONRPCMessage = { jsonrpc: '2.0', method: 'test', id: 1 };
      const eventId = await eventStore.storeEvent(streamId, message);

      const replayed: Array<{ eventId: string; message: JSONRPCMessage }> = [];
      const returnedStreamId = await eventStore.replayEventsAfter(eventId, {
        send: async (eventId, message) => {
          replayed.push({ eventId, message });
        },
      });

      expect(returnedStreamId).toBe(streamId);
      expect(replayed).toHaveLength(0);
    });
  });

  describe('cleanup', () => {
    it('removes old events', async () => {
      const streamId = 'stream-1';
      const message: JSONRPCMessage = { jsonrpc: '2.0', method: 'test', id: 1 };

      await eventStore.storeEvent(streamId, message);

      db.prepare('UPDATE mcp_events SET created_at = ?').run(
        Math.floor(Date.now() / 1000) - 400
      );

      eventStore.cleanup();

      const count = db.prepare('SELECT COUNT(*) as count FROM mcp_events').get() as { count: number };
      expect(count.count).toBe(0);
    });

    it('preserves recent events', async () => {
      const streamId = 'stream-1';
      const message: JSONRPCMessage = { jsonrpc: '2.0', method: 'test', id: 1 };

      await eventStore.storeEvent(streamId, message);

      eventStore.cleanup();

      const count = db.prepare('SELECT COUNT(*) as count FROM mcp_events').get() as { count: number };
      expect(count.count).toBe(1);
    });
  });
});
