import { ERROR_KINDS, fail, WireError } from "./errors.ts";
import { AGGREGATE_ITEM_BYTES, V1_FRAME_BYTES } from "./constants.ts";

export class NdjsonFrameSplitter {
  readonly maximumBytes: number;
  private buffer: Uint8Array;
  private length = 0;
  private failed: WireError | undefined;
  private finished = false;

  constructor(maximumBytes = V1_FRAME_BYTES) {
    if (!Number.isSafeInteger(maximumBytes) || maximumBytes <= 0 || maximumBytes > AGGREGATE_ITEM_BYTES) {
      fail(ERROR_KINDS.INVALID_FRAME, "frame limit must be a bounded positive integer");
    }
    this.maximumBytes = maximumBytes;
    this.buffer = new Uint8Array(Math.min(maximumBytes + 1, 32 * 1024));
  }

  push(chunk: Uint8Array): Uint8Array[] {
    this.ensureUsable();
    if (!(chunk instanceof Uint8Array)) {
      fail(ERROR_KINDS.INVALID_FRAME, "frame chunk must be a Uint8Array");
    }
    const frames: Uint8Array[] = [];
    for (let index = 0; index < chunk.length; index += 1) {
      const byte = chunk[index];
      if (byte === 0x0a) {
        frames.push(this.finishFrame());
      } else {
        this.appendByte(byte);
      }
    }
    return frames;
  }

  finish(): void {
    this.ensureUsable();
    this.finished = true;
    if (this.length !== 0) {
      this.recordFailure(ERROR_KINDS.UNTERMINATED_FRAME, "final frame is not LF terminated");
    }
  }

  private appendByte(byte: number): void {
    if (this.length >= this.maximumBytes && byte !== 0x0d) {
      this.recordFailure(ERROR_KINDS.FRAME_TOO_LARGE, "frame exceeds its byte limit");
    }
    if (this.length >= this.maximumBytes + 1) {
      this.recordFailure(ERROR_KINDS.FRAME_TOO_LARGE, "frame exceeds its byte limit");
    }
    if (this.length >= this.buffer.length) {
      const nextCapacity = Math.min(this.maximumBytes + 1, Math.max(this.buffer.length * 2, this.length + 1));
      const next = new Uint8Array(nextCapacity);
      next.set(this.buffer.subarray(0, this.length));
      this.buffer = next;
    }
    this.buffer[this.length] = byte;
    this.length += 1;
  }

  private finishFrame(): Uint8Array {
    const storedLength = this.length;
    let bodyLength = storedLength;
    if (bodyLength > 0 && this.buffer[bodyLength - 1] === 0x0d) {
      bodyLength -= 1;
    }
    if (bodyLength > this.maximumBytes) {
      this.recordFailure(ERROR_KINDS.FRAME_TOO_LARGE, "frame exceeds its byte limit");
    }
    for (let index = 0; index < bodyLength; index += 1) {
      if (this.buffer[index] === 0x0d) {
        this.recordFailure(ERROR_KINDS.BARE_CARRIAGE_RETURN, "frame contains a bare carriage return");
      }
    }
    if (bodyLength === 0) {
      this.recordFailure(ERROR_KINDS.EMPTY_FRAME, "empty NDJSON frame");
    }
    const frame = this.buffer.slice(0, bodyLength);
    this.length = 0;
    return frame;
  }

  private ensureUsable(): void {
    if (this.failed !== undefined) {
      throw this.failed;
    }
    if (this.finished) {
      fail(ERROR_KINDS.INVALID_FRAME, "frame splitter has already reached EOF");
    }
  }

  private recordFailure(kind: typeof ERROR_KINDS[keyof typeof ERROR_KINDS], message: string): never {
    const error = new WireError(kind, message);
    this.failed = error;
    throw error;
  }
}

export const FrameReader = NdjsonFrameSplitter;

// NdjsonStreamReader keeps unconsumed transport bytes between calls so callers
// can change the frame limit at the bootstrap/v1 boundary without losing bytes
// that arrived in the same transport chunk.
export class NdjsonStreamReader {
  private readonly iterator: AsyncIterator<Uint8Array>;
  private pending: Uint8Array = new Uint8Array(0);
  private pendingOffset = 0;
  private buffer = new Uint8Array(32 * 1024);
  private length = 0;
  private ended = false;

  constructor(source: AsyncIterable<Uint8Array>) {
    if (source === null || typeof source !== "object" || typeof source[Symbol.asyncIterator] !== "function") {
      fail(ERROR_KINDS.INVALID_FRAME, "frame source must be an async byte iterable");
    }
    this.iterator = source[Symbol.asyncIterator]();
  }

  async readFrame(maximumBytes = V1_FRAME_BYTES): Promise<Uint8Array | null> {
    if (!Number.isSafeInteger(maximumBytes) || maximumBytes <= 0 || maximumBytes > AGGREGATE_ITEM_BYTES) {
      fail(ERROR_KINDS.INVALID_FRAME, "frame limit must be a bounded positive integer");
    }
    if (this.ended) {
      return null;
    }

    while (true) {
      if (this.pendingOffset >= this.pending.length) {
        const next = await this.iterator.next();
        if (next.done === true) {
          this.ended = true;
          if (this.length !== 0) {
            fail(ERROR_KINDS.UNTERMINATED_FRAME, "final frame is not LF terminated");
          }
          return null;
        }
        if (!(next.value instanceof Uint8Array)) {
          fail(ERROR_KINDS.INVALID_FRAME, "frame source yielded a non-byte chunk");
        }
        this.pending = next.value;
        this.pendingOffset = 0;
        if (this.pending.length === 0) {
          continue;
        }
      }

      const byte = this.pending[this.pendingOffset];
      this.pendingOffset += 1;
      if (byte === 0x0a) {
        return this.finishFrame(maximumBytes);
      }
      this.appendByte(byte, maximumBytes);
    }
  }

  private appendByte(byte: number, maximumBytes: number): void {
    if (this.length >= maximumBytes && byte !== 0x0d) {
      fail(ERROR_KINDS.FRAME_TOO_LARGE, "frame exceeds its byte limit");
    }
    if (this.length >= maximumBytes + 1) {
      fail(ERROR_KINDS.FRAME_TOO_LARGE, "frame exceeds its byte limit");
    }
    if (this.length >= this.buffer.length) {
      const capacity = Math.min(maximumBytes + 1, Math.max(this.buffer.length * 2, this.length + 1));
      const next = new Uint8Array(capacity);
      next.set(this.buffer.subarray(0, this.length));
      this.buffer = next;
    }
    this.buffer[this.length] = byte;
    this.length += 1;
  }

  private finishFrame(maximumBytes: number): Uint8Array {
    let bodyLength = this.length;
    if (bodyLength > 0 && this.buffer[bodyLength - 1] === 0x0d) {
      bodyLength -= 1;
    }
    if (bodyLength > maximumBytes) {
      fail(ERROR_KINDS.FRAME_TOO_LARGE, "frame exceeds its byte limit");
    }
    for (let index = 0; index < bodyLength; index += 1) {
      if (this.buffer[index] === 0x0d) {
        fail(ERROR_KINDS.BARE_CARRIAGE_RETURN, "frame contains a bare carriage return");
      }
    }
    if (bodyLength === 0) {
      fail(ERROR_KINDS.EMPTY_FRAME, "empty NDJSON frame");
    }
    const frame = this.buffer.slice(0, bodyLength);
    this.length = 0;
    return frame;
  }
}
