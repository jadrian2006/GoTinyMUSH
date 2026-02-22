#!/usr/bin/env node

import { loadConfig } from "./config.js";
import { authenticate } from "./mush/auth.js";
import { MushClient } from "./mush/client.js";
import { createProvider } from "./ai/factory.js";
import { Router } from "./brain/router.js";
import { ContextManager } from "./brain/context.js";
import { TriggerMatcher } from "./brain/trigger.js";
import { Responder } from "./brain/responder.js";
import { RateLimiter } from "./util/rate-limiter.js";

async function main() {
  console.log("[bot] GoTinyMUSH AI Bot starting...");

  const config = loadConfig();
  console.log(`[bot] Bot name: ${config.bot.name}`);
  console.log(`[bot] LLM provider: ${config.llm.provider} (${config.llm.model})`);
  console.log(`[bot] MUSH: ${config.mush.restUrl}`);

  // Authenticate with MUSH
  const auth = await authenticate(config.mush);
  console.log(
    `[bot] Authenticated as ${auth.playerName} (#${auth.playerRef})`
  );

  // Create LLM provider
  const llm = createProvider(config.llm);

  // Create brain components
  const trigger = new TriggerMatcher(config.bot);
  const context = new ContextManager(config.context);
  const rateLimiter = new RateLimiter(config.cooldownMs);
  const client = new MushClient(config.mush.wsUrl, auth.token);
  const responder = new Responder(client, config);
  const router = new Router(
    config,
    trigger,
    context,
    rateLimiter,
    llm,
    responder,
    auth
  );

  // Wire up events
  client.on("event", (msg) => router.handleEvent(msg));
  client.on("connected", () => {
    console.log("[bot] WebSocket connected");
    responder.onConnect();
  });
  client.on("disconnected", () => {
    console.log("[bot] WebSocket disconnected");
  });

  // Graceful shutdown
  let shuttingDown = false;
  const shutdown = () => {
    if (shuttingDown) return;
    shuttingDown = true;
    console.log("[bot] Shutting down...");
    responder.stopKeepalive();
    client.close();
    process.exit(0);
  };
  process.on("SIGINT", shutdown);
  process.on("SIGTERM", shutdown);

  // Connect
  client.connect();
  console.log("[bot] Running. Press Ctrl+C to stop.");
}

main().catch((err) => {
  console.error("[bot] Fatal:", err);
  process.exit(1);
});
