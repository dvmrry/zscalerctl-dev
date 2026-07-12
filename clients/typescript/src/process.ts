import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { isAbsolute } from "node:path";

import { EngineClient, EngineClientError, type EngineTransport, type EngineTransportExit } from "./client.ts";
import type { Redaction } from "./types.ts";

const MAX_STDERR_BYTES = 64 * 1024;

export interface EngineProcessOptions {
  // Requiring an absolute path avoids silently selecting a different binary
  // through a consumer-controlled PATH.
  readonly executable: string;
  readonly profile?: string;
  readonly config?: string;
  readonly timeout?: string;
  readonly redaction?: Redaction;
  readonly noCache?: boolean;
  readonly cancelTimeoutMs?: number;
  readonly closeTimeoutMs?: number;
  readonly cwd?: string;
  readonly env?: Readonly<NodeJS.ProcessEnv>;
}

class NodeEngineTransport implements EngineTransport {
  readonly output: AsyncIterable<Uint8Array>;
  readonly completion: Promise<EngineTransportExit>;

  private readonly child: ChildProcessWithoutNullStreams;
  private aborted = false;
  private stderrBytes = 0;

  constructor(child: ChildProcessWithoutNullStreams) {
    this.child = child;
    this.output = this.readOutput();
    this.completion = new Promise((resolve) => {
      child.once("error", () => resolve({ code: null, signal: null }));
      child.once("close", (code, signal) => resolve({ code, signal }));
    });
    // Drain stderr without exposing its contents. The candidate host is silent
    // by default; the bound prevents a compromised child from using this side
    // channel as an unbounded memory or pipe-pressure sink.
    child.stderr.on("data", (chunk: Uint8Array) => {
      this.stderrBytes += chunk.byteLength;
      if (this.stderrBytes > MAX_STDERR_BYTES) {
        this.abort();
      }
    });
    child.stderr.on("error", () => this.abort());
    child.stdin.on("error", () => this.abort());
  }

  async write(data: Uint8Array): Promise<void> {
    if (this.aborted || this.child.stdin.destroyed || this.child.stdin.writableEnded) {
      throw new Error("engine input is closed");
    }
    await new Promise<void>((resolve, reject) => {
      this.child.stdin.write(data, (error) => {
        if (error !== null && error !== undefined) reject(error);
        else resolve();
      });
    });
  }

  async closeInput(): Promise<void> {
    if (this.child.stdin.writableEnded) return;
    if (this.aborted || this.child.stdin.destroyed) {
      throw new Error("engine input is closed");
    }
    await new Promise<void>((resolve, reject) => {
      const failed = (): void => reject(new Error("engine input close failed"));
      this.child.stdin.once("error", failed);
      this.child.stdin.end(() => {
        this.child.stdin.removeListener("error", failed);
        resolve();
      });
    });
  }

  abort(): void {
    if (this.aborted) return;
    this.aborted = true;
    this.child.stdin.destroy();
    this.child.stdout.destroy();
    this.child.stderr.destroy();
    if (this.child.exitCode === null && this.child.signalCode === null) {
      this.child.kill("SIGKILL");
    }
  }

  private async *readOutput(): AsyncGenerator<Uint8Array> {
    for await (const chunk of this.child.stdout) {
      if (!(chunk instanceof Uint8Array)) {
        throw new Error("engine output was not bytes");
      }
      yield chunk;
    }
  }
}

function checkedString(name: string, value: unknown, allowEmpty: boolean): string {
  if (typeof value !== "string" || value.includes("\u0000") || (!allowEmpty && value.length === 0)) {
    throw new EngineClientError("request", `${name} is invalid`);
  }
  return value;
}

function copyEnvironment(source: Readonly<NodeJS.ProcessEnv>): NodeJS.ProcessEnv {
  const output: NodeJS.ProcessEnv = Object.create(null) as NodeJS.ProcessEnv;
  for (const [name, value] of Object.entries(source)) {
    if (name.includes("\u0000") || (value !== undefined && value.includes("\u0000"))) {
      throw new EngineClientError("request", "engine environment contains a NUL byte");
    }
    if (value !== undefined) output[name] = value;
  }
  return output;
}

function processArguments(options: EngineProcessOptions): string[] {
  const args: string[] = [];
  if (options.profile !== undefined && options.profile !== "") {
    args.push("--profile", checkedString("profile", options.profile, false));
  }
  if (options.config !== undefined && options.config !== "") {
    args.push("--config", checkedString("config", options.config, false));
  }
  if (options.timeout !== undefined) {
    args.push("--timeout", checkedString("timeout", options.timeout, false));
  }
  if (options.redaction !== undefined) {
    if (options.redaction !== "standard" && options.redaction !== "share" && options.redaction !== "paranoid") {
      throw new EngineClientError("request", "redaction is invalid");
    }
    args.push("--redaction", options.redaction);
  }
  if (options.noCache === true) args.push("--no-cache");
  if (options.noCache !== undefined && typeof options.noCache !== "boolean") {
    throw new EngineClientError("request", "noCache is invalid");
  }
  return args;
}

export async function spawnEngine(options: EngineProcessOptions): Promise<EngineClient> {
  if (options === null || typeof options !== "object") {
    throw new EngineClientError("request", "engine process options are invalid");
  }
  const executable = checkedString("executable", options.executable, false);
  if (!isAbsolute(executable)) {
    throw new EngineClientError("request", "engine executable must be an absolute path");
  }
  const cwd = options.cwd === undefined ? undefined : checkedString("cwd", options.cwd, false);
  const args = processArguments(options);
  const env = copyEnvironment(options.env ?? process.env);

  let child: ChildProcessWithoutNullStreams;
  try {
    child = spawn(executable, args, {
      cwd,
      env,
      shell: false,
      windowsHide: true,
      stdio: ["pipe", "pipe", "pipe"],
    });
  } catch {
    throw new EngineClientError("transport", "engine process could not be started");
  }
  const transport = new NodeEngineTransport(child);
  try {
    return await EngineClient.connect(transport, {
      cancelTimeoutMs: options.cancelTimeoutMs,
      closeTimeoutMs: options.closeTimeoutMs,
    });
  } catch (error) {
    transport.abort();
    throw error;
  }
}
